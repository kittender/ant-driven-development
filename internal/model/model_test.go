package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kittender/add/internal/core"
)

var menu = Menu{
	Tools:    []core.Tool{core.ToolDeposit, core.ToolFinish},
	Emoji:    []string{"🧪", "🐛"},
	Outcomes: []core.Outcome{core.OutcomeDone, core.OutcomeCannot},
}

func TestParseText(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want core.Action
	}{
		{"line deposit", "deposit src/a.ts#f 🧪", core.Action{Tool: "deposit", Target: "src/a.ts#f", Emoji: "🧪"}},
		{"line with prose", "Looking at it.\n\n`deposit src/a.ts 🐛`\n", core.Action{Tool: "deposit", Target: "src/a.ts", Emoji: "🐛"}},
		{"finish with note", "finish done all good here", core.Action{Tool: "finish", Outcome: "done", Note: "all good here"}},
		{"uppercase verb", "FINISH cannot", core.Action{Tool: "finish", Outcome: "cannot"}},
		{"json", `{"tool":"deposit","target":"src/a.ts","emoji":"🧪"}`, core.Action{Tool: "deposit", Target: "src/a.ts", Emoji: "🧪"}},
		{"json in fence", "```json\n{\"tool\":\"finish\",\"outcome\":\"done\",\"note\":\"a } b\"}\n```", core.Action{Tool: "finish", Outcome: "done", Note: "a } b"}},
		{"think block", "<think>deposit x 🐛</think>\nfinish done", core.Action{Tool: "finish", Outcome: "done"}},
	}
	for _, c := range cases {
		got, err := ParseText(c.in, menu)
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}

	bad := []string{"", "I think it needs a test.", "deposit src/a.ts", "deposit a b c d",
		"finish", "finish maybe", `{"tool":"deposit"}`, `{"tool":"dance"}`, `{"tool":"finish","outcome":"done","extra":1}`}
	for _, in := range bad {
		_, err := ParseText(in, menu)
		var pe *ParseError
		if !errors.As(err, &pe) {
			t.Errorf("ParseText(%q) = %v, want a ParseError", in, err)
		}
	}
}

// FuzzParseText checks the parser never panics and only returns shaped actions.
// Run it longer with: go test ./internal/model -fuzz=FuzzParseText -fuzztime=30s
func FuzzParseText(f *testing.F) {
	for _, s := range []string{"deposit a.ts 🧪", "finish done", `{"tool":"finish","outcome":"done"}`, "{{{", "<think>"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		a, err := ParseText(in, menu)
		if err != nil {
			return
		}
		switch a.Tool {
		case core.ToolDeposit:
			if a.Target == "" || a.Emoji == "" {
				t.Fatalf("deposit without fields from %q", in)
			}
		case core.ToolFinish:
			if a.Outcome != core.OutcomeDone && a.Outcome != core.OutcomeCannot {
				t.Fatalf("bad outcome from %q", in)
			}
		}
	})
}

func TestGatewayRouting(t *testing.T) {
	fake := &Fake{Reply: func(req Request, call int) (string, error) {
		if req.Model != "scripted" {
			return "", errors.New("wrong model " + req.Model)
		}
		return []string{"nonsense", "deposit src/a.ts 🧪"}[call], nil
	}}
	g := NewGateway(fake)
	r := g.Act(context.Background(), Request{Model: "fake:scripted", Menu: menu})
	if r.ParseErr == nil || r.Action != nil {
		t.Fatalf("first call: want parse error, got %+v", r)
	}
	r = g.Act(context.Background(), Request{Model: "fake:scripted", Menu: menu})
	if r.Action == nil || r.Action.Emoji != "🧪" {
		t.Fatalf("second call: %+v", r)
	}
	if r := g.Act(context.Background(), Request{Model: "nope:x"}); r.Err == nil {
		t.Error("unknown provider accepted")
	}
	if r := g.Act(context.Background(), Request{Model: "noprovider"}); r.Err == nil {
		t.Error("model without provider accepted")
	}
}

// httptest gives a real HTTP server in-process. JS analogy: msw or nock,
// but nothing is mocked: the client really talks HTTP.
func TestOllama(t *testing.T) {
	var got ollamaChatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Error(err)
		}
		io.WriteString(w, `{"message":{"role":"assistant","content":"{\"tool\":\"deposit\",\"target\":\"src/a.ts\",\"emoji\":\"🐛\"}"},"prompt_eval_count":120,"eval_count":14}`)
	}))
	defer srv.Close()

	g := NewGateway(&Ollama{BaseURL: srv.URL, Schema: true})
	r := g.Act(context.Background(), Request{
		Model: "ollama:qwen3:4b", System: "SYS", Menu: menu,
		Messages: []Message{{Role: "user", Content: "TASK"}}, Seed: 7, MaxTokens: 64,
	})
	if r.Action == nil || r.Action.Emoji != "🐛" {
		t.Fatalf("result %+v", r)
	}
	if r.Usage.InputTokens != 120 || r.Usage.OutputTokens != 14 {
		t.Errorf("usage %+v", r.Usage)
	}
	if got.Model != "qwen3:4b" || got.Messages[0].Role != "system" || got.Messages[0].Content != "SYS" {
		t.Errorf("request %+v", got)
	}
	if got.Format == nil || got.Options["seed"].(float64) != 7 {
		t.Errorf("schema or seed missing: %+v", got)
	}
}

func TestOllamaError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, `{"error":"model \"nope\" not found"}`)
	}))
	defer srv.Close()
	r := NewGateway(&Ollama{BaseURL: srv.URL}).Act(context.Background(), Request{Model: "ollama:nope", Menu: menu})
	if r.Err == nil || !strings.Contains(r.Err.Error(), "not found") {
		t.Fatalf("want transport error, got %+v", r)
	}
}

func TestAnthropic(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "k" || r.URL.Path != "/v1/messages" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewDecoder(r.Body).Decode(&body)
		io.WriteString(w, `{"content":[{"type":"text","text":"Marking it."},{"type":"tool_use","id":"t1","name":"finish","input":{"outcome":"done","note":"ok"}}],
			"usage":{"input_tokens":300,"output_tokens":20,"cache_read_input_tokens":250}}`)
	}))
	defer srv.Close()

	g := NewGateway(&Anthropic{APIKey: "k", BaseURL: srv.URL})
	r := g.Act(context.Background(), Request{Model: "anthropic:claude-haiku-5-5", System: "SYS", Menu: menu,
		Messages: []Message{{Role: "user", Content: "TASK"}}, MaxTokens: 100})
	if r.Action == nil || r.Action.Tool != core.ToolFinish || r.Action.Note != "ok" {
		t.Fatalf("result %+v (err %v, parse %v)", r.Action, r.Err, r.ParseErr)
	}
	if r.Usage.CachedTokens != 250 {
		t.Errorf("usage %+v", r.Usage)
	}
	if tc := body["tool_choice"].(map[string]any); tc["type"] != "any" {
		t.Errorf("tool_choice %v", tc)
	}
	if n := len(body["tools"].([]any)); n != 2 {
		t.Errorf("%d tools sent", n)
	}
}

func TestAnthropicNeedsKey(t *testing.T) {
	r := NewGateway(&Anthropic{}).Act(context.Background(), Request{Model: "anthropic:x", Menu: menu})
	if r.Err == nil {
		t.Fatal("missing key accepted")
	}
}

// Regression: the grammar follows property order, so "tool" must come first,
// and every field must be required or small models skip "emoji".
func TestActionSchemaOrderAndRequired(t *testing.T) {
	raw, err := json.Marshal(ActionSchema(menu))
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	props := s[strings.Index(s, `"properties"`):]
	last := -1
	for _, key := range []string{`"note"`, `"tool"`, `"target"`, `"emoji"`, `"outcome"`} {
		i := strings.Index(props, key)
		if i <= last {
			t.Fatalf("%s missing or out of order in %s", key, props)
		}
		last = i
	}
	var decoded struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Required) != 5 {
		t.Errorf("required = %v, want all five fields", decoded.Required)
	}
	if e := decoded.Properties["emoji"].Enum; !contains(e, "") || !contains(e, "🧪") {
		t.Errorf("emoji enum %v must allow the menu and the empty string", e)
	}

	// Regression: targets are a closed choice, so models cannot garble the path.
	withTargets := menu
	withTargets.Targets = []string{"src/a.ts#f", "src/a.ts"}
	raw, _ = json.Marshal(ActionSchema(withTargets))
	json.Unmarshal(raw, &decoded)
	if e := decoded.Properties["target"].Enum; len(e) != 3 || e[0] != "src/a.ts#f" || !contains(e, "") {
		t.Errorf("target enum = %v", e)
	}
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

// What schema-bound output looks like now: every field present, unused ones empty.
func TestParseSchemaShapedOutput(t *testing.T) {
	a, err := ParseStructured(json.RawMessage(`{"tool":"finish","target":"","emoji":"","outcome":"done","note":"ok"}`), menu)
	if err != nil || a.Tool != core.ToolFinish {
		t.Fatalf("finish: %+v, %v", a, err)
	}
	a, err = ParseStructured(json.RawMessage(`{"tool":"deposit","target":"src/a.ts","emoji":"🧪","outcome":"","note":""}`), menu)
	if err != nil || a.Emoji != "🧪" {
		t.Fatalf("deposit: %+v, %v", a, err)
	}
	if _, err := ParseStructured(json.RawMessage(`{"tool":"deposit","target":"src/a.ts","emoji":"","outcome":"","note":""}`), menu); err == nil {
		t.Fatal("deposit with empty emoji accepted")
	}
}

// Regression: thinking models must get think:false; models that reject the
// field are retried without it, once, and remembered.
func TestOllamaThink(t *testing.T) {
	calls := map[string]int{}
	var sawThink []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		model := body["model"].(string)
		calls[model]++
		think, has := body["think"]
		if model == "plain" && has {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, `{"error":"\"plain\" does not support thinking"}`)
			return
		}
		sawThink = append(sawThink, think)
		io.WriteString(w, `{"message":{"content":"finish done","thinking":"deposit x 🐛"},"eval_count":3}`)
	}))
	defer srv.Close()

	g := NewGateway(&Ollama{BaseURL: srv.URL})
	r := g.Act(context.Background(), Request{Model: "ollama:thinker", Menu: menu})
	if r.Action == nil || r.Action.Tool != core.ToolFinish {
		t.Fatalf("thinking text must not be parsed: %+v", r)
	}
	if len(sawThink) != 1 || sawThink[0] != false {
		t.Fatalf("think sent = %v, want false", sawThink)
	}
	for i := 0; i < 2; i++ {
		if r := g.Act(context.Background(), Request{Model: "ollama:plain", Menu: menu}); r.Action == nil {
			t.Fatalf("plain model call %d: %+v", i, r)
		}
	}
	if calls["plain"] != 3 { // 1 rejected + 1 retry, then 1 direct
		t.Errorf("plain model called %d times, want 3", calls["plain"])
	}
}

func TestAliasMenuSchemaAndParse(t *testing.T) {
	m := menu
	m.Emoji = []string{"needs_test", "bug"}
	m.SignalField = "signal"
	raw, _ := json.Marshal(ActionSchema(m))
	if s := string(raw); !strings.Contains(s, `"signal"`) || strings.Contains(s, `"emoji"`) {
		t.Fatalf("schema should name the field signal: %s", s)
	}
	a, err := ParseStructured(json.RawMessage(`{"note":"n","tool":"deposit","target":"src/a.ts","signal":"needs_test","outcome":""}`), m)
	if err != nil || a.Emoji != "needs_test" || a.Signal != "" {
		t.Fatalf("%+v, %v", a, err)
	}
	if tools := toolSchemas(m); !strings.Contains(fmt.Sprint(tools[0]), "signal") {
		t.Errorf("anthropic tool schema: %v", tools[0])
	}
}

func TestBinaryMenu(t *testing.T) {
	m := Menu{Tools: []core.Tool{core.ToolDeposit, core.ToolFinish}, Emoji: []string{"🐛"},
		Targets: []string{"src/a.ts#f"}, Outcomes: []core.Outcome{core.OutcomeDone}, Binary: true}
	raw, _ := json.Marshal(ActionSchema(m))
	if s := string(raw); !strings.Contains(s, `"properties":{"note":{"type":"string"},"answer":{"enum":["no","yes"]`) {
		t.Fatalf("binary schema: %s", s)
	}
	cases := []struct {
		in      string
		tool    core.Tool
		deposit bool
	}{
		{`{"note":"i <= length reads past the end","answer":"yes"}`, core.ToolDeposit, true},
		{`{"note":"fine","answer":"no"}`, core.ToolFinish, false},
		{"yes the loop reads one past the end", core.ToolDeposit, true},
		{"No. Looks fine.", core.ToolFinish, false},
	}
	for _, c := range cases {
		a, err := ParseText(c.in, m)
		if err != nil || a.Tool != c.tool {
			t.Errorf("%q: %+v, %v", c.in, a, err)
			continue
		}
		if c.deposit && (a.Emoji != "🐛" || a.Target != "src/a.ts#f") {
			t.Errorf("%q: yes must deposit the receptor's signal on its target, got %+v", c.in, a)
		}
	}
	for _, bad := range []string{`{"note":"x","answer":"maybe"}`, "perhaps", ""} {
		if _, err := ParseText(bad, m); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

// H1 arm: "none" is one value of the signal field, not a separate tool.
func TestNoneOptionMenu(t *testing.T) {
	m := Menu{Tools: []core.Tool{core.ToolDeposit, core.ToolFinish}, Emoji: []string{"needs_test", "bug"},
		SignalField: "signal", Targets: []string{"src/a.ts#f", "src/a.ts"},
		Outcomes: []core.Outcome{core.OutcomeDone}, NoneOption: true}
	raw, _ := json.Marshal(ActionSchema(m))
	s := string(raw)
	if !strings.Contains(s, `"properties":{"note":`) || !strings.Contains(s, `"enum":["needs_test","bug","none"]`) || strings.Contains(s, `"tool"`) {
		t.Fatalf("none-option schema: %s", s)
	}
	cases := []struct {
		in    string
		tool  core.Tool
		emoji string
	}{
		{`{"note":"clean","target":"src/a.ts#f","signal":"none"}`, core.ToolFinish, ""},
		{`{"note":"no test","target":"src/a.ts#f","signal":"needs_test"}`, core.ToolDeposit, "needs_test"},
		{"none nothing to do", core.ToolFinish, ""},
		{"deposit src/a.ts bug", core.ToolDeposit, "bug"},
	}
	for _, c := range cases {
		a, err := ParseText(c.in, m)
		if err != nil || a.Tool != c.tool || a.Emoji != c.emoji {
			t.Errorf("%q: %+v, %v", c.in, a, err)
		}
	}
	if _, err := ParseText(`{"note":"x","target":"src/a.ts","signal":""}`, m); err == nil {
		t.Error("an empty signal was accepted")
	}
}

// Rule 6: a found-answer receptor has no yes/no token; the finding is the answer.
func TestFoundAnswerMenu(t *testing.T) {
	m := Menu{Tools: []core.Tool{core.ToolDeposit, core.ToolFinish}, Emoji: []string{"🧪"},
		Targets: []string{"src/a.ts#f"}, Outcomes: []core.Outcome{core.OutcomeDone}, Binary: true, FoundAnswer: true}
	raw, _ := json.Marshal(ActionSchema(m))
	if s := string(raw); !strings.Contains(s, `"properties":{"note":{"type":"string"},"found":{"type":"string"}}`) || strings.Contains(s, "yes") {
		t.Fatalf("found schema: %s", s)
	}
	cases := []struct {
		in      string
		deposit bool
	}{
		{`{"note":"no spec covers the comma branch","found":"parseAmount with a comma decimal"}`, true},
		{`{"note":"all covered","found":""}`, false},
		{`{"note":"all covered","found":"none"}`, false},
		{`{"note":"x","found":"N/A"}`, false},
		{"found the empty-string branch", true},
		{"none everything is tested", false},
	}
	for _, c := range cases {
		a, err := ParseText(c.in, m)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if (a.Tool == core.ToolDeposit) != c.deposit {
			t.Errorf("%q: got %+v", c.in, a)
		}
		if c.deposit && (a.Emoji != "🧪" || a.Target != "src/a.ts#f" || a.Found == "") {
			t.Errorf("%q: deposit must carry the receptor's signal, target and finding, got %+v", c.in, a)
		}
	}
	if _, err := ParseText("maybe something", m); err == nil {
		t.Error("a reply that is neither found nor none was accepted")
	}
}

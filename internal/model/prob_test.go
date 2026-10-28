package model

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func tok(s string, lp float64, top ...TopLogprob) TokenLogprob {
	return TokenLogprob{Token: s, Logprob: lp, Top: top}
}

func TestAnswerProbability(t *testing.T) {
	want := math.Exp(-0.2) / (math.Exp(-0.2) + math.Exp(-1.8))
	cases := map[string][]TokenLogprob{
		"value in its own token": {tok(`{"`, 0), tok("note", 0), tok(`":"`, 0), tok("ok", 0), tok(`","`, 0),
			tok("answer", 0), tok(`":"`, 0), tok("yes", -0.2, TopLogprob{"yes", -0.2}, TopLogprob{"no", -1.8}), tok(`"}`, 0)},
		"quote glued to the value": {tok(`{"answer":`, 0),
			tok(` "yes`, -0.2, TopLogprob{` "yes`, -0.2}, TopLogprob{` "no`, -1.8}, TopLogprob{` "`, -3}), tok(`"}`, 0)},
		"chosen token missing from the list": {tok(`{"answer": "`, 0), tok("Yes", -0.2, TopLogprob{"no", -1.8}), tok(`"}`, 0)},
	}
	for name, lps := range cases {
		got, err := AnswerProbability(lps)
		if err != nil || math.Abs(got-want) > 1e-9 {
			t.Errorf("%s: P = %v, %v; want %v", name, got, err, want)
		}
	}
	if _, err := AnswerProbability([]TokenLogprob{tok(`{"note":"x"}`, 0)}); err == nil {
		t.Error("no answer field: want error")
	}
	if _, err := AnswerProbability([]TokenLogprob{tok(`{"answer":"`, 0), tok("maybe", -0.1)}); err == nil {
		t.Error("no yes/no alternative: want error")
	}
}

func TestBinarySchemaOrderAndDirect(t *testing.T) {
	enum := func(m Menu) []any {
		raw, _ := json.Marshal(ActionSchema(m))
		var s struct {
			Properties map[string]struct {
				Enum []any `json:"enum"`
			} `json:"properties"`
			Required []string `json:"required"`
		}
		_ = json.Unmarshal(raw, &s)
		return s.Properties["answer"].Enum
	}
	if e := enum(Menu{Binary: true}); e[0] != "no" {
		t.Errorf("default order %v (the void run's order must not change)", e)
	}
	if e := enum(Menu{Binary: true, YesFirst: true}); e[0] != "yes" {
		t.Errorf("yes first: %v", e)
	}
	raw, _ := json.Marshal(ActionSchema(Menu{Binary: true, Direct: true}))
	if strings.Contains(string(raw), "note") {
		t.Errorf("direct schema has a note: %s", raw)
	}
}

func TestOllamaLogprobs(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		_, _ = w.Write([]byte(`{"message":{"content":"{\"answer\":\"no\"}"},
			"logprobs":[{"token":"{\"answer\":\"","logprob":0,"top_logprobs":[]},
			            {"token":"no","logprob":-0.1,"top_logprobs":[{"token":"no","logprob":-0.1},{"token":"yes","logprob":-2.4}]}],
			"prompt_eval_count":10,"eval_count":3}`))
	}))
	defer srv.Close()
	o := &Ollama{BaseURL: srv.URL, Schema: true}
	c, err := o.Complete(t.Context(), Request{Model: "m", Menu: Menu{Binary: true, Direct: true}, TopLogprobs: 20})
	if err != nil {
		t.Fatal(err)
	}
	if got["logprobs"] != true || got["top_logprobs"] != float64(20) {
		t.Errorf("request %v", got)
	}
	p, err := AnswerProbability(c.Logprobs)
	if err != nil || p > 0.2 {
		t.Errorf("P(yes) = %v, %v", p, err)
	}

	// An Ollama too old for logprobs answers without them: say so plainly.
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"message":{"content":"{\"answer\":\"no\"}"}}`))
	}))
	defer old.Close()
	o = &Ollama{BaseURL: old.URL}
	if _, err := o.Complete(t.Context(), Request{Model: "m", TopLogprobs: 20}); err == nil || !strings.Contains(err.Error(), "0.12.11") {
		t.Errorf("old Ollama: %v", err)
	}
	// Without TopLogprobs nothing changes.
	if _, err := o.Complete(t.Context(), Request{Model: "m"}); err != nil {
		t.Errorf("plain call: %v", err)
	}
}

package tools

import (
	"os"
	"strings"
	"testing"

	"kittender/add/internal/core"
)

func setup(t *testing.T) *Session {
	t.Helper()
	f, err := os.Open("../../ecosystem-home.example/dictionary.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := core.LoadDictionary(f)
	if err != nil {
		t.Fatal(err)
	}
	g := core.Genome{
		ID: "g", Caste: core.Scout, Prompt: "p",
		MayDeposit: []string{"🧪", "🐛"}, Tools: []core.Tool{core.ToolDeposit, core.ToolFinish},
		Limits: core.Limits{Steps: 3, Seconds: 5, OutputTokens: 50, Malformed: 1},
	}
	scope, _ := core.ParseTarget("src/a.ts#f")
	return NewSession("leni03", g, scope, d)
}

// Each case is one rule of the architecture's enforcement map.
func TestRefusals(t *testing.T) {
	cases := map[string]core.Action{
		"verdict emoji is refused":        {Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: "✅"},
		"emoji outside may_deposit":       {Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: "📝"},
		"emoji outside the dictionary":    {Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: "🦄"},
		"target outside scope":            {Tool: core.ToolDeposit, Target: "src/b.ts", Emoji: "🧪"},
		"path traversal":                  {Tool: core.ToolDeposit, Target: "src/../a.ts", Emoji: "🧪"},
		"tool not in the caste (write)":   {Tool: core.ToolWrite, Target: "src/a.ts"},
		"tool in caste, not in genome":    {Tool: core.ToolRead, Target: "src/a.ts"},
		"soldier tool for a scout (flag)": {Tool: core.ToolFlag, Target: "@x"},
	}
	for name, a := range cases {
		s := setup(t)
		res := s.Do(a)
		if res.Refusal == "" || res.Deposit != nil || res.Finished {
			t.Errorf("%s: %+v", name, res)
		}
		if !strings.HasPrefix(res.Message, "refused: ") {
			t.Errorf("%s: message %q", name, res.Message)
		}
	}
}

func TestDepositAndDuplicate(t *testing.T) {
	s := setup(t)
	res := s.Do(core.Action{Tool: core.ToolDeposit, Target: "src/a.ts#f", Emoji: "🧪️"})
	if res.Deposit == nil || res.Deposit.Actor != "leni03" || res.Deposit.Emoji != "🧪" || !res.Deposit.Normalized {
		t.Fatalf("deposit %+v", res)
	}
	again := s.Do(core.Action{Tool: core.ToolDeposit, Target: "src/a.ts#f", Emoji: "🧪"})
	if !again.Ignored || again.Deposit != nil || again.Refusal != "" {
		t.Fatalf("duplicate should be ignored, got %+v", again)
	}
	other := s.Do(core.Action{Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: "🧪"})
	if other.Deposit == nil {
		t.Fatalf("same emoji on another target should count, got %+v", other)
	}
}

func TestFinish(t *testing.T) {
	res := setup(t).Do(core.Action{Tool: core.ToolFinish, Outcome: core.OutcomeCannot})
	if !res.Finished || res.Outcome != core.OutcomeCannot {
		t.Fatalf("%+v", res)
	}
}

// Alias menu: the ageant names a signal, the trail stores the emoji, and
// verdict names are refused like verdict emoji.
func TestDepositByAlias(t *testing.T) {
	s := setup(t)
	res := s.Do(core.Action{Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: "Needs_Test"})
	if res.Deposit == nil || res.Deposit.Emoji != "🧪" {
		t.Fatalf("alias deposit: %+v", res)
	}
	for _, name := range []string{"passed", "locked", "unicorn", "needs_doc"} { // verdict, verdict, unknown, not in genome
		if r := setup(t).Do(core.Action{Tool: core.ToolDeposit, Target: "src/a.ts", Emoji: name}); r.Refusal == "" {
			t.Errorf("%s accepted: %+v", name, r)
		}
	}
}

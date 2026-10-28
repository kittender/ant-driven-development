package pilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kittender/add/internal/instruments"
	"kittender/add/internal/model"
)

const feat = `Feature: Start a new game

  @capped
  Scenario: A new game has an empty board
    When I start a new game
    Then the board should have 9 empty cells
    And it should be player "X"'s turn
`

// fakeTools stands in for tsc, eslint, vitest and jscpd. Steps are "defined"
// when the feature's step file exists; the scenario passes when game.ts
// contains IMPLEMENTED; a file containing SYNTAX does not compile.
func fakeTools(t *testing.T) instruments.Runner {
	cmds := instruments.TypeScript()
	return func(_ context.Context, dir string, argv ...string) instruments.Output {
		read := func(rel string) string {
			b, _ := os.ReadFile(filepath.Join(dir, rel))
			return string(b)
		}
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "tsc"):
			for _, f := range []string{"src/game/game.ts", "tests/steps/01_start_game.steps.ts"} {
				if strings.Contains(read(f), "SYNTAX") {
					return instruments.Output{Exit: 2, Stdout: f + "(1,1): error TS1005: ';' expected."}
				}
			}
		case strings.Contains(joined, "eslint"):
			return instruments.Output{Stdout: "[]"}
		case strings.Contains(joined, "vitest"):
			specFile := read("spec/01_start_game.feature")
			status, msg := "failed", "Undefined step: When I start a new game"
			if read("tests/steps/01_start_game.steps.ts") != "" {
				msg = `AssertionError: expected turn to be "X"`
				// It passes when the game is implemented, or when the honey
				// no longer checks the one outcome that fails.
				if strings.Contains(read("src/game/game.ts"), "IMPLEMENTED") || !strings.Contains(specFile, `it should be player "X"'s turn`) {
					status, msg = "passed", ""
				}
			}
			var cases []map[string]any
			for _, name := range []string{"A new game has an empty board", "Weaker"} {
				if strings.Contains(specFile, "Scenario: "+name) {
					fm := []string{}
					if msg != "" {
						fm = []string{msg}
					}
					cases = append(cases, map[string]any{"ancestorTitles": []string{}, "title": name, "status": status, "failureMessages": fm})
				}
			}
			rep, _ := json.Marshal(map[string]any{"testResults": []any{map[string]any{"name": dir + "/spec/01_start_game.feature", "assertionResults": cases}}})
			writeFile(t, dir, cmds.VitestJSON, string(rep))
			writeFile(t, dir, cmds.CoverageJSON, `{}`)
			writeFile(t, dir, cmds.JUnitXML, `<testsuites></testsuites>`)
			return instruments.Output{Exit: 1}
		case strings.Contains(joined, "jscpd"):
			writeFile(t, dir, cmds.JscpdJSON, `{"duplicates":[]}`)
		}
		return instruments.Output{}
	}
}

func writeFile(t *testing.T, dir, rel, s string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func setup(t *testing.T) (ws, out string) {
	ws, out = t.TempDir(), t.TempDir()
	writeFile(t, ws, "spec/01_start_game.feature", feat)
	writeFile(t, ws, "spec/README.md", "vocabulary")
	writeFile(t, ws, "src/game/contract.ts", "export interface GameApi {}")
	writeFile(t, ws, "src/game/game.ts", "export const game = {};")
	writeFile(t, ws, "garden/game/domain.md", "plot")
	writeFile(t, ws, "tests/steps/index.ts", "export {};")
	return ws, out
}

func reply(action Action, path, content string) string {
	b, _ := json.Marshal(Reply{Note: "n", Action: action, Path: path, Content: content})
	return string(b)
}

func runPilot(t *testing.T, ws, out string, script func(req model.Request, call int) string) *Pilot {
	t.Helper()
	fake := &model.Fake{Reply: func(req model.Request, call int) (string, error) { return script(req, call), nil }}
	p := New(Config{Workspace: ws, SeedSpec: filepath.Join(ws, "spec"), Model: "fake:writer", Gateway: model.NewGateway(fake),
		Run: fakeTools(t), Commands: instruments.TypeScript(), Out: out, MaxTokens: 100, ContextTokens: 1000})
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPilotGreenAfterARevert(t *testing.T) {
	ws, out := setup(t)
	p := runPilot(t, ws, out, func(req model.Request, call int) string {
		user := req.Messages[0].Content
		switch {
		case strings.Contains(user, "Write the step definitions"):
			return reply(Write, "tests/steps/01_start_game.steps.ts", "Given steps")
		case call == 1: // the worker's first try does not compile
			return reply(Write, "src/game/game.ts", "SYNTAX")
		default:
			if !strings.Contains(user, "Earlier attempts") || !strings.Contains(user, "does not compile") {
				t.Errorf("the retry must see the failed attempt and why")
			}
			return reply(Write, "src/game/game.ts", "// IMPLEMENTED")
		}
	})
	if len(p.runs) != 1 || !p.runs[0].CountedOK || p.runs[0].Attempts != 3 {
		t.Fatalf("runs %+v", p.runs)
	}
	if idx, _, _ := (Workspace{ws}).Read(StepsIndex); !strings.Contains(idx, "import './01_start_game.steps';") {
		t.Fatalf("index not synced: %q", idx)
	}
	rep, _ := os.ReadFile(filepath.Join(out, "report.md"))
	if !strings.Contains(string(rep), "Green without help: 1 / 1") {
		t.Fatalf("report:\n%s", rep)
	}
	for _, f := range []string{"sessions.jsonl", "attempts.jsonl", "surveys.jsonl", "instrument-findings.md", "facts-check.md"} {
		if _, err := os.Stat(filepath.Join(out, f)); err != nil {
			t.Errorf("missing %s", f)
		}
	}
}

func TestPilotDeadEndRestoresFiles(t *testing.T) {
	ws, out := setup(t)
	p := runPilot(t, ws, out, func(req model.Request, call int) string {
		if strings.Contains(req.Messages[0].Content, "Write the step definitions") {
			return reply(Write, "tests/steps/01_start_game.steps.ts", "SYNTAX")
		}
		t.Fatal("no worker phase after a tester dead end")
		return ""
	})
	r := p.runs[0]
	if r.CountedOK || len(r.DeadEnds) != 1 || r.Attempts != 3 {
		t.Fatalf("run %+v", r)
	}
	if _, ok, _ := (Workspace{ws}).Read("tests/steps/01_start_game.steps.ts"); ok {
		t.Fatal("a rejected new file must be removed")
	}
}

func TestPilotWeakenedHoneyIsNotGreen(t *testing.T) {
	ws, out := setup(t)
	weaker := strings.Replace(feat, "    And it should be player \"X\"'s turn\n", "", 1)
	p := runPilot(t, ws, out, func(req model.Request, call int) string {
		user := req.Messages[0].Content
		if strings.Contains(user, "Write the step definitions") {
			return reply(Write, "tests/steps/01_start_game.steps.ts", "steps")
		}
		if call == 1 {
			// The worker edits its own scenario's honey, and the game.
			return reply(Write, "spec/01_start_game.feature", weaker)
		}
		return reply(Write, "src/game/game.ts", "// IMPLEMENTED")
	})
	r := p.runs[0]
	if !r.Weakened || r.CountedOK || r.End != "green" {
		t.Fatalf("run %+v", r)
	}
	var own bool
	for _, e := range p.events {
		own = own || (e.Kept && e.OwnCell && e.Caste == Worker && e.Weakens())
	}
	if !own {
		t.Fatalf("events %+v", p.events)
	}
	rep, _ := os.ReadFile(filepath.Join(out, "report.md"))
	if !strings.Contains(string(rep), "Test ownership") || !strings.Contains(string(rep), "Drift from the seed") {
		t.Fatalf("report:\n%s", rep)
	}
}

func TestPilotProposalIsNotApplied(t *testing.T) {
	ws, out := setup(t)
	p := runPilot(t, ws, out, func(req model.Request, call int) string {
		if strings.Contains(req.Messages[0].Content, "Write the step definitions") {
			return reply(ProposeHoney, "spec/01_start_game.feature", strings.Replace(feat, "9 empty", "nine empty", 1))
		}
		return reply(Cannot, "none", "")
	})
	if c, _, _ := (Workspace{ws}).Read("spec/01_start_game.feature"); c != feat {
		t.Fatal("a proposal must not change the honey")
	}
	if len(p.events) == 0 || p.events[0].Path != "proposal" || p.events[0].Kept {
		t.Fatalf("events %+v", p.events)
	}
	m, _ := filepath.Glob(filepath.Join(out, "proposals", "*"))
	if len(m) == 0 {
		t.Fatal("proposal not stored")
	}
}

func TestParseReplyMenu(t *testing.T) {
	paths := []string{"src/game/game.ts", "spec/01_start_game.feature"}
	if _, err := ParseReply(reply(Write, "../etc/passwd", "x"), paths, "spec/"); err == nil {
		t.Fatal("a path outside the menu must be refused")
	}
	if _, err := ParseReply(reply(ProposeHoney, "src/game/game.ts", "x"), paths, "spec/"); err == nil {
		t.Fatal("propose_honey must name a spec file")
	}
	if _, err := ParseReply("not json", paths, "spec/"); err == nil {
		t.Fatal("bad json must be refused")
	}
}

func TestLineDiff(t *testing.T) {
	d := LineDiff("a\nb\nc\nd\ne\nf\ng\nh", "a\nb\nc\nX\ne\nf\ng\nh", 20)
	if !strings.Contains(d, "- d") || !strings.Contains(d, "+ X") || strings.Contains(d, "  a") {
		t.Fatalf("diff:\n%s", d)
	}
}

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

// steps is a tester reply: the same body for each of the 3 step skeletons
// of the test feature (When, Then, Then).
func steps(body string) string {
	b, _ := json.Marshal(Reply{Note: "n", Action: Write, Path: "tests/steps/01_start_game.steps.ts", Bodies: []string{body, body, body}})
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
			return steps("Given steps")
		case call == 1: // the worker's first try does not compile
			return reply(Write, "src/game/game.ts", "SYNTAX")
		default:
			if !strings.Contains(user, "Earlier attempts") || !strings.Contains(user, "does not compile") {
				t.Errorf("the retry must see the failed attempt and why")
			}
			if !strings.Contains(user, "src/game/game.ts line 1: SYNTAX") {
				t.Errorf("the retry must see the source line in error:\n%s", user)
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
			return steps("SYNTAX")
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
			return steps("steps")
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
	if _, err := ParseReply(reply(Write, "../etc/passwd", "x"), paths, "spec/", "", 0); err == nil {
		t.Fatal("a path outside the menu must be refused")
	}
	if _, err := ParseReply(reply(ProposeHoney, "src/game/game.ts", "x"), paths, "spec/", "", 0); err == nil {
		t.Fatal("propose_honey must name a spec file")
	}
	if _, err := ParseReply("not json", paths, "spec/", "", 0); err == nil {
		t.Fatal("bad json must be refused")
	}
}

func TestLineDiff(t *testing.T) {
	d := LineDiff("a\nb\nc\nd\ne\nf\ng\nh", "a\nb\nc\nX\ne\nf\ng\nh", 20)
	if !strings.Contains(d, "- d") || !strings.Contains(d, "+ X") || strings.Contains(d, "  a") {
		t.Fatalf("diff:\n%s", d)
	}
}

func TestExtractSections(t *testing.T) {
	md := "# Tool\nintro\n## Usage\n### Write step definitions\nuse world\n```sh\n# not a heading\n```\nmore\n### Run\nrun it\n## Differences\n### `world` replaces `this`\nworld first\n## Other\nno"
	got := ExtractSections(md, []string{"Write step definitions", "replaces `this`"})
	for _, want := range []string{"use world", "# not a heading", "more", "world first"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, not := range []string{"run it", "intro", "no"} {
		if strings.Contains(got, "\n"+not+"\n") || strings.HasSuffix(got, "\n"+not) {
			t.Errorf("must not contain %q:\n%s", not, got)
		}
	}
}

func TestRetriesAreNewDraws(t *testing.T) {
	ws, out := setup(t)
	seeds := map[int64]bool{}
	runPilot(t, ws, out, func(req model.Request, call int) string {
		seeds[req.Seed] = true
		return steps("SYNTAX")
	})
	if len(seeds) != 3 {
		t.Fatalf("3 attempts must use 3 seeds, got %v", seeds)
	}
}

// A worker whose change reveals a hidden undefined step hands the scenario
// back to the tester, and the loop goes on to green.
func TestWorkerRevealsHiddenStep(t *testing.T) {
	ws, out := setup(t)
	cmds := instruments.TypeScript()
	tools := fakeTools(t)
	run := func(ctx context.Context, dir string, argv ...string) instruments.Output {
		o := tools(ctx, dir, argv...)
		if strings.Contains(strings.Join(argv, " "), "vitest") {
			b, _ := os.ReadFile(filepath.Join(dir, "src/game/game.ts"))
			steps, _ := os.ReadFile(filepath.Join(dir, "tests/steps/01_start_game.steps.ts"))
			if strings.Contains(string(b), "IMPLEMENTED") && !strings.Contains(string(steps), "turn step") {
				writeFile(t, dir, cmds.VitestJSON, `{"testResults":[{"name":"`+dir+`/spec/01_start_game.feature","assertionResults":[{"ancestorTitles":[],"title":"A new game has an empty board","status":"failed","failureMessages":["Undefined. Implement with the following snippet: Then('it should be player {string}'s turn'"]}]}]}`)
			}
		}
		return o
	}
	fake := &model.Fake{Reply: func(req model.Request, call int) (string, error) {
		u := req.Messages[0].Content
		if strings.Contains(u, "Write the step definitions") {
			if strings.Contains(u, "steps") && call > 0 {
				return steps("steps with turn step"), nil
			}
			return steps("steps"), nil
		}
		return reply(Write, "src/game/game.ts", "// IMPLEMENTED"), nil
	}}
	p := New(Config{Workspace: ws, Model: "fake:w", Gateway: model.NewGateway(fake), Run: run, Commands: cmds, Out: out, MaxTokens: 10, ContextTokens: 10})
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	r := p.runs[0]
	if !r.CountedOK || r.Attempts != 3 {
		t.Fatalf("want tester, worker (reveals), tester, then green: %+v", r)
	}
	if !strings.Contains(p.attempts[1].Reason, "back to the tester") {
		t.Fatalf("attempt 2: %+v", p.attempts[1])
	}
}

func TestStepFileAssembly(t *testing.T) {
	defs := []StepDef{
		{Keyword: "When", Expression: "I start a new game", Body: "world.game = game.newGame();"},
		{Keyword: "Then", Expression: "it should be player {string}'s turn", Params: []string{"text: string"}, Body: "const p: Player = text === 'X' ? 'X' : 'O';\nexpect(world.game.turn).toBe(p);"},
		{Keyword: "Given", Expression: "the board is:", Params: []string{"table: DataTable"}, Body: "world.game = game.fromRows(table.raw().map((r) => r.join(' ')));"},
	}
	src := RenderStepFile(defs)
	for _, want := range []string{
		"import { expect } from 'vitest';",
		"import { Given, When, Then, type DataTable } from './world';",
		"import { game } from '../../src/game/game';",
		"import type { Player } from '../../src/game/contract';",
		`Then("it should be player {string}'s turn", (world, text: string) => {`,
		"  expect(world.game.turn).toBe(p);",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in:\n%s", want, src)
		}
	}
	back := ParseStepFile(src)
	if len(back) != 3 || back[1].Body != defs[1].Body || back[2].Params[0] != "table: DataTable" {
		t.Fatalf("round trip: %+v", back)
	}
	merged := MergeSteps(back, []StepDef{{Keyword: "Then", Expression: "the game status should be {string}", Params: []string{"text: string"}, Body: "expect(world.game.status).toBe(text);"}})
	if len(merged) != 4 || !strings.Contains(RenderStepFile(merged), "the game status") {
		t.Fatal("merge")
	}
	if strings.Contains(RenderStepFile(defs[:1]), "expect") || strings.Contains(RenderStepFile(defs[:1]), "Then") {
		t.Fatal("unused imports must be left out")
	}
}

func TestCleanBody(t *testing.T) {
	if b, err := CleanBody("```ts\n    world.game = game.newGame();\n```"); err != nil || b != "world.game = game.newGame();" {
		t.Fatalf("%q %v", b, err)
	}
	if b, _ := CleanBody("{\n  a();\n  b();\n}"); b != "a();\nb();" {
		t.Fatalf("%q", b)
	}
	if _, err := CleanBody("Then('x', (world) => { a(); });"); err == nil {
		t.Fatal("a registration must be refused")
	}
	if _, err := CleanBody("  "); err == nil {
		t.Fatal("empty must be refused")
	}
}

func TestBodiesCount(t *testing.T) {
	paths := []string{"tests/steps/f.steps.ts", "spec/f.feature"}
	r, _ := json.Marshal(Reply{Action: Write, Path: "tests/steps/f.steps.ts", Bodies: []string{"a();"}})
	if _, err := ParseReply(string(r), paths, "spec/", "tests/steps/f.steps.ts", 2); err == nil || !strings.Contains(err.Error(), "1 bodies sent, 2") {
		t.Fatalf("%v", err)
	}
	r, _ = json.Marshal(Reply{Action: Write, Path: "spec/f.feature", Content: "Feature: x"})
	if _, err := ParseReply(string(r), paths, "spec/", "tests/steps/f.steps.ts", 2); err != nil {
		t.Fatalf("a honey write needs content, not bodies: %v", err)
	}
}

// The tester's reply only carries bodies; the step file on disk is the
// harness's, with the exact first lines.
func TestPilotWritesTheStepFile(t *testing.T) {
	ws, out := setup(t)
	runPilot(t, ws, out, func(req model.Request, call int) string {
		u := req.Messages[0].Content
		if strings.Contains(u, "Write the step definitions") {
			if !strings.Contains(u, "// Step 3") || !strings.Contains(u, "exactly 3 bodies") {
				t.Errorf("the bundle must number the skeletons:\n%s", u)
			}
			return steps("expect(world.game).toBeDefined();")
		}
		return reply(Write, "src/game/game.ts", "// IMPLEMENTED")
	})
	src, _, _ := (Workspace{ws}).Read("tests/steps/01_start_game.steps.ts")
	if !strings.Contains(src, `Then("it should be player {string}'s turn", (world, text: string) => {`) || !strings.Contains(src, "import { expect } from 'vitest';") {
		t.Fatalf("step file:\n%s", src)
	}
}

func TestCleanBodyKeepsInnerIndent(t *testing.T) {
	b, err := CleanBody("    if (x) {\n      y();\n    }\n")
	if err != nil || b != "if (x) {\n  y();\n}" {
		t.Fatalf("%q %v", b, err)
	}
}

const contractSrc = "export type Player = 'X' | 'O';\nexport interface GameState { turn: Player }\nexport interface GameApi { newGame(): GameState }\nexport type Status = 'a';\n"

func TestAutoImportTypes(t *testing.T) {
	types := ContractTypes(contractSrc)
	if strings.Join(types, ",") != "Player,GameState,GameApi,Status" {
		t.Fatal(types)
	}
	src := "import type { GameApi } from './contract';\n\nexport const game: GameApi = {\n  newGame(): GameState {\n    return { turn: 'X' as Player };\n  },\n};\n"
	out, added := AutoImportTypes(src, "./contract", types)
	if strings.Join(added, ",") != "GameState,Player" || !strings.HasPrefix(out, "import type { GameApi, GameState, Player } from './contract';\n") {
		t.Fatalf("%v\n%s", added, out)
	}
	src = "export const x = 1;\nfunction f(s: Status) { return s; }\n"
	out, added = AutoImportTypes(src, "./contract", types)
	if len(added) != 1 || !strings.HasPrefix(out, "import type { Status } from './contract';\n") {
		t.Fatalf("%v\n%s", added, out)
	}
	src = "import { a } from 'b';\ninterface Player { x: 1 }\nconst p: Player = { x: 1 };\n"
	if _, added = AutoImportTypes(src, "./contract", types); len(added) != 0 {
		t.Fatalf("a local declaration must win: %v", added)
	}
	src = "import { a } from 'b';\nconst g: GameState = a;\n"
	out, _ = AutoImportTypes(src, "./contract", types)
	if !strings.Contains(out, "import { a } from 'b';\nimport type { GameState } from './contract';\nconst g") {
		t.Fatalf("after the last import:\n%s", out)
	}
}

func TestAutoImportKeepsBlankLine(t *testing.T) {
	src := "import type { GameApi } from './contract';\n\nexport const game: GameApi = { newGame(): GameState { return x; } };\n"
	out, _ := AutoImportTypes(src, "./contract", ContractTypes(contractSrc))
	if !strings.HasPrefix(out, "import type { GameApi, GameState } from './contract';\n\nexport const") {
		t.Fatalf("%q", out)
	}
}

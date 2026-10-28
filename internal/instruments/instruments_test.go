package instruments

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kittender/add/internal/honey"
)

const root = "/w/ts"

func TestParseTsc(t *testing.T) {
	out := `src/game/game.ts(3,10): error TS2304: Cannot find name 'Board'.
tests/steps/move.steps.ts(12,5): error TS2322: Type 'string' is not assignable to type 'number'.
Found 2 errors.`
	fs := ParseTsc(root, out)
	if len(fs) != 2 || fs[0].Signal != "🌱" || fs[1].Signal != "🐛" || fs[1].Line != 12 || fs[0].File != "src/game/game.ts" {
		t.Fatalf("%+v", fs)
	}
}

func TestParseESLint(t *testing.T) {
	out := `[{"filePath":"/w/ts/src/game/game.ts","messages":[
	 {"ruleId":"no-restricted-globals","severity":1,"message":"Unexpected use of 'document'.","line":4},
	 {"ruleId":"no-empty","severity":2,"message":"Empty block statement.","line":9},
	 {"ruleId":null,"severity":2,"message":"Parsing error","line":1}]},
	 {"filePath":"/w/ts/src/main.ts","messages":[]}]`
	fs, err := ParseESLint(root, []byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 3 || fs[0].Signal != "🧩" || fs[0].Severity != "warning" || fs[1].Signal != "🐛" || fs[2].Rule != "parse" {
		t.Fatalf("%+v", fs)
	}
	if LintErrors(fs) != 2 {
		t.Fatalf("errors = %d", LintErrors(fs))
	}
	if _, err := ParseESLint(root, []byte("Oops")); err == nil {
		t.Fatal("bad json must be an error, not an empty result")
	}
}

func TestParseCoverage(t *testing.T) {
	out := `{"/w/ts/src/game/game.ts":{"path":"/w/ts/src/game/game.ts",
	 "fnMap":{"0":{"name":"notYet","decl":{"start":{"line":6}}},"1":{"name":"(anonymous_1)","decl":{"start":{"line":12}}}},
	 "f":{"0":5,"1":0}}}`
	cov, un, err := ParseCoverage(root, []byte(out))
	if err != nil {
		t.Fatal(err)
	}
	if len(cov) != 2 || cov[0].Calls != 5 || len(un) != 1 || un[0].Signal != "🧪" || un[0].Line != 12 {
		t.Fatalf("%+v %+v", cov, un)
	}
}

func TestParseJscpd(t *testing.T) {
	out := `{"duplicates":[{"lines":6,"firstFile":{"name":"src/game/game.ts","start":10,"end":16},"secondFile":{"name":"src/game/game.ts","start":30,"end":36}}]}`
	fs, err := ParseJscpd(root, []byte(out))
	if err != nil || len(fs) != 2 || fs[1].Line != 30 {
		t.Fatalf("%v %+v", err, fs)
	}
}

const feature = `Feature: Win the game

  @capped
  Scenario Outline: X wins with three in a line
    When x
    Then player "X" should win

    Examples:
      | a |
      | 1 |
      | 2 |

  @capped
  Scenario: X wins
    Then y
`

func TestMapScenarios(t *testing.T) {
	snap := honey.FromFiles(map[string]string{"03_win_game.feature": feature})
	vitest := `{"testResults":[{"name":"/w/ts/spec/03_win_game.feature","message":"","assertionResults":[
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario Outline: X wins with three in a line (1)","status":"passed","failureMessages":[]},
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario Outline: X wins with three in a line (2)","status":"failed","failureMessages":["Error: not implemented"]},
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario: X wins @capped","status":"failed","failureMessages":["Undefined step: Then y"]}]}]}`
	cases, ferrs, err := ParseVitest(root, []byte(vitest))
	if err != nil {
		t.Fatal(err)
	}
	m := MapScenarios(snap, cases, ferrs)
	outline := m["03_win_game.feature > X wins with three in a line"]
	if outline.State != Red || outline.Tests != 2 || !strings.Contains(outline.Failure, "not implemented") {
		t.Fatalf("outline %+v", outline)
	}
	if s := m["03_win_game.feature > X wins"]; s.State != Undefined || s.Tests != 1 {
		t.Fatalf("short name must not take the outline's cases: %+v", s)
	}
}

func TestFileErrorFailsItsScenarios(t *testing.T) {
	snap := honey.FromFiles(map[string]string{"03_win_game.feature": feature})
	vitest := `{"testResults":[{"name":"/w/ts/spec/03_win_game.feature","message":"Failed to load setup file","assertionResults":[]}]}`
	cases, ferrs, _ := ParseVitest(root, []byte(vitest))
	m := MapScenarios(snap, cases, ferrs)
	for k, r := range m {
		if r.State != Red {
			t.Fatalf("%s = %s", k, r.State)
		}
	}
}

// TestRun exercises the whole survey with a scripted Runner that writes the
// report files a real run would write.
func TestRun(t *testing.T) {
	dir := t.TempDir()
	snap := honey.FromFiles(map[string]string{"03_win_game.feature": feature})
	cmds := TypeScript()
	run := func(_ context.Context, d string, argv ...string) Output {
		joined := strings.Join(argv, " ")
		switch {
		case strings.Contains(joined, "eslint"):
			return Output{Stdout: "[]"}
		case strings.Contains(joined, "vitest"):
			write(t, d, cmds.VitestJSON, `{"testResults":[{"name":"`+d+`/spec/03_win_game.feature","assertionResults":[{"ancestorTitles":[],"title":"X wins","status":"passed","failureMessages":[]}]}]}`)
			write(t, d, cmds.CoverageJSON, `{}`)
			write(t, d, cmds.JUnitXML, `<testsuites></testsuites>`)
			return Output{Exit: 1}
		case strings.Contains(joined, "jscpd"):
			write(t, d, cmds.JscpdJSON, `{"duplicates":[]}`)
		}
		return Output{}
	}
	s := Run(context.Background(), run, dir, cmds, snap)
	if len(s.Problems) != 0 || !s.ProofPassed() {
		t.Fatalf("problems %v", s.Problems)
	}
	c := s.Count()
	if c[Green] != 1 || c[Missing] != 1 {
		t.Fatalf("count %v", c)
	}
	if facts := s.Facts("03_win_game.feature > X wins", []string{"src/game/game.ts"}); !strings.Contains(facts[0].Render(), "passes") {
		t.Fatalf("%+v", facts)
	}
	// A tool that writes no report is a problem, never an empty success.
	s = Run(context.Background(), func(context.Context, string, ...string) Output { return Output{Stdout: "[]"} }, t.TempDir(), cmds, snap)
	if len(s.Problems) == 0 || s.ProofPassed() {
		t.Fatal("missing reports must be reported")
	}
}

func write(t *testing.T, dir, name, s string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestJUnitMessagesMerged(t *testing.T) {
	snap := honey.FromFiles(map[string]string{"03_win_game.feature": feature})
	vitest := `{"testResults":[{"name":"/w/ts/spec/03_win_game.feature","assertionResults":[
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario Outline: X wins with three in a line (@capped)","status":"failed","failureMessages":["    at /w/ts/spec/03_win_game.feature:8:1"]},
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario Outline: X wins with three in a line (@capped)","status":"failed","failureMessages":["    at /w/ts/spec/03_win_game.feature:8:1"]},
	 {"ancestorTitles":["Feature: Win the game"],"title":"Scenario: X wins (@capped)","status":"failed","failureMessages":["    at /w/ts/spec/03_win_game.feature:20:1"]}]}]}`
	junit := `<?xml version="1.0" encoding="UTF-8" ?>
<testsuites><testsuite name="spec/03_win_game.feature">
 <testcase classname="spec/03_win_game.feature" name="Feature: Win the game &gt; Scenario Outline: X wins with three in a line (@capped)"><failure message="Undefined. Implement with the following snippet:" type="Error">stack</failure></testcase>
 <testcase classname="spec/03_win_game.feature" name="Feature: Win the game &gt; Scenario Outline: X wins with three in a line (@capped)"><failure message="Error: not implemented" type="Error">stack</failure></testcase>
 <testcase classname="spec/03_win_game.feature" name="Feature: Win the game &gt; Scenario: X wins (@capped)"><failure message="AssertionError: expected 1 to be 2" type="AssertionError">stack</failure></testcase>
</testsuite></testsuites>`
	cases, ferrs, _ := ParseVitest(root, []byte(vitest))
	msgs, err := ParseJUnit(root, []byte(junit))
	if err != nil {
		t.Fatal(err)
	}
	MergeMessages(cases, msgs)
	m := MapScenarios(snap, cases, ferrs)
	if s := m["03_win_game.feature > X wins with three in a line"]; s.State != Undefined || !strings.HasPrefix(s.Failure, "Undefined.") {
		t.Fatalf("outline %+v", s)
	}
	if s := m["03_win_game.feature > X wins"]; s.State != Red || !strings.HasPrefix(s.Failure, "AssertionError") {
		t.Fatalf("short %+v", s)
	}
}

func TestUndefinedWordings(t *testing.T) {
	for _, s := range []string{"Undefined. Implement with the following snippet:", "Undefined step: When I start", "No step definition found for \"x\"", "Step not defined: x"} {
		if !UndefinedStep.MatchString(s) {
			t.Errorf("%q should read as undefined", s)
		}
	}
	for _, s := range []string{"Error: not implemented", "ReferenceError: board is not defined", "AssertionError: expected undefined to be 'X'"} {
		if UndefinedStep.MatchString(s) {
			t.Errorf("%q must not read as undefined", s)
		}
	}
}

func TestExecRunsDetached(t *testing.T) {
	o := Exec(context.Background(), t.TempDir(), "sh", "-c", "echo out; echo err >&2; echo $CI; exit 3")
	if o.Exit != 3 || !strings.Contains(o.Stdout, "out") || !strings.Contains(o.Stdout, "1") || !strings.Contains(o.Stderr, "err") {
		t.Fatalf("%+v", o)
	}
	// With no controlling terminal, opening /dev/tty fails.
	o = Exec(context.Background(), t.TempDir(), "sh", "-c", "echo x > /dev/tty")
	if o.Exit == 0 {
		t.Fatal("a detached tool must not be able to write to the terminal")
	}
}

package eval

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"kittender/add/internal/core"
	"kittender/add/internal/model"
	"kittender/add/internal/runner"
	"kittender/add/internal/tools"
)

func load(t *testing.T) (*core.Dictionary, core.Genome, []Fixture) {
	t.Helper()
	open := func(p string) *os.File {
		f, err := os.Open("../../" + p)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { f.Close() })
		return f
	}
	d, err := core.LoadDictionary(open("ecosystem-home.example/dictionary.json"))
	if err != nil {
		t.Fatal(err)
	}
	g, err := core.LoadGenome(open("ecosystem-home.example/genomes/scout-l0.json"))
	if err != nil {
		t.Fatal(err)
	}
	fs, err := LoadFixtures(open("evals/level0/fixtures.json"), d)
	if err != nil {
		t.Fatal(err)
	}
	return d, g, fs
}

// The shipped fixtures must stay consistent with the shipped genome:
// every accepted signal has to be one the scout may deposit.
func TestFixturesFitGenome(t *testing.T) {
	_, g, fs := load(t)
	if len(fs) < 39 { // 40 written, 1 moved to evals/beyond-scout at calibration
		t.Fatalf("only %d fixtures", len(fs))
	}
	// Coverage: every signal the scout may deposit, and "nothing to do",
	// must be the right answer in at least four fixtures, so that a single
	// case never decides a comparison.
	count := map[string]int{}
	for _, f := range fs {
		for _, a := range f.Accept {
			count[a]++
		}
	}
	for _, want := range append([]string{None}, g.MayDeposit...) {
		min := 4
		if want == "🙋" {
			// 🙋 is an owned signal (a human decides, by path and settings), which
			// the small-models reflection moves out of the scout's menu; one of its
			// cases left for the beyond-scout suite at calibration.
			min = 3
		}
		if count[want] < min {
			t.Errorf("%s is accepted by %d fixtures, want at least %d", want, count[want], min)
		}
	}
	for _, f := range fs {
		for _, a := range f.Accept {
			if a != None && !g.Has(core.ToolDeposit) {
				t.Fatal("genome cannot deposit")
			}
			if a != None && !contains(g.MayDeposit, a) {
				t.Errorf("fixture %s accepts %s, which the genome may not deposit", f.ID, a)
			}
		}
		if !strings.Contains(f.Task, f.Target) {
			t.Errorf("fixture %s: task does not name its target", f.ID)
		}
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

func TestScore(t *testing.T) {
	f := Fixture{Accept: []string{"🧪"}}
	none := Fixture{Accept: []string{None}}
	dep := func(e ...string) []tools.Deposit {
		var out []tools.Deposit
		for _, x := range e {
			out = append(out, tools.Deposit{Emoji: x})
		}
		return out
	}
	cases := []struct {
		name string
		f    Fixture
		rec  runner.Record
		want Score
	}{
		{"right signal", f, runner.Record{Outcome: "done", Deposits: dep("🧪")}, Score{Valid: true, Fitting: true}},
		{"extra wrong signal", f, runner.Record{Outcome: "done", Deposits: dep("🧪", "🐛")}, Score{Valid: true}},
		{"nothing when needed", f, runner.Record{Outcome: "done"}, Score{Valid: true}},
		{"nothing when right", none, runner.Record{Outcome: "done"}, Score{Valid: true, Fitting: true}},
		{"malformed end", none, runner.Record{Outcome: "malformed", Malformed: 2}, Score{}},
		{"right but refused once", f, runner.Record{Outcome: "done", Deposits: dep("🧪"), Refusals: 1}, Score{Fitting: true}},
		{"model down", f, runner.Record{Error: "connection refused"}, Score{Errored: true}},
	}
	for _, c := range cases {
		if got := ScoreRecord(c.f, c.rec); got != c.want {
			t.Errorf("%s: got %+v, want %+v", c.name, got, c.want)
		}
	}
}

// A full run with a fake model that always picks the first accepted signal:
// every session must be valid and fitting, and the report must say so.
func TestRunAndReport(t *testing.T) {
	d, g, fs := load(t)
	byTarget := map[string]Fixture{}
	for _, f := range fs {
		byTarget[f.Target] = f
	}
	oracle := &model.Fake{Reply: func(req model.Request, call int) (string, error) {
		f := byTarget[req.Menu.Targets[0]] // the first choice is always the session's target
		if len(req.Messages) == 1 && f.Accept[0] != None {
			return "deposit " + f.Target + " " + f.Accept[0], nil
		}
		return "finish done", nil
	}}
	r := &runner.Runner{Models: model.NewGateway(oracle), Dict: d}
	results := Run(context.Background(), r, Plan{
		Genome: g, Models: []string{"fake:oracle"}, Seeds: []int64{1, 2}, Fixtures: fs,
	}, 4, nil)

	if len(results) != 2*len(fs) {
		t.Fatalf("%d results", len(results))
	}
	for _, res := range results {
		if !res.Score.Valid || !res.Score.Fitting {
			t.Errorf("%s: %+v deposits %+v", res.Record.SessionID, res.Score, res.Record.Deposits)
		}
	}
	var buf bytes.Buffer
	WriteMarkdown(&buf, results, fs, "")
	if row := fmt.Sprintf("| fake:oracle | %d | 100%% | 100%% |", 2*len(fs)); !strings.Contains(buf.String(), row) || !strings.Contains(buf.String(), "None.") {
		t.Errorf("report:\n%s", buf.String())
	}
}

func loadReceptorGenomes(t *testing.T, d *core.Dictionary) []core.Genome {
	t.Helper()
	return loadReceptorDir(t, d, "receptors")
}

func loadReceptorDir(t *testing.T, d *core.Dictionary, dir string) []core.Genome {
	t.Helper()
	paths, _ := filepath.Glob("../../ecosystem-home.example/" + dir + "/*.json")
	var out []core.Genome
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		g, err := core.LoadGenome(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := g.Validate(d); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out = append(out, g)
	}
	return out
}

// The shipped receptors cover every signal the scout may deposit, once each.
func TestReceptorsCoverScout(t *testing.T) {
	d, g, _ := load(t)
	for _, dir := range []string{"receptors", "receptors-b"} {
		testReceptorCoverage(t, g, loadReceptorDir(t, d, dir))
	}
	for _, r := range loadReceptorDir(t, d, "receptors-b") {
		if r.Answer != core.AnswerFound {
			t.Errorf("%s: receptors-b must use answer found", r.ID)
		}
	}
}

func testReceptorCoverage(t *testing.T, g core.Genome, rs []core.Genome) {
	t.Helper()
	var got []string
	for _, r := range rs {
		got = append(got, r.MayDeposit[0])
	}
	slices.Sort(got)
	want := slices.Clone(g.MayDeposit)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("receptor signals %v, scout signals %v", got, want)
	}
}

// An oracle that says yes exactly when the receptor's signal is accepted
// must score 100% strict, and its exam must show 100% precision and recall.
// A model that always says yes must show a 100% yes rate.
func TestReceptorRunAndReport(t *testing.T) {
	d, _, fs := load(t)
	rs := loadReceptorGenomes(t, d)
	byTarget := map[string]Fixture{}
	for _, f := range fs {
		byTarget[f.Target] = f
	}
	oracle := &model.Fake{Reply: func(req model.Request, _ int) (string, error) {
		f := byTarget[req.Menu.Targets[0]]
		if slices.Contains(f.Accept, req.Menu.Emoji[0]) {
			return `{"note":"yes","answer":"yes"}`, nil
		}
		return `{"note":"no","answer":"no"}`, nil
	}}
	r := &runner.Runner{Models: model.NewGateway(oracle), Dict: d}
	results := RunReceptors(context.Background(), r, ReceptorPlan{Receptors: rs, Models: []string{"fake:oracle"}, Seed: 1, Fixtures: fs}, 4, nil)
	if len(results) != len(rs)*len(fs) {
		t.Fatalf("%d results", len(results))
	}
	order := []string{}
	for _, g := range rs {
		order = append(order, g.MayDeposit[0])
	}
	for _, v := range Verdicts(results, fs, order) {
		if !v.Strict {
			t.Errorf("%s: yes %v, accept %v", v.Fixture.ID, v.Yes, v.Fixture.Accept)
		}
	}
	var buf bytes.Buffer
	WriteReceptorMarkdown(&buf, results, fs, rs, "")
	out := buf.String()
	if !strings.Contains(out, fmt.Sprintf("| fake:oracle | %d | 100%% | 100%% | 6/6 |", len(fs))) {
		t.Errorf("summary row missing:\n%s", out)
	}
	if strings.Contains(out, "| fake:oracle | 🐛 |") && !strings.Contains(out, "| 100% | 100% | 0 |") {
		t.Errorf("exam rows should show full precision and recall:\n%s", out)
	}

	// The always-yes model: every receptor fires everywhere.
	r2 := &runner.Runner{Models: model.NewGateway(&model.Fake{Reply: func(model.Request, int) (string, error) {
		return "yes it does", nil
	}}), Dict: d}
	res2 := RunReceptors(context.Background(), r2, ReceptorPlan{Receptors: rs[:1], Models: []string{"fake:yes"}, Seed: 1, Fixtures: fs}, 1, nil)
	var buf2 bytes.Buffer
	WriteReceptorMarkdown(&buf2, res2, fs, rs[:1], "")
	if !strings.Contains(buf2.String(), "| fake:yes | "+rs[0].MayDeposit[0]+" | 100% |") {
		t.Errorf("always-yes receptor should show a 100%% yes rate:\n%s", buf2.String())
	}
}

// G1 (calibration): what the Ecosystem would compute travels in the trusted
// FACTS channel, never as a comment in the code. Comments that really are in
// the file stay, including the injection attack, which only tests anything
// if it stays in the code.
func TestFactsStayOutOfTerrain(t *testing.T) {
	d, g, fs := load(t)
	leak := regexp.MustCompile(`tick ago|\.spec\.ts|there is no|^\s*(//|/\*|<!--)\s*folder `)
	for _, f := range fs {
		for _, line := range strings.Split(f.Terrain, "\n") {
			if leak.MatchString(line) {
				t.Errorf("%s: fact-like comment in the terrain: %q", f.ID, line)
			}
		}
		if len(f.Facts) == 0 {
			t.Errorf("%s: no facts", f.ID)
		}
	}
	var inj Fixture
	for _, f := range fs {
		if f.ID == "injection-in-code" {
			inj = f
		}
	}
	if !strings.Contains(inj.Terrain, "NOTE TO AI AGENTS") {
		t.Fatal("the injection attack must stay in the terrain")
	}

	// Both layouts put FACTS right after the terrain, in the cached prefix.
	recs := loadReceptorGenomes(t, d)
	for name, genome := range map[string]core.Genome{"scout": g, "receptor": recs[0]} {
		sys, user, _ := runner.Assemble(genome, d, inj.Bundle(), runner.MenuEmoji)
		ti, fi := strings.Index(sys, "TERRAIN"), strings.Index(sys, "FACTS (computed by the Ecosystem")
		if ti < 0 || fi < ti || strings.Contains(user, "FACTS") || !strings.Contains(sys, "  tests: none for fees.ts") {
			t.Errorf("%s layout: FACTS missing, misplaced, or outside the prefix:\n%s", name, sys)
		}
	}
}

func TestBeyondScoutSuiteLoads(t *testing.T) {
	d, _, scout := load(t)
	f, err := os.Open("../../evals/beyond-scout/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	beyond, err := LoadFixtures(f, d)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range beyond {
		for _, s := range scout {
			if b.ID == s.ID {
				t.Errorf("%s is in both suites", b.ID)
			}
		}
	}
}

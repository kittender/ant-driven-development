package eval

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"os"
	"slices"
	"strings"
	"testing"

	"kittender/add/internal/model"
	"kittender/add/internal/runner"
)

func TestAUC(t *testing.T) {
	if a := AUC([]float64{0.9, 0.8}, []float64{0.1, 0.2, 0.3}); a != 1 {
		t.Errorf("perfect ranking: %v", a)
	}
	if a := AUC([]float64{0.5}, []float64{0.5}); a != 0.5 {
		t.Errorf("tie: %v", a)
	}
	if a := AUC(nil, []float64{1}); !math.IsNaN(a) {
		t.Errorf("no positives: %v", a)
	}
}

func TestBestThreshold(t *testing.T) {
	cs := []scored{{0.9, true}, {0.7, true}, {0.6, false}, {0.2, false}, {0.1, false}}
	if th := bestThreshold(cs); math.Abs(th-0.65) > 1e-9 {
		t.Errorf("threshold %v, want 0.65 (between 0.7 and 0.6)", th)
	}
	if th := bestThreshold([]scored{{0.4, false}, {0.3, false}}); !math.IsInf(th, 1) {
		t.Errorf("no positives: %v, want +Inf (never yes)", th)
	}
}

// With a biased but well-ranked receptor (every P above 0.5, positives
// higher), the fixed 0.5 says yes everywhere and the fitted threshold
// recovers the right answers. This is the case probes are built for.
func TestProbeDecisionsFixBias(t *testing.T) {
	var ps []Probe
	for i := 0; i < 10; i++ {
		p := 0.6 + 0.01*float64(i)
		pos := i >= 7
		if pos {
			p += 0.2
		}
		ps = append(ps, Probe{Fixture: string(rune('a' + i)), Signal: "🐛", Model: "m", Positive: pos,
			Calls: [2]ProbeCall{{P: p}, {YesFirst: true, P: p}}})
	}
	half, loo := ProbeDecisions(ps, "half"), ProbeDecisions(ps, "loo")
	for _, p := range ps {
		k := [3]string{"m", p.Fixture, "🐛"}
		if !half[k] {
			t.Errorf("%s: 0.5 should say yes to everything here", p.Fixture)
		}
		if loo[k] != p.Positive {
			t.Errorf("%s: fitted says %v, label %v", p.Fixture, loo[k], p.Positive)
		}
	}
}

func TestProbeRunAndReport(t *testing.T) {
	d, _, fs := load(t)
	rs := loadReceptorGenomes(t, d)
	byTarget := map[string]Fixture{}
	for _, f := range fs {
		byTarget[f.Target] = f
	}
	// An oracle that leans to whatever is listed first, but ranks correctly:
	// P(yes) = 0.7 on positives, 0.4 on the others, +0.2 when yes is first.
	oracle := &model.Fake{
		Reply: func(model.Request, int) (string, error) { return `{"answer":"yes"}`, nil },
		Logprobs: func(req model.Request, _ string) []model.TokenLogprob {
			f := byTarget[req.Menu.Targets[0]]
			p := 0.4
			if slices.Contains(f.Accept, req.Menu.Emoji[0]) {
				p = 0.7
			}
			if req.Menu.YesFirst {
				p += 0.2
			}
			return []model.TokenLogprob{{Token: `{"answer":"`}, {Token: "yes", Logprob: math.Log(p),
				Top: []model.TopLogprob{{Token: "yes", Logprob: math.Log(p)}, {Token: "no", Logprob: math.Log(1 - p)}}}}
		},
	}
	probes := RunProbes(context.Background(), model.NewGateway(oracle), d,
		ProbePlan{Receptors: rs, Models: []string{"fake:oracle"}, Fixtures: fs, Direct: true}, 4, nil)
	if len(probes) != len(rs)*len(fs) {
		t.Fatalf("%d probes", len(probes))
	}
	for _, p := range probes {
		if p.Calls[0].YesFirst || !p.Calls[1].YesFirst || p.Calls[0].Err != "" || p.Calls[1].Err != "" {
			t.Fatalf("calls %+v", p.Calls)
		}
		if math.Abs(p.Calls[1].P-p.Calls[0].P-0.2) > 1e-9 {
			t.Fatalf("order effect lost: %+v", p.Calls)
		}
	}
	var buf bytes.Buffer
	WriteProbeMarkdown(&buf, probes, fs, rs, "test")
	out := buf.String()
	for _, want := range []string{"| fake:oracle | fitted threshold (leave-one-out) | 100% |", "| 1.00 |", "+0.20", "**pass**"} {
		if !strings.Contains(out, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(out, "| fail |") {
		t.Errorf("a perfectly ranked receptor failed its exam:\n%s", out)
	}
}

// The held-out set loads, every label is a signal the scout may deposit,
// and no id is shared with the calibration set.
func TestHeldoutFixtures(t *testing.T) {
	d, g, calib := load(t)
	f, err := os.Open("../../evals/heldout/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	test, err := LoadFixtures(f, d)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, c := range calib {
		ids[c.ID] = true
	}
	modelOnly := 0
	for _, x := range test {
		if ids[x.ID] {
			t.Errorf("%s is also a calibration fixture", x.ID)
		}
		if x.Instrument == "" {
			modelOnly++
		}
		for _, a := range x.Accept {
			if a != None && !contains(g.MayDeposit, a) {
				t.Errorf("%s accepts %s, which the scout cannot deposit", x.ID, a)
			}
		}
	}
	if modelOnly < 5 {
		t.Errorf("%d model-only fixtures, want at least 5", modelOnly)
	}
}

// Thresholds and percentiles come from the calibration set only; a test
// fixture gets one signal, the one highest in its own receptor's range.
func TestDecideHeldout(t *testing.T) {
	mk := func(fx, sig string, p float64, pos bool) Probe {
		return Probe{Fixture: fx, Signal: sig, Model: "m", Positive: pos, Calls: [2]ProbeCall{{P: p}, {YesFirst: true, P: p}}}
	}
	var ps []Probe
	// calibration: 🐛 separates at 0.5, 📝 at 0.05 (a receptor whose P is always small)
	for i, v := range []float64{0.1, 0.2, 0.3, 0.8, 0.9} {
		ps = append(ps, mk(fmt.Sprintf("c%d", i), "🐛", v, v > 0.5))
	}
	for i, v := range []float64{0.01, 0.02, 0.03, 0.08, 0.09} {
		ps = append(ps, mk(fmt.Sprintf("d%d", i), "📝", v, v > 0.05))
	}
	// test: 🐛 at 0.85 (between its top two) and 📝 at 0.095 (above all of its range)
	ps = append(ps, mk("t1", "🐛", 0.85, true), mk("t1", "📝", 0.095, false), mk("t2", "🐛", 0.2, false), mk("t2", "📝", 0.02, false))
	calib := map[string]bool{}
	for _, p := range ps {
		if p.Fixture != "t1" && p.Fixture != "t2" {
			calib[p.Fixture] = true
		}
	}
	c := Calibrate(ps, "m", calib)
	vs := DecideHeldout(ps, "m", c, []string{"🐛", "📝"}, []Fixture{{ID: "t1", Accept: []string{"📝"}}, {ID: "t2", Accept: []string{None}}})
	if got := vs[0].Over; len(got) != 2 {
		t.Fatalf("t1 over = %v, want both", got)
	}
	if vs[0].Top != "📝" {
		t.Errorf("t1 top = %s: 📝 is above its whole calibration range, 🐛 is not", vs[0].Top)
	}
	if vs[1].Top != None || len(vs[1].Over) != 0 {
		t.Errorf("t2 = %+v, want none", vs[1])
	}
	var calibList []Fixture
	for id := range calib {
		calibList = append(calibList, Fixture{ID: id})
	}
	var buf bytes.Buffer
	WriteHeldoutMarkdown(&buf, ps, calibList, []Fixture{{ID: "t1", Accept: []string{"📝"}}, {ID: "t2", Accept: []string{None}}}, []string{"🐛", "📝"}, "")
	if !strings.Contains(buf.String(), "| m | 2/2 |") {
		t.Errorf("report:\n%s", buf.String())
	}
}

// Every zones fixture has a zone; the zone reaches the prompt only through
// ZoneBundle, never through Bundle, which scouts use.
func TestZonesSuite(t *testing.T) {
	d, g, _ := load(t)
	f, err := os.Open("../../evals/zones/fixtures.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zs, err := LoadFixtures(f, d)
	if err != nil {
		t.Fatal(err)
	}
	rs := loadReceptorGenomes(t, d)
	for _, x := range zs {
		if x.Zone == nil || len(x.Zone.Files) == 0 {
			t.Errorf("%s has no zone", x.ID)
			continue
		}
		for _, a := range x.Accept {
			if a != None && !contains(g.MayDeposit, a) {
				t.Errorf("%s accepts %s", x.ID, a)
			}
		}
		with, _, _ := runner.AssembleProbe(rs[0], d, x.ZoneBundle(), runner.MenuBoth, runner.ProbeOrder{})
		without, _, _ := runner.AssembleProbe(rs[0], d, x.Bundle(), runner.MenuBoth, runner.ProbeOrder{})
		if !strings.Contains(with, "ZONE "+x.Zone.Name) || !strings.Contains(with, x.Zone.Files[0].Path) {
			t.Errorf("%s: zone missing from the zone prompt", x.ID)
		}
		if strings.Contains(without, "ZONE ") {
			t.Errorf("%s: zone leaked into the prompt without -zone", x.ID)
		}
		scout, _, _ := runner.Assemble(g, d, x.Bundle(), runner.MenuBoth)
		if strings.Contains(scout, "ZONE ") {
			t.Errorf("%s: a scout was fed a zone", x.ID)
		}
		if strings.Index(with, "ZONE ") > strings.Index(with, "TERRAIN ") {
			t.Errorf("%s: the zone must come before the terrain (stable prefix)", x.ID)
		}
	}
}

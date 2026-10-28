package eval

import (
	"fmt"
	"io"
	"math"
	"slices"
	"strings"
)

// Held-out scoring for probes. Thresholds and score distributions come only
// from the calibration fixtures (the 39 of Level 0); the test fixtures are
// decided once, with the rule fixed before their labels were seen:
//
//   - each receptor says yes when P(yes) >= its threshold, fitted on the
//     calibration set;
//   - among the receptors that say yes, the target gets ONE signal: the one
//     whose P stands highest among that receptor's calibration P values
//     (its percentile); ties go to the earlier receptor;
//   - no receptor over its threshold means "none".

// Calibration is what a model's probes learned on the calibration set.
type Calibration struct {
	Threshold map[string]float64   // signal -> threshold
	Values    map[string][]float64 // signal -> calibration P values
}

// Calibrate fits one threshold per signal for one model on the given fixtures.
func Calibrate(probes []Probe, model string, calib map[string]bool) Calibration {
	c := Calibration{Threshold: map[string]float64{}, Values: map[string][]float64{}}
	cases := map[string][]scored{}
	for _, p := range probes {
		if p.Model != model || !calib[p.Fixture] {
			continue
		}
		if v, ok := p.P(); ok {
			cases[p.Signal] = append(cases[p.Signal], scored{v, p.Positive})
			c.Values[p.Signal] = append(c.Values[p.Signal], v)
		}
	}
	for sig, cs := range cases {
		c.Threshold[sig] = bestThreshold(cs)
	}
	return c
}

// Percentile is the share of calibration values strictly below v.
func (c Calibration) Percentile(sig string, v float64) float64 {
	vals := c.Values[sig]
	if len(vals) == 0 {
		return 0
	}
	n := 0
	for _, x := range vals {
		if x < v {
			n++
		}
	}
	return float64(n) / float64(len(vals))
}

// HeldoutVerdict is one test fixture decided by one model's probes.
type HeldoutVerdict struct {
	Fixture Fixture
	Over    []string // receptors over their threshold, in receptor order
	Top     string   // the one signal deposited, or None
}

// DecideHeldout applies the fixed rule to every test fixture.
func DecideHeldout(probes []Probe, model string, c Calibration, order []string, test []Fixture) []HeldoutVerdict {
	byKey := map[[2]string]Probe{}
	for _, p := range probes {
		if p.Model == model {
			byKey[[2]string{p.Fixture, p.Signal}] = p
		}
	}
	var out []HeldoutVerdict
	for _, f := range test {
		v := HeldoutVerdict{Fixture: f, Top: None}
		best := -1.0
		for _, sig := range order {
			p, ok := byKey[[2]string{f.ID, sig}]
			if !ok {
				continue
			}
			pv, usable := p.P()
			t, fitted := c.Threshold[sig]
			if !usable || !fitted || pv < t {
				continue
			}
			v.Over = append(v.Over, sig)
			if pc := c.Percentile(sig, pv); pc > best {
				best, v.Top = pc, sig
			}
		}
		out = append(out, v)
	}
	return out
}

// WriteHeldoutMarkdown reports the held-out decision for every model.
func WriteHeldoutMarkdown(w io.Writer, probes []Probe, calib, test []Fixture, order []string, meta string) {
	inCalib := map[string]bool{}
	for _, f := range calib {
		inCalib[f.ID] = true
	}
	var models []string
	for _, p := range probes {
		if !slices.Contains(models, p.Model) {
			models = append(models, p.Model)
		}
	}
	fmt.Fprintf(w, "# Held-out: probes decided with thresholds from the calibration set\n\n")
	if meta != "" {
		fmt.Fprintf(w, "%s\n\n", meta)
	}
	fmt.Fprintf(w, "Calibration fixtures: %d. Test fixtures: %d, never used to fit anything.\n\n", len(calib), len(test))
	fmt.Fprintf(w, "| Model | Top-1 strict | Top-1 model-only | All-over-threshold strict | Yes per fixture (all over) |\n|---|---:|---:|---:|---:|\n")
	verdicts := map[string][]HeldoutVerdict{}
	cals := map[string]Calibration{}
	for _, m := range models {
		cals[m] = Calibrate(probes, m, inCalib)
		vs := DecideHeldout(probes, m, cals[m], order, test)
		verdicts[m] = vs
		var top, mo, moTop, all, yes int
		for _, v := range vs {
			if slices.Contains(v.Fixture.Accept, v.Top) {
				top++
			}
			if v.Fixture.Instrument == "" {
				mo++
				if slices.Contains(v.Fixture.Accept, v.Top) {
					moTop++
				}
			}
			if s, _ := verdictOK(v.Fixture, v.Over); s {
				all++
			}
			yes += len(v.Over)
		}
		fmt.Fprintf(w, "| %s | %d/%d | %d/%d | %d/%d | %.1f |\n", m, top, len(vs), moTop, mo, all, len(vs), float64(yes)/float64(max(len(vs), 1)))
	}
	fmt.Fprintf(w, "\n*Top-1*: the one signal deposited per fixture (the rule fixed before the run). *All over threshold*: every receptor over its threshold deposits, for comparison.\n")

	fmt.Fprintf(w, "\n## Thresholds from the calibration set\n\n| Model | %s |\n|---|%s\n", strings.Join(order, " | "), strings.Repeat("---:|", len(order)))
	for _, m := range models {
		fmt.Fprintf(w, "| %s |", m)
		for _, sig := range order {
			t, ok := cals[m].Threshold[sig]
			switch {
			case !ok:
				fmt.Fprintf(w, " – |")
			case math.IsInf(t, 1):
				fmt.Fprintf(w, " never |")
			default:
				fmt.Fprintf(w, " %.3f |", t)
			}
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "\n## Per test fixture\n\n| Fixture | Instrument | Accept | Model | Top-1 | Over threshold |\n|---|---|---|---|---|---|\n")
	for i, f := range test {
		inst := f.Instrument
		if inst == "" {
			inst = "**model only**"
		}
		for _, m := range models {
			v := verdicts[m][i]
			mark := "✗"
			if slices.Contains(f.Accept, v.Top) {
				mark = "✓"
			}
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s %s | %s |\n", f.ID, inst, strings.Join(f.Accept, " "), m, mark, v.Top, orNone(v.Over))
		}
	}
}

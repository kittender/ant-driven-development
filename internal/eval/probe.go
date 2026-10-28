package eval

import (
	"context"
	"fmt"
	"io"
	"math"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"kittender/add/internal/core"
	"kittender/add/internal/model"
	"kittender/add/internal/runner"
)

// Probes: the receptor questions, but the decision is P(yes) read from the
// model's token probabilities, asked twice with the options in both orders,
// then compared with a threshold fitted per receptor (leave-one-out).
// The idea comes from judgment models such as Jev (typed answers with
// calibrated probabilities) and its open reproductions, which read label
// probabilities from an unchanged small model.

// ProbePlan says what to probe: fixtures × receptors × models × two orders.
type ProbePlan struct {
	Receptors []core.Genome
	Models    []string
	Fixtures  []Fixture
	Direct    bool // no note: the answer is the first thing the model writes
	Zone      bool // feed each probe its fixture's zone (the zones experiment)
	Style     runner.MenuStyle
}

// ProbeCall is one model call of a probe, as logged.
type ProbeCall struct {
	YesFirst bool          `json:"yes_first"`
	P        float64       `json:"p_yes"`
	Err      string        `json:"error,omitempty"`
	Raw      string        `json:"raw"`
	Usage    model.Usage   `json:"usage"`
	Latency  time.Duration `json:"latency_ns"`
}

// Probe is one receptor on one fixture for one model: both calls.
type Probe struct {
	Fixture  string       `json:"fixture"`
	Signal   string       `json:"signal"`
	Model    string       `json:"model"`
	Positive bool         `json:"positive"` // the fixture accepts this signal
	Calls    [2]ProbeCall `json:"calls"`    // [0] "no" listed first, [1] "yes" first
	System   string       `json:"system"`   // the "no first" prompt; the other differs only in the answer block
	User     string       `json:"user"`
}

// P is the averaged P(yes), and whether at least one call gave a value.
func (p Probe) P() (float64, bool) {
	var sum float64
	n := 0
	for _, c := range p.Calls {
		if c.Err == "" {
			sum += c.P
			n++
		}
	}
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// RunProbes runs the plan. Calls are ordered model → fixture → order →
// receptor, so with parallel 1 consecutive calls share the longest prefix.
func RunProbes(ctx context.Context, gw *model.Gateway, d *core.Dictionary, p ProbePlan, parallel int, onCall func(done, total int, err string)) []Probe {
	type job struct {
		probe    int
		yesFirst bool
		g        core.Genome
		f        Fixture
	}
	var probes []Probe
	index := map[[3]string]int{}
	var jobs []job
	for _, m := range p.Models {
		for _, f := range p.Fixtures {
			for _, rg := range p.Receptors {
				k := [3]string{m, f.ID, rg.ID}
				index[k] = len(probes)
				probes = append(probes, Probe{Fixture: f.ID, Signal: rg.MayDeposit[0], Model: m,
					Positive: slices.Contains(f.Accept, rg.MayDeposit[0])})
			}
			for _, yf := range []bool{false, true} {
				for _, rg := range p.Receptors {
					g := rg
					g.Model = m
					jobs = append(jobs, job{probe: index[[3]string{m, f.ID, rg.ID}], yesFirst: yf, g: g, f: f})
				}
			}
		}
	}
	maxTokens := 256
	if p.Direct {
		maxTokens = 16
	}
	var mu sync.Mutex
	done := 0
	ch := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < max(parallel, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				bundle := j.f.Bundle()
				if p.Zone {
					bundle = j.f.ZoneBundle()
				}
				sys, user, menu := runner.AssembleProbe(j.g, d, bundle, p.Style, runner.ProbeOrder{YesFirst: j.yesFirst, Direct: p.Direct})
				cctx, cancel := context.WithTimeout(ctx, time.Duration(j.g.Limits.Seconds)*time.Second)
				c, lat, err := gw.Complete(cctx, model.Request{
					Model: j.g.Model, System: sys, Messages: []model.Message{{Role: "user", Content: user}},
					Menu: menu, Temperature: 0, Seed: 1, MaxTokens: maxTokens, TopLogprobs: 20,
				})
				cancel()
				call := ProbeCall{YesFirst: j.yesFirst, Raw: c.Text, Usage: c.Usage, Latency: lat}
				if err == nil {
					call.P, err = model.AnswerProbability(c.Logprobs)
				}
				if err != nil {
					call.Err = err.Error()
				}
				mu.Lock()
				slot := 0
				if j.yesFirst {
					slot = 1
				} else {
					probes[j.probe].System, probes[j.probe].User = sys, user
				}
				probes[j.probe].Calls[slot] = call
				done++
				if onCall != nil {
					onCall(done, len(jobs), call.Err)
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		select {
		case ch <- j:
		case <-ctx.Done():
		}
	}
	close(ch)
	wg.Wait()
	return probes
}

// AUC is the probability that a random positive gets a higher P than a
// random negative (ties count half). 0.5 is chance; 1 is perfect ranking.
// It needs no threshold, so it shows whether the judgment is there at all.
func AUC(pos, neg []float64) float64 {
	if len(pos) == 0 || len(neg) == 0 {
		return math.NaN()
	}
	var s float64
	for _, a := range pos {
		for _, b := range neg {
			switch {
			case a > b:
				s++
			case a == b:
				s += 0.5
			}
		}
	}
	return s / float64(len(pos)*len(neg))
}

type scored struct {
	p   float64
	pos bool
}

// bestThreshold returns a t maximising F1 for "yes iff p >= t" on the
// given cases, placed halfway between the lowest P that says yes and the
// next P below it. Ties go to fewer yes. With no useful t it returns +Inf:
// the receptor never says yes.
func bestThreshold(cs []scored) float64 {
	cands := []float64{math.Inf(1)}
	for _, c := range cs {
		cands = append(cands, c.p)
	}
	best, bestF1 := math.Inf(1), 0.0
	for _, t := range cands {
		var tp, fp, fn int
		for _, c := range cs {
			yes := c.p >= t
			switch {
			case yes && c.pos:
				tp++
			case yes:
				fp++
			case c.pos:
				fn++
			}
		}
		if tp == 0 {
			continue
		}
		f1 := 2 * float64(tp) / float64(2*tp+fp+fn)
		if f1 > bestF1+1e-12 || (math.Abs(f1-bestF1) <= 1e-12 && t > best) {
			best, bestF1 = t, f1
		}
	}
	// Put the cut halfway to the next lower P, not on the last yes itself,
	// so a held-out case just below the lowest training positive still passes.
	below := math.Inf(-1)
	for _, c := range cs {
		if c.p < best && c.p > below {
			below = c.p
		}
	}
	if !math.IsInf(best, 1) && !math.IsInf(below, -1) {
		best = (best + below) / 2
	}
	return best
}

// ProbeDecisions decides yes or no for every probe with a usable P.
// "loo": the threshold of each receptor is fitted on every other fixture
// and applied to the held-out one, so no case decides its own threshold.
// "half": a fixed threshold of 0.5, no fitting at all.
// The map key is model, fixture, signal.
func ProbeDecisions(probes []Probe, mode string) map[[3]string]bool {
	out := map[[3]string]bool{}
	type ms struct{ model, signal string }
	groups := map[ms][]int{}
	for i, p := range probes {
		if _, ok := p.P(); ok {
			groups[ms{p.Model, p.Signal}] = append(groups[ms{p.Model, p.Signal}], i)
		}
	}
	for _, idx := range groups {
		for _, i := range idx {
			pi, _ := probes[i].P()
			t := 0.5
			if mode == "loo" {
				var train []scored
				for _, j := range idx {
					if j != i {
						pj, _ := probes[j].P()
						train = append(train, scored{pj, probes[j].Positive})
					}
				}
				t = bestThreshold(train)
			}
			out[[3]string{probes[i].Model, probes[i].Fixture, probes[i].Signal}] = pi >= t
		}
	}
	return out
}

// WriteProbeMarkdown writes the probe report.
func WriteProbeMarkdown(w io.Writer, probes []Probe, fixtures []Fixture, receptors []core.Genome, meta string) {
	var order, models []string
	for _, g := range receptors {
		order = append(order, g.MayDeposit[0])
	}
	for _, p := range probes {
		if !slices.Contains(models, p.Model) {
			models = append(models, p.Model)
		}
	}
	decisions := map[string]map[[3]string]bool{"loo": ProbeDecisions(probes, "loo"), "half": ProbeDecisions(probes, "half")}

	fmt.Fprintf(w, "# Level 0: probes (P(yes) from token probabilities)\n\n")
	if meta != "" {
		fmt.Fprintf(w, "%s\n\n", meta)
	}

	// 1. Per model: fixture verdicts under both decision rules.
	fmt.Fprintf(w, "| Model | Decision | Strict | Lenient | Model-only strict | Yes per fixture | Median AUC | Order bias | Calls | Usable | Tokens in/out per call | Median call latency |\n")
	fmt.Fprintf(w, "|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, m := range models {
		var aucs []float64
		for _, sig := range order {
			if a := probeAUC(probes, m, sig); !math.IsNaN(a) {
				aucs = append(aucs, a)
			}
		}
		sort.Float64s(aucs)
		medAUC := math.NaN()
		if len(aucs) > 0 {
			medAUC = aucs[len(aucs)/2]
			if len(aucs)%2 == 0 {
				medAUC = (aucs[len(aucs)/2-1] + aucs[len(aucs)/2]) / 2
			}
		}
		bias, _ := orderBias(probes, m, "")
		var calls, usable, in, out int
		var lat []time.Duration
		for _, p := range probes {
			if p.Model != m {
				continue
			}
			for _, c := range p.Calls {
				calls++
				if c.Err == "" {
					usable++
				}
				in += c.Usage.InputTokens
				out += c.Usage.OutputTokens
				lat = append(lat, c.Latency)
			}
		}
		cc := max(calls, 1)
		for _, mode := range []string{"loo", "half"} {
			n, strict, lenient, mo, moStrict, yes := fixtureVerdicts(probes, fixtures, m, decisions[mode])
			label := "fitted threshold (leave-one-out)"
			if mode == "half" {
				label = "fixed 0.5"
			}
			fmt.Fprintf(w, "| %s | %s | %s | %s | %d/%d | %.1f | %.2f | %+.2f | %d | %s | %d / %d | %s |\n",
				m, label, pctOf(strict, n), pctOf(lenient, n), moStrict, mo, float64(yes)/float64(max(n, 1)),
				medAUC, bias, calls, pctOf(usable, calls), in/cc, out/cc, median(lat).Round(time.Millisecond))
		}
	}
	fmt.Fprintf(w, "\n*AUC*: chance that a fixture needing the signal gets a higher P(yes) than one that does not; 0.5 is chance, needs no threshold. ")
	fmt.Fprintf(w, "*Order bias*: mean P(yes) with \"yes\" listed first minus with \"no\" first; 0 means the order of the options does not matter.\n")

	// 2. Per receptor: the entrance exam, with the fitted threshold.
	fmt.Fprintf(w, "\n## Entrance exam per receptor (fitted threshold)\n\n")
	fmt.Fprintf(w, "| Model | Signal | AUC | Yes rate | Label rate | Precision | Recall | Exam | Order bias | Mean P, positives | Mean P, others | Unusable |\n")
	fmt.Fprintf(w, "|---|---|---:|---:|---:|---:|---:|---|---:|---:|---:|---:|\n")
	for _, m := range models {
		for _, sig := range order {
			var n, yes, pos, tp, bad int
			var sumPos, sumNeg float64
			var nPos, nNeg int
			for _, p := range probes {
				if p.Model != m || p.Signal != sig {
					continue
				}
				pv, ok := p.P()
				if !ok {
					bad++
					continue
				}
				n++
				if p.Positive {
					pos++
					sumPos += pv
					nPos++
				} else {
					sumNeg += pv
					nNeg++
				}
				if decisions["loo"][[3]string{m, p.Fixture, sig}] {
					yes++
					if p.Positive {
						tp++
					}
				}
			}
			exam := "fail"
			if yes > 0 && float64(tp) >= 0.6*float64(yes) && yes <= 2*pos {
				exam = "**pass**"
			}
			bias, _ := orderBias(probes, m, sig)
			fmt.Fprintf(w, "| %s | %s | %.2f | %s | %s | %s | %s | %s | %+.2f | %.2f | %.2f | %d |\n", m, sig,
				probeAUC(probes, m, sig), pctOf(yes, n), pctOf(pos, n), pctOf(tp, yes), pctOf(tp, pos), exam, bias,
				sumPos/float64(max(nPos, 1)), sumNeg/float64(max(nNeg, 1)), bad)
		}
	}
	fmt.Fprintf(w, "\n*Exam*: precision ≥ 60%% and yes rate ≤ 2× label rate (rule fixed in advance).\n")

	// 3. Per fixture: P(yes) of every receptor, and the fitted decision.
	fmt.Fprintf(w, "\n## Per fixture\n\nP(yes) per receptor, in receptor order; **bold** where the fitted threshold says yes.\n\n")
	fmt.Fprintf(w, "| Fixture | Instrument | Accept | Model | Verdict | %s |\n|---|---|---|---|---|%s\n",
		strings.Join(order, " | "), strings.Repeat("---:|", len(order)))
	byKey := map[[3]string]Probe{}
	for _, p := range probes {
		byKey[[3]string{p.Model, p.Fixture, p.Signal}] = p
	}
	for _, f := range fixtures {
		inst := f.Instrument
		if inst == "" {
			inst = "**model only**"
		}
		for _, m := range models {
			said := saidSet(f, m, order, decisions["loo"])
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s %s |", f.ID, inst, strings.Join(f.Accept, " "), m, verdictMark(f, said), orNone(said))
			for _, sig := range order {
				p, ok := byKey[[3]string{m, f.ID, sig}]
				pv, usable := p.P()
				switch {
				case !ok || !usable:
					fmt.Fprintf(w, " – |")
				case decisions["loo"][[3]string{m, f.ID, sig}]:
					fmt.Fprintf(w, " **%.2f** |", pv)
				default:
					fmt.Fprintf(w, " %.2f |", pv)
				}
			}
			fmt.Fprintln(w)
		}
	}
	fmt.Fprintf(w, "\n✓ strict · ~ lenient only · ✗ wrong\n")
}

func probeAUC(probes []Probe, m, sig string) float64 {
	var pos, neg []float64
	for _, p := range probes {
		if p.Model != m || p.Signal != sig {
			continue
		}
		if pv, ok := p.P(); ok {
			if p.Positive {
				pos = append(pos, pv)
			} else {
				neg = append(neg, pv)
			}
		}
	}
	return AUC(pos, neg)
}

// orderBias is the mean of P(yes | yes first) − P(yes | no first) over
// probes where both calls worked. sig "" means every signal.
func orderBias(probes []Probe, m, sig string) (float64, int) {
	var s float64
	n := 0
	for _, p := range probes {
		if p.Model != m || (sig != "" && p.Signal != sig) || p.Calls[0].Err != "" || p.Calls[1].Err != "" {
			continue
		}
		s += p.Calls[1].P - p.Calls[0].P
		n++
	}
	if n == 0 {
		return 0, 0
	}
	return s / float64(n), n
}

func saidSet(f Fixture, m string, order []string, dec map[[3]string]bool) []string {
	var said []string
	for _, sig := range order {
		if dec[[3]string{m, f.ID, sig}] {
			said = append(said, sig)
		}
	}
	return said
}

func verdictOK(f Fixture, said []string) (strict, lenient bool) {
	if len(said) == 0 {
		ok := slices.Contains(f.Accept, None)
		return ok, ok
	}
	strict = true
	for _, s := range said {
		if slices.Contains(f.Accept, s) {
			lenient = true
		} else {
			strict = false
		}
	}
	return strict, lenient
}

func verdictMark(f Fixture, said []string) string {
	strict, lenient := verdictOK(f, said)
	switch {
	case strict:
		return "✓"
	case lenient:
		return "~"
	}
	return "✗"
}

func orNone(said []string) string {
	if len(said) == 0 {
		return "none"
	}
	return strings.Join(said, "")
}

func fixtureVerdicts(probes []Probe, fixtures []Fixture, m string, dec map[[3]string]bool) (n, strict, lenient, mo, moStrict, yes int) {
	var order []string
	for _, p := range probes {
		if p.Model == m && !slices.Contains(order, p.Signal) {
			order = append(order, p.Signal)
		}
	}
	for _, f := range fixtures {
		said := saidSet(f, m, order, dec)
		s, l := verdictOK(f, said)
		n++
		yes += len(said)
		if s {
			strict++
		}
		if l {
			lenient++
		}
		if f.Instrument == "" {
			mo++
			if s {
				moStrict++
			}
		}
	}
	return
}

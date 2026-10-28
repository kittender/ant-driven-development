package eval

import (
	"context"
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"kittender/add/internal/core"
	"kittender/add/internal/runner"
)

// Receptor runs: every receptor genome answers yes or no on every fixture.
// A fixture's verdict is the set of receptors that said yes.

// ReceptorPlan says what to run: fixtures × receptors × models.
type ReceptorPlan struct {
	Receptors []core.Genome // each with exactly one signal in MayDeposit
	Models    []string
	Seed      int64
	Fixtures  []Fixture
}

// RunReceptors runs the plan. Jobs are ordered model → fixture → receptor,
// so with parallel = 1 the receptors of one target run back to back and a
// local runtime can reuse their shared prefix.
func RunReceptors(ctx context.Context, r *runner.Runner, p ReceptorPlan, parallel int, onDone func(Result)) []Result {
	type job struct {
		idx     int
		fixture Fixture
		session runner.Session
	}
	var jobsList []job
	for _, m := range p.Models {
		for _, f := range p.Fixtures {
			for _, rg := range p.Receptors {
				g := rg
				g.Model, g.Seed = m, p.Seed
				g.ID = fmt.Sprintf("%s@%s", rg.ID, m)
				jobsList = append(jobsList, job{
					idx:     len(jobsList),
					fixture: f,
					session: runner.Session{
						ID:     fmt.Sprintf("l0r-%s-%s-%s", f.ID, strings.TrimPrefix(rg.ID, "receptor-"), slug(m)),
						Genome: g,
						Bundle: f.Bundle(),
					},
				})
			}
		}
	}
	results := make([]Result, len(jobsList))
	jobs := make(chan job)
	var wg sync.WaitGroup
	for w := 0; w < max(parallel, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				rec := r.Run(ctx, j.session)
				res := Result{Fixture: j.fixture.ID, Signal: j.session.Genome.MayDeposit[0],
					Score: scoreReceptor(j.fixture, j.session.Genome, rec), Record: rec}
				results[j.idx] = res
				if onDone != nil {
					onDone(res)
				}
			}
		}()
	}
	for _, j := range jobsList {
		select {
		case jobs <- j:
		case <-ctx.Done():
		}
	}
	close(jobs)
	wg.Wait()
	return results
}

// scoreReceptor: one receptor's answer is right when it says yes exactly on
// the fixtures that accept its signal. Valid means it answered cleanly.
func scoreReceptor(f Fixture, g core.Genome, rec runner.Record) Score {
	if rec.Error != "" {
		return Score{Errored: true}
	}
	finished := rec.Outcome == core.OutcomeDone
	yes := len(rec.Deposits) > 0
	return Score{
		Valid:   finished && rec.Malformed == 0 && rec.Refusals == 0,
		Fitting: yes == slices.Contains(f.Accept, g.MayDeposit[0]),
	}
}

// FixtureVerdict aggregates the receptors of one model on one fixture.
type FixtureVerdict struct {
	Fixture Fixture
	Model   string
	Yes     []string // emoji of the receptors that said yes, in dictionary order
	Strict  bool     // yes-set non-empty and all in Accept, or empty and Accept has none
	Lenient bool     // at least one yes in Accept, or empty and Accept has none
	Errored bool
}

// Verdicts groups receptor results per model and fixture.
func Verdicts(results []Result, fixtures []Fixture, order []string) []FixtureVerdict {
	byID := map[string]Fixture{}
	for _, f := range fixtures {
		byID[f.ID] = f
	}
	type key struct{ model, fixture string }
	yes := map[key][]string{}
	errored := map[key]bool{}
	var keys []key
	seen := map[key]bool{}
	for _, r := range results {
		k := key{r.Record.Model, r.Fixture}
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
		if r.Score.Errored {
			errored[k] = true
		}
		for _, d := range r.Record.Deposits {
			yes[k] = append(yes[k], d.Emoji)
		}
	}
	rank := map[string]int{}
	for i, e := range order {
		rank[e] = i
	}
	var out []FixtureVerdict
	for _, k := range keys {
		f := byID[k.fixture]
		ys := yes[k]
		sort.SliceStable(ys, func(i, j int) bool { return rank[ys[i]] < rank[ys[j]] })
		v := FixtureVerdict{Fixture: f, Model: k.model, Yes: ys, Errored: errored[k]}
		if len(ys) == 0 {
			v.Strict = slices.Contains(f.Accept, None)
			v.Lenient = v.Strict
		} else {
			v.Strict, v.Lenient = true, false
			for _, y := range ys {
				if slices.Contains(f.Accept, y) {
					v.Lenient = true
				} else {
					v.Strict = false
				}
			}
		}
		out = append(out, v)
	}
	return out
}

// ReceptorStats is one receptor's record on one model: the entrance exam.
type ReceptorStats struct {
	Signal    string
	Model     string
	Answers   int // clean answers
	Yes       int
	Positives int // fixtures whose Accept contains the signal
	TruePos   int
	Malformed int
}

func pctOf(n, d int) string {
	if d == 0 {
		return "–"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(d))
}

// WriteReceptorMarkdown writes the receptor report.
func WriteReceptorMarkdown(w io.Writer, results []Result, fixtures []Fixture, receptors []core.Genome, meta string) {
	var order []string // signals in receptor order
	for _, g := range receptors {
		order = append(order, g.MayDeposit[0])
	}
	byFixture := map[string]Fixture{}
	for _, f := range fixtures {
		byFixture[f.ID] = f
	}

	fmt.Fprintf(w, "# Level 0: receptors\n\n")
	if meta != "" {
		fmt.Fprintf(w, "%s\n\n", meta)
	}

	verdicts := Verdicts(results, fixtures, order)
	var models []string
	for _, v := range verdicts {
		if !slices.Contains(models, v.Model) {
			models = append(models, v.Model)
		}
	}

	// 1. Per model: fixture-level verdicts, the number to compare with the scout.
	fmt.Fprintf(w, "| Model | Fixtures | Strict | Lenient | Model-only strict | Yes per fixture | Calls | Valid calls | Tokens in/out per call | Median call latency |\n")
	fmt.Fprintf(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, m := range models {
		var n, strict, lenient, mo, moStrict, yes int
		for _, v := range verdicts {
			if v.Model != m || v.Errored {
				continue
			}
			n++
			yes += len(v.Yes)
			if v.Strict {
				strict++
			}
			if v.Lenient {
				lenient++
			}
			if v.Fixture.Instrument == "" {
				mo++
				if v.Strict {
					moStrict++
				}
			}
		}
		var calls, valid, in, out int
		var lat []time.Duration
		for _, r := range results {
			if r.Record.Model != m || r.Score.Errored {
				continue
			}
			calls++
			if r.Score.Valid {
				valid++
			}
			in += r.Record.Usage.InputTokens
			out += r.Record.Usage.OutputTokens
			for _, s := range r.Record.Steps {
				lat = append(lat, s.Latency)
			}
		}
		c := max(calls, 1)
		fmt.Fprintf(w, "| %s | %d | %s | %s | %d/%d | %.1f | %d | %s | %d / %d | %s |\n",
			m, n, pctOf(strict, n), pctOf(lenient, n), moStrict, mo, float64(yes)/float64(max(n, 1)),
			calls, pctOf(valid, calls), in/c, out/c, median(lat).Round(time.Millisecond))
	}
	fmt.Fprintf(w, "\n*Strict*: the receptors that said yes are all right (or none said yes and nothing was needed). ")
	fmt.Fprintf(w, "*Lenient*: at least one right yes. *Model-only*: fixtures no instrument could detect.\n")

	// 2. Per receptor: the entrance exam. Yes rate against label rate shows default bias.
	fmt.Fprintf(w, "\n## Entrance exam per receptor\n\n")
	fmt.Fprintf(w, "| Model | Signal | Yes rate | Label rate | Precision | Recall | Malformed |\n|---|---|---:|---:|---:|---:|---:|\n")
	for _, m := range models {
		for _, sig := range order {
			st := ReceptorStats{Signal: sig, Model: m}
			for _, r := range results {
				if r.Record.Model != m || r.Signal != sig || r.Score.Errored {
					continue
				}
				pos := slices.Contains(byFixture[r.Fixture].Accept, sig)
				if pos {
					st.Positives++
				}
				if r.Record.Malformed > 0 {
					st.Malformed++
				}
				if r.Record.Outcome == core.OutcomeDone {
					st.Answers++
				}
				if len(r.Record.Deposits) > 0 {
					st.Yes++
					if pos {
						st.TruePos++
					}
				}
			}
			total := st.Answers + st.Malformed
			fmt.Fprintf(w, "| %s | %s | %s | %s | %s | %s | %d |\n", m, sig,
				pctOf(st.Yes, total), pctOf(st.Positives, total), pctOf(st.TruePos, st.Yes), pctOf(st.TruePos, st.Positives), st.Malformed)
		}
	}
	fmt.Fprintf(w, "\nA yes rate far above the label rate is a default bias: that receptor's yes carries little information.\n")

	// 3. Per fixture: what each model's receptors said.
	fmt.Fprintf(w, "\n## Per fixture\n\n| Fixture | Instrument | Accept |")
	for _, m := range models {
		fmt.Fprintf(w, " %s |", m)
	}
	fmt.Fprintf(w, "\n|---|---|---|")
	for range models {
		fmt.Fprintf(w, "---|")
	}
	fmt.Fprintln(w)
	cell := map[string]map[string]FixtureVerdict{}
	for _, v := range verdicts {
		if cell[v.Fixture.ID] == nil {
			cell[v.Fixture.ID] = map[string]FixtureVerdict{}
		}
		cell[v.Fixture.ID][v.Model] = v
	}
	for _, f := range fixtures {
		inst := f.Instrument
		if inst == "" {
			inst = "**model only**"
		}
		fmt.Fprintf(w, "| %s | %s | %s |", f.ID, inst, strings.Join(f.Accept, " "))
		for _, m := range models {
			v, ok := cell[f.ID][m]
			switch {
			case !ok:
				fmt.Fprintf(w, " – |")
			default:
				got := strings.Join(v.Yes, "")
				if got == "" {
					got = "none"
				}
				mark := "✗"
				if v.Strict {
					mark = "✓"
				} else if v.Lenient {
					mark = "~"
				}
				fmt.Fprintf(w, " %s %s |", mark, got)
			}
		}
		fmt.Fprintln(w)
	}
	fmt.Fprintf(w, "\n✓ strict · ~ lenient only · ✗ wrong\n")
}

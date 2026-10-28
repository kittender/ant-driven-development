package eval

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"time"
)

// ModelSummary aggregates one model's sessions.
type ModelSummary struct {
	Model       string
	Sessions    int // excluding errored
	Errored     int
	Valid       int
	Fitting     int
	Malformed   int // sessions with at least one malformed reply
	Refusals    int // total refused calls
	Normalized  int // deposits that needed U+FE0F stripped
	Steps       int
	InTokens    int
	OutTokens   int
	StepLatency []time.Duration
}

func pct(n, d int) string {
	if d == 0 {
		return "–"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(d))
}

func median(ds []time.Duration) time.Duration {
	if len(ds) == 0 {
		return 0
	}
	s := slices.Clone(ds)
	slices.Sort(s)
	return s[len(s)/2]
}

// Summarize groups results by model, in first-seen order.
func Summarize(results []Result) []*ModelSummary {
	var order []string
	by := map[string]*ModelSummary{}
	for _, r := range results {
		m := r.Record.Model
		s, ok := by[m]
		if !ok {
			s = &ModelSummary{Model: m}
			by[m] = s
			order = append(order, m)
		}
		if r.Score.Errored {
			s.Errored++
			continue
		}
		s.Sessions++
		if r.Score.Valid {
			s.Valid++
		}
		if r.Score.Fitting {
			s.Fitting++
		}
		if r.Record.Malformed > 0 {
			s.Malformed++
		}
		s.Refusals += r.Record.Refusals
		for _, d := range r.Record.Deposits {
			if d.Normalized {
				s.Normalized++
			}
		}
		s.Steps += len(r.Record.Steps)
		s.InTokens += r.Record.Usage.InputTokens
		s.OutTokens += r.Record.Usage.OutputTokens
		for _, st := range r.Record.Steps {
			s.StepLatency = append(s.StepLatency, st.Latency)
		}
	}
	out := make([]*ModelSummary, len(order))
	for i, m := range order {
		out[i] = by[m]
	}
	return out
}

// WriteMarkdown writes the level 0 report.
// meta is one line describing the run (genome, menu, flags), printed under the title.
func WriteMarkdown(w io.Writer, results []Result, fixtures []Fixture, meta string) {
	sums := Summarize(results)

	fmt.Fprintf(w, "# Level 0: signal choice\n\n")
	if meta != "" {
		fmt.Fprintf(w, "%s\n\n", meta)
	}
	fmt.Fprintf(w, "| Model | Sessions | Valid | Fitting | Malformed | Refusals | FE0F fixed | Steps/session | Tokens in/out per session | Median step latency | Errored |\n")
	fmt.Fprintf(w, "|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|\n")
	for _, s := range sums {
		n := max(s.Sessions, 1)
		fmt.Fprintf(w, "| %s | %d | %s | %s | %s | %d | %d | %.1f | %d / %d | %s | %d |\n",
			s.Model, s.Sessions, pct(s.Valid, s.Sessions), pct(s.Fitting, s.Sessions),
			pct(s.Malformed, s.Sessions), s.Refusals, s.Normalized,
			float64(s.Steps)/float64(n), s.InTokens/n, s.OutTokens/n,
			median(s.StepLatency).Round(time.Millisecond), s.Errored)
	}

	// Model-only fixtures: the cases no instrument could detect.
	fmt.Fprintf(w, "\nModel-only fixtures (no instrument could detect them), fitting:")
	for _, sm := range sums {
		fit, n := 0, 0
		for _, r := range results {
			if r.Record.Model != sm.Model || r.Score.Errored {
				continue
			}
			for _, f := range fixtures {
				if f.ID == r.Fixture && f.Instrument == "" {
					n++
					if r.Score.Fitting {
						fit++
					}
				}
			}
		}
		fmt.Fprintf(w, " %s %d/%d ·", sm.Model, fit, n)
	}
	fmt.Fprintln(w)

	// Fixture × model matrix: fitting sessions / sessions.
	fmt.Fprintf(w, "\n## Fitting by fixture\n\n| Fixture | Accept |")
	for _, s := range sums {
		fmt.Fprintf(w, " %s |", s.Model)
	}
	fmt.Fprintf(w, "\n|---|---|")
	for range sums {
		fmt.Fprintf(w, "---:|")
	}
	fmt.Fprintln(w)

	type cell struct{ fit, n int }
	grid := map[string]map[string]*cell{}
	for _, r := range results {
		if r.Score.Errored {
			continue
		}
		if grid[r.Fixture] == nil {
			grid[r.Fixture] = map[string]*cell{}
		}
		c := grid[r.Fixture][r.Record.Model]
		if c == nil {
			c = &cell{}
			grid[r.Fixture][r.Record.Model] = c
		}
		c.n++
		if r.Score.Fitting {
			c.fit++
		}
	}
	for _, f := range fixtures {
		fmt.Fprintf(w, "| %s | %v |", f.ID, f.Accept)
		for _, s := range sums {
			if c := grid[f.ID][s.Model]; c != nil {
				fmt.Fprintf(w, " %d/%d |", c.fit, c.n)
			} else {
				fmt.Fprintf(w, " – |")
			}
		}
		fmt.Fprintln(w)
	}

	// What models deposited instead, to tune fixtures and genome wording.
	fmt.Fprintf(w, "\n## Misses\n\n")
	type miss struct{ fixture, model, got string }
	var misses []miss
	for _, r := range results {
		if r.Score.Errored || r.Score.Fitting {
			continue
		}
		got := ""
		for _, d := range r.Record.Deposits {
			got += d.Emoji
		}
		if got == "" {
			got = "(none, " + string(r.Record.Outcome) + ")"
		}
		misses = append(misses, miss{r.Fixture, r.Record.Model, got})
	}
	sort.SliceStable(misses, func(i, j int) bool { return misses[i].fixture < misses[j].fixture })
	if len(misses) == 0 {
		fmt.Fprintf(w, "None.\n")
	}
	for _, m := range misses {
		fmt.Fprintf(w, "- %s, %s: %s\n", m.fixture, m.model, m.got)
	}
}

package pilot

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kittender/add/internal/honey"
	"kittender/add/internal/instruments"
)

// Verdict applies the decision table of outline Part B to the green count.
func Verdict(green int) string {
	switch {
	case green >= 18:
		return "Supported: small local models can close the loop on a well-specified task; Level 1 is built for real"
	case green <= 8:
		return "Not supported on this model with these bundles: smaller tasks or a larger worker model before Level 1"
	default:
		return "Open: report which features fail and why (tester, worker, or the bundle)"
	}
}

// WriteReport writes report.md and the two labelling sheets of Part A.
func (p *Pilot) WriteReport() error {
	if p.cfg.Out == "" {
		return nil
	}
	var b strings.Builder
	w := func(format string, a ...any) { fmt.Fprintf(&b, format+"\n", a...) }

	green, total := 0, 0
	for _, r := range p.runs {
		total++
		if r.CountedOK {
			green++
		}
	}
	var tokens, calls int
	var modelS, proverS float64
	rejectedWrites, regressions := 0, 0
	for _, a := range p.attempts {
		calls++
		tokens += a.Usage.InputTokens + a.Usage.OutputTokens
		modelS += a.Latency
		proverS += a.ProverS
		if a.Action == Write && !a.Accepted {
			rejectedWrites++
			if strings.Contains(a.Reason, "broke other scenarios") {
				regressions++
			}
		}
	}

	w("# Pilot loop: %s", filepath.Base(p.cfg.Workspace))
	w("")
	w("Model `%s` · %d scenarios run · %s wall time · rules: docs/concept-0.4-outline.adoc, \"Level 0c and the first loop\"", p.cfg.Model, total, time.Since(p.start).Round(time.Second))
	w("")
	w("## Part B verdict")
	w("")
	w("**Green without help: %d / %d** (capped scenarios passing at the end, not counting any whose own honey was weakened).", green, total)
	w("")
	if p.cfg.Only != "" || p.cfg.Limit > 0 {
		w("Trial run (filtered with -only or -limit): the decision table of the outline applies only to a full run, so no verdict.")
	} else {
		w("%s.", Verdict(green))
	}
	w("")
	own := 0
	for _, e := range p.events {
		if e.Kept && e.Weakens() && e.OwnCell && e.Caste == Worker {
			own++
		}
	}
	if own > 0 {
		w("**Test ownership:** %d kept weakening(s) of a scenario by the worker implementing it. Rule: test ownership (law 10) moves into the harness as a MUST before the loop runs again.", own)
		w("")
	}
	w("| Measure | Value |")
	w("|---|---:|")
	w("| Writer sessions (model calls) | %d |", calls)
	w("| Tokens (in + out) | %d |", tokens)
	w("| Model time | %s |", time.Duration(modelS*float64(time.Second)).Round(time.Second))
	w("| Prover time | %s |", time.Duration(proverS*float64(time.Second)).Round(time.Second))
	if green > 0 {
		w("| Sessions per green scenario | %.1f |", float64(calls)/float64(green))
	}
	reflexes, reflexAccepted := 0, 0
	for _, a := range p.attempts {
		if len(a.Reflexes) > 0 {
			reflexes++
			if a.Accepted {
				reflexAccepted++
			}
		}
	}
	w("| Writes rejected by the Prover | %d |", rejectedWrites)
	w("| Writes a reflex edited (auto-import, unwrap, unused parameter) | %d, of which accepted %d |", reflexes, reflexAccepted)
	lost, lostIface, lostCode, workers, workerNoCompile := 0, 0, 0, 0, 0
	hints := map[string]int{}
	for _, a := range p.attempts {
		if a.Caste == Worker {
			workers++
			if strings.HasPrefix(a.Reason, "it does not compile") {
				workerNoCompile++
			}
		}
		for _, h := range a.Hints {
			hints[h]++
		}
		if a.Accepted {
			continue
		}
		switch a.Lost {
		case "interface":
			lost++
			lostIface++
		case "code":
			lost++
			lostCode++
		}
	}
	pct := func(n, d int) string {
		if d == 0 {
			return "n/a"
		}
		return fmt.Sprintf("%.0f%%", 100*float64(n)/float64(d))
	}
	w("| Sessions lost before any test ran | %d (%s) |", lost, pct(lost, calls))
	w("| … interface losses (invalid reply, or compile errors outside src/) | %d (%s) |", lostIface, pct(lostIface, calls))
	w("| … code losses (compile errors in src/) | %d (%s) |", lostCode, pct(lostCode, calls))
	w("| Worker sessions that do not compile | %d of %d (%s) |", workerNoCompile, workers, pct(workerNoCompile, workers))
	var hl []string
	for _, h := range compilerHints {
		if hints[h.ID] > 0 {
			hl = append(hl, fmt.Sprintf("%s %d", h.ID, hints[h.ID]))
		}
	}
	if len(hl) == 0 {
		hl = []string{"none"}
	}
	w("| Compiler hints shown (reflex 5) | %s |", strings.Join(hl, ", "))
	w("| … of which broke another scenario | %d |", regressions)
	w("")

	withReflex := 0
	for _, r := range p.runs {
		if !r.CountedOK {
			continue
		}
		for _, a := range p.attempts {
			if a.Scenario == r.Key && a.Accepted && len(a.Reflexes) > 0 {
				withReflex++
				break
			}
		}
	}
	w("Green scenarios with a reflex edit in an accepted session: %d of %d.", withReflex, green)
	w("")
	w("## Scenarios")
	w("")
	w("| Scenario | Start | End | Counted | Sessions | Dead ends | Seconds |")
	w("|---|---|---|---|---:|---|---:|")
	for _, r := range p.runs {
		c := "no"
		switch {
		case r.CountedOK:
			c = "yes"
		case r.Weakened:
			c = "no, honey weakened"
		case r.Vanished:
			c = "no, scenario gone"
		}
		w("| %s | %s | %s | %s | %d | %s | %.0f |", r.Key, r.Start, r.End, c, r.Attempts, strings.Join(r.DeadEnds, ", "), r.Seconds)
	}
	w("")

	w("## Honey events")
	w("")
	if len(p.events) == 0 {
		w("None: no session changed or proposed to change the honey.")
	} else {
		w("| Session | Genome | Target | Event on | Kind | Own cell | Path | Kept | Detail |")
		w("|---:|---|---|---|---|---|---|---|---|")
		for _, e := range p.events {
			on := e.File
			if e.Scenario != "" {
				on += " > " + e.Scenario
			}
			w("| %d | %s | %s | %s | %s | %v | %s | %v | %s |", e.Session, e.Genome, e.Target, on, e.Kind, e.OwnCell, e.Path, e.Kept, strings.ReplaceAll(e.Detail, "|", "\\|"))
		}
	}
	w("")
	if drift := p.drift(); drift != "" {
		w("## Drift from the seed (`%s`)", p.cfg.SeedSpec)
		w("")
		w("%s", drift)
		w("")
	}

	w("## Plot (proposed): garden/game/domain.md")
	w("")
	maxWarn := 0
	for _, s := range p.surveys {
		n := 0
		for _, f := range s.Lint {
			if f.Signal == "🧩" {
				n++
			}
		}
		maxWarn = max(maxWarn, n)
	}
	w("Most warnings from the plot's proof in any survey: %d (observation, not a rule).", maxWarn)
	w("")

	w("## Part A: instruments")
	w("")
	problems := 0
	for _, s := range p.surveys {
		if len(s.Problems) > 0 {
			problems++
		}
	}
	w("Surveys: %d · with problems (parse validity must be 100%%): %d.", len(p.surveys), problems)
	w("")
	w("Precision, fact accuracy and recall need human labels: see `instrument-findings.md` and `facts-check.md` in this folder.")
	w("")

	if err := os.WriteFile(filepath.Join(p.cfg.Out, "report.md"), []byte(b.String()), 0o644); err != nil {
		return err
	}
	if err := p.findingsSheet(); err != nil {
		return err
	}
	return p.factsSheet()
}

func (p *Pilot) drift() string {
	if p.cfg.SeedSpec == "" {
		return ""
	}
	seed, err := honey.Load(p.cfg.SeedSpec)
	if err != nil {
		return "cannot read the seed: " + err.Error()
	}
	now, err := p.honey()
	if err != nil {
		return "cannot read the workspace honey: " + err.Error()
	}
	ev := honey.Diff(seed, now)
	if len(ev) == 0 {
		return "None: the workspace's honey equals the seed."
	}
	var l []string
	for _, e := range ev {
		l = append(l, fmt.Sprintf("- %s %s > %s: %s", e.Kind, e.File, e.Scenario, e.Detail))
	}
	return strings.Join(l, "\n")
}

// findingsSheet lists every distinct instrument finding of the run, for the
// human to label fitting or not (Part A, signal precision).
func (p *Pilot) findingsSheet() error {
	type key struct{ ins, file, rule, msg string }
	seen := map[key]int{}
	var order []key
	var first []instruments.Finding
	for _, s := range p.surveys {
		for _, f := range s.Findings() {
			k := key{f.Instrument, f.File, f.Rule, f.Message}
			if _, ok := seen[k]; !ok {
				order = append(order, k)
				first = append(first, f)
			}
			seen[k]++
		}
	}
	var b strings.Builder
	b.WriteString("# Instrument findings: label each one\n\n")
	b.WriteString("Question for each row (as for the fixtures): would a careful developer, seeing this code, deposit this signal here? Write yes or no in the last column. Precision = yes / all (outline Part A: adopt at 80% or more).\n\n")
	b.WriteString("| # | Instrument | Where | Rule | Message | Signal | Surveys | Fitting? |\n|---:|---|---|---|---|---|---:|---|\n")
	for i, k := range order {
		f := first[i]
		fmt.Fprintf(&b, "| %d | %s | %s:%d | %s | %s | %s | %d | |\n", i+1, f.Instrument, f.File, f.Line, f.Rule, strings.ReplaceAll(f.Message, "|", "\\|"), f.Signal, seen[k])
	}
	if len(order) == 0 {
		b.WriteString("\nNo instrument raised a signal during this run.\n")
	}
	return os.WriteFile(filepath.Join(p.cfg.Out, "instrument-findings.md"), []byte(b.String()), 0o644)
}

// factsSheet samples 20 facts from the first, middle and last surveys for
// checking against the repository (Part A, fact accuracy must be 100%).
func (p *Pilot) factsSheet() error {
	var b strings.Builder
	b.WriteString("# Facts to check against the repository\n\n")
	b.WriteString("20 facts sampled (seed 1) from the first, middle and last surveys. For each: is it true of the workspace at that moment? Any false fact is an instrument bug (outline Part A).\n")
	if len(p.surveys) == 0 {
		return os.WriteFile(filepath.Join(p.cfg.Out, "facts-check.md"), []byte(b.String()), 0o644)
	}
	idx := []int{0, len(p.surveys) / 2, len(p.surveys) - 1}
	done := map[int]bool{}
	rng := rand.New(rand.NewSource(1))
	for _, i := range idx {
		if done[i] {
			continue
		}
		done[i] = true
		s := p.surveys[i]
		facts := s.AllFacts()
		rng.Shuffle(len(facts), func(a, c int) { facts[a], facts[c] = facts[c], facts[a] })
		if len(facts) > 20 {
			facts = facts[:20]
		}
		sort.SliceStable(facts, func(a, c int) bool { return facts[a].Kind < facts[c].Kind })
		fmt.Fprintf(&b, "\n## Survey %d of %d, %s\n\n| # | Fact | True? |\n|---:|---|---|\n", i+1, len(p.surveys), s.At.Format("15:04:05"))
		for j, f := range facts {
			fmt.Fprintf(&b, "| %d | %s | |\n", j+1, strings.ReplaceAll(f.Render(), "|", "\\|"))
		}
	}
	return os.WriteFile(filepath.Join(p.cfg.Out, "facts-check.md"), []byte(b.String()), 0o644)
}

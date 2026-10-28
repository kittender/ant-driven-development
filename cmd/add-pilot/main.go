// Command add-pilot runs the first ADD loop on a real workspace: a tester
// and a worker per capped scenario, a script standing in for the parts of
// the Ecosystem not built yet. Rules: docs/concept-0.4-outline.adoc,
// section "Level 0c and the first loop: tic-tac-toe".
//
//	go run ./cmd/add-pilot -check                 # instruments only, no model: check the mapping first
//	go run ./cmd/add-pilot                        # the pilot, qwen3:8b
//	go run ./cmd/add-pilot -only 01_start -limit 1 # one scenario, to try it out
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kittender/add/internal/honey"
	"kittender/add/internal/instruments"
	"kittender/add/internal/model"
	"kittender/add/internal/pilot"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "add-pilot:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("add-pilot", flag.ContinueOnError)
	ws := fs.String("workspace", "benchmarks/tictactoe/typescript", "the project the colony works on")
	seed := fs.String("seed", "benchmarks/tictactoe/spec", "the untouched seed honey, to measure drift")
	modelSpec := fs.String("model", "ollama:qwen3:8b", "writer model, provider:model")
	ollamaURL := fs.String("ollama", "http://localhost:11434", "Ollama base URL")
	results := fs.String("out", "evals/pilot/results", "results root; each run gets a timestamped folder")
	maxTokens := fs.Int("max-tokens", 6000, "output limit per reply (a whole file)")
	ctxTokens := fs.Int("ctx", 16384, "model context window (Ollama num_ctx)")
	attempts := fs.Int("attempts", 2, "attempts by the first genome before a dead end; the second genome gets one")
	only := fs.String("only", "", "run only scenarios whose key contains this text")
	limit := fs.Int("limit", 0, "stop after this many scenarios (0 = all)")
	stepsAPI := fs.String("steps-api", "node_modules/quickpickle/README.md", "file in the workspace that documents the step library, given to testers")
	check := fs.Bool("check", false, "run the instruments once, print how test results map onto the honey, and stop")
	if err := fs.Parse(args); err != nil {
		return err
	}
	wsAbs, err := filepath.Abs(*ws)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(wsAbs, "spec")); err != nil {
		return fmt.Errorf("no spec folder in %s: %w", wsAbs, err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	stamp := time.Now().Format("20060102-150405")
	dir := filepath.Join(*results, stamp)
	if *check {
		dir = filepath.Join(*results, stamp+"-check")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	cfg := pilot.Config{
		Workspace: wsAbs, SeedSpec: *seed, Model: *modelSpec,
		Gateway:  model.NewGateway(&model.Ollama{BaseURL: *ollamaURL}),
		Run:      instruments.Exec,
		Commands: instruments.TypeScript(),
		Out:      dir, MaxTokens: *maxTokens, ContextTokens: *ctxTokens, Attempts: *attempts,
		StepsAPI: *stepsAPI, StepsAPIExtra: []string{"node_modules/quickpickle/dist/models/DataTable.d.ts"},
		Only: *only, Limit: *limit, Progress: out,
	}
	if *check {
		return checkRun(ctx, cfg, out)
	}
	fmt.Fprintf(out, "add-pilot: %s on %s, results in %s\n", *modelSpec, wsAbs, dir)
	fmt.Fprintln(out, "The workspace is changed in place: commit or copy it first if you want to start again from here.")
	err = pilot.New(cfg).Run(ctx)
	fmt.Fprintf(out, "report: %s\n", filepath.Join(dir, "report.md"))
	return err
}

// checkRun is the dry run: one survey, no model. It shows whether every
// capped scenario is matched by test results, the states, and the raw
// failure of each, so the mapping can be confirmed before the pilot spends
// model time (outline Part A: parse validity).
func checkRun(ctx context.Context, cfg pilot.Config, out io.Writer) error {
	snap, err := honey.Load(filepath.Join(cfg.Workspace, "spec"))
	if err != nil {
		return err
	}
	if err := (pilot.Workspace{Dir: cfg.Workspace}).SyncStepsIndex(); err != nil {
		return err
	}
	s := instruments.Run(ctx, cfg.Run, cfg.Workspace, cfg.Commands, snap)
	b, _ := json.MarshalIndent(s, "", "  ")
	if err := os.WriteFile(filepath.Join(cfg.Out, "survey.json"), b, 0o644); err != nil {
		return err
	}
	for _, o := range s.Outputs {
		fmt.Fprintf(out, "ran %-60s exit %d in %s\n", strings.Join(o.Command, " "), o.Exit, o.Duration.Round(time.Millisecond))
	}
	fmt.Fprintf(out, "\ncapped scenarios: %d\n", len(snap.Capped()))
	matched := map[string]bool{}
	for _, sc := range snap.Capped() {
		r := s.Scenarios[sc.Key()]
		fmt.Fprintf(out, "  %-9s %2d tests  %s\n", r.State, r.Tests, sc.Key())
		for _, t := range r.Titles {
			matched[t] = true
		}
		if r.Failure != "" {
			fmt.Fprintf(out, "            first failure: %s\n", oneLine(r.Failure, 140))
		}
	}
	var unmatched []string
	for _, c := range s.Cases {
		if !matched[c.Title] {
			unmatched = append(unmatched, fmt.Sprintf("%s | %s | %s", c.File, c.Title, c.Status))
		}
	}
	sort.Strings(unmatched)
	if len(unmatched) > 0 {
		fmt.Fprintf(out, "\ntest cases that matched no scenario (%d):\n", len(unmatched))
		for _, u := range unmatched {
			fmt.Fprintln(out, "  "+u)
		}
	}
	for _, e := range s.FileErrs {
		fmt.Fprintf(out, "\nfile failed to load: %s: %s\n", e.File, oneLine(e.Failure, 300))
	}
	fmt.Fprintf(out, "\ncompiler errors %d · lint findings %d (errors %d) · functions covered %d · duplicates %d\n",
		len(s.Compiler), len(s.Lint), instruments.LintErrors(s.Lint), len(s.Coverage), len(s.Dupes)/2)
	if len(s.Problems) > 0 {
		fmt.Fprintln(out, "\nPROBLEMS (harness bugs to fix before the pilot):")
		for _, p := range s.Problems {
			fmt.Fprintln(out, "  - "+oneLine(p, 300))
		}
	}
	fmt.Fprintf(out, "\nfull survey: %s\n", filepath.Join(cfg.Out, "survey.json"))
	return nil
}

func oneLine(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > n {
		s = s[:n] + " …"
	}
	return s
}

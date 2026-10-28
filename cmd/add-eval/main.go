// Command add-eval runs the level 0 experiment.
//
//	go run ./cmd/add-eval -models ollama:qwen3:4b,ollama:llama3.2:3b -seeds 3
//
// It writes sessions.jsonl (the colony log for each session) and report.md
// into a timestamped folder under -out, and prints the report.
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
	"strings"
	"sync"
	"time"

	"kittender/add/internal/core"
	"kittender/add/internal/eval"
	"kittender/add/internal/model"
	"kittender/add/internal/runner"
)

func main() {
	// Go has no exceptions: main calls run and turns its error into an exit code.
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "add-eval:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("add-eval", flag.ContinueOnError)
	var (
		fixturesPath = fs.String("fixtures", "evals/level0/fixtures.json", "fixtures file")
		dictPath     = fs.String("dict", "ecosystem-home.example/dictionary.json", "signal dictionary")
		genomePath   = fs.String("genome", "ecosystem-home.example/genomes/scout-l0-b.json", "genome under test")
		models       = fs.String("models", "fake:finish", "comma-separated provider:model list")
		seeds        = fs.Int("seeds", 3, "number of seeds per model and fixture (1..n)")
		parallel     = fs.Int("parallel", 1, "sessions at once (keep 1 for a single local GPU)")
		only         = fs.String("only", "", "comma-separated fixture ids to run")
		outDir       = fs.String("out", "evals/level0/results", "results folder")
		ollamaURL    = fs.String("ollama", envOr("OLLAMA_HOST", "http://localhost:11434"), "Ollama base URL")
		schema       = fs.Bool("schema", true, "constrain Ollama output with the action JSON Schema")
		menu         = fs.String("menu", "emoji", "how signals are shown to models: emoji, alias (names like needs_test) or both (🧪 needs_test)")
		think        = fs.Bool("think", false, "let Ollama thinking models (qwen3...) reason first; raise output_tokens if on")
		receptors    = fs.String("receptors", "", "run every receptor genome in this folder on every fixture instead of the scout genome (e.g. ecosystem-home.example/receptors)")
		calibration  = fs.String("calibration", "", "with -probe: fixtures used only to fit thresholds (e.g. evals/level0/fixtures.json); -fixtures are then decided once, held out")
		zone         = fs.Bool("zone", false, "with -probe: feed each probe its fixture's zone (neighbouring files), for the zones experiment")
		probe        = fs.String("probe", "", "with -receptors: read P(yes) from token probabilities instead of the written answer; note (a note, then the answer) or direct (the answer alone)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *probe != "" && *probe != "note" && *probe != "direct" {
		return fmt.Errorf("-probe must be note or direct, got %q", *probe)
	}
	if *zone && *probe == "" {
		return fmt.Errorf("-zone needs -probe (scouts roam free of zones)")
	}
	if *calibration != "" && *probe == "" {
		return fmt.Errorf("-calibration needs -probe")
	}
	if *probe != "" && *receptors == "" {
		return fmt.Errorf("-probe needs -receptors (the yes/no questions, e.g. ecosystem-home.example/receptors)")
	}
	style := runner.MenuStyle(*menu)
	if style != runner.MenuEmoji && style != runner.MenuAlias && style != runner.MenuBoth {
		return fmt.Errorf("-menu must be emoji, alias or both, got %q", *menu)
	}

	dict, err := loadWith(*dictPath, core.LoadDictionary)
	if err != nil {
		return err
	}
	genome, err := loadWith(*genomePath, core.LoadGenome)
	if err != nil {
		return err
	}
	if err := genome.Validate(dict); err != nil {
		return err
	}
	var receptorList []core.Genome
	if *receptors != "" {
		if receptorList, err = loadReceptors(*receptors, dict); err != nil {
			return err
		}
	}
	fixtures, err := loadWith(*fixturesPath, func(r io.Reader) ([]eval.Fixture, error) {
		return eval.LoadFixtures(r, dict)
	})
	if err != nil {
		return err
	}
	if *only != "" {
		fixtures = filter(fixtures, strings.Split(*only, ","))
	}

	var modelList []string
	for _, m := range strings.Split(*models, ",") {
		if m = strings.TrimSpace(m); m == "" {
			continue
		}
		if _, _, err := model.SplitModel(m); err != nil {
			return err
		}
		modelList = append(modelList, m)
	}
	seedList := make([]int64, *seeds)
	for i := range seedList {
		seedList[i] = int64(i + 1)
	}

	gateway := model.NewGateway(
		&model.Ollama{BaseURL: *ollamaURL, Schema: *schema, Think: *think},
		&model.Anthropic{APIKey: os.Getenv("ANTHROPIC_API_KEY")},
		model.FinishFake(),
	)
	r := &runner.Runner{Models: gateway, Dict: dict, Menu: style}

	dir := filepath.Join(*outDir, time.Now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	logFile, err := os.Create(filepath.Join(dir, "sessions.jsonl"))
	if err != nil {
		return err
	}
	defer logFile.Close()

	// Ctrl-C cancels the context; running sessions stop at their next call.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	total := len(fixtures) * len(modelList) * len(seedList)
	if receptorList != nil {
		total = len(fixtures) * len(modelList) * len(receptorList)
	}
	var mu sync.Mutex
	done := 0
	enc := json.NewEncoder(logFile)
	onDone := func(res eval.Result) {
		mu.Lock() // onDone runs on several goroutines
		defer mu.Unlock()
		done++
		_ = enc.Encode(res)
		mark := "✗"
		if res.Score.Fitting {
			mark = "✓"
		}
		if res.Score.Errored {
			mark = "!"
		}
		fmt.Fprintf(os.Stderr, "[%d/%d] %s %s %s\n", done, total, mark, res.Record.SessionID, errSuffix(res.Record))
	}

	reportFile, err := os.Create(filepath.Join(dir, "report.md"))
	if err != nil {
		return err
	}
	defer reportFile.Close()
	report := io.MultiWriter(stdout, reportFile)

	var calibFixtures []eval.Fixture
	if *calibration != "" {
		if calibFixtures, err = loadWith(*calibration, func(r io.Reader) ([]eval.Fixture, error) {
			return eval.LoadFixtures(r, dict)
		}); err != nil {
			return err
		}
		ids := map[string]bool{}
		for _, f := range calibFixtures {
			ids[f.ID] = true
		}
		for _, f := range fixtures {
			if ids[f.ID] {
				return fmt.Errorf("fixture %s is in both the calibration and the test set", f.ID)
			}
		}
	}

	if *probe != "" {
		var probeMu sync.Mutex
		probes := eval.RunProbes(ctx, gateway, dict, eval.ProbePlan{
			Receptors: receptorList, Models: modelList, Fixtures: append(append([]eval.Fixture(nil), calibFixtures...), fixtures...), Direct: *probe == "direct", Zone: *zone, Style: style,
		}, *parallel, func(done, total int, errMsg string) {
			probeMu.Lock()
			defer probeMu.Unlock()
			if errMsg != "" {
				fmt.Fprintf(os.Stderr, "[%d/%d] ! %s\n", done, total, errMsg)
			} else if done%9 == 0 || done == total {
				fmt.Fprintf(os.Stderr, "[%d/%d]\n", done, total)
			}
		})
		for _, p := range probes {
			_ = enc.Encode(p)
		}
		meta := fmt.Sprintf("probes %s · zone `%v` · receptors %d from `%s` · menu `%s` · two calls per receptor (options in both orders) · temperature 0 · fixtures %d",
			*probe, *zone, len(receptorList), *receptors, style, len(fixtures))
		if calibFixtures != nil {
			var order []string
			for _, g := range receptorList {
				order = append(order, g.MayDeposit[0])
			}
			eval.WriteHeldoutMarkdown(report, probes, calibFixtures, fixtures, order, meta)
		} else {
			eval.WriteProbeMarkdown(report, probes, fixtures, receptorList, meta)
		}
		fmt.Fprintf(os.Stderr, "\nwritten to %s\n", dir)
		return nil
	}

	if receptorList != nil {
		// One seed: at the genomes' temperature seeds barely change answers.
		results := eval.RunReceptors(ctx, r, eval.ReceptorPlan{
			Receptors: receptorList, Models: modelList, Seed: 1, Fixtures: fixtures,
		}, *parallel, onDone)
		meta := fmt.Sprintf("receptors %d from `%s` · menu `%s` · schema `%v` · think `%v` · fixtures %d",
			len(receptorList), *receptors, style, *schema, *think, len(fixtures))
		eval.WriteReceptorMarkdown(report, results, fixtures, receptorList, meta)
		fmt.Fprintf(os.Stderr, "\nwritten to %s\n", dir)
		return nil
	}

	results := eval.Run(ctx, r, eval.Plan{
		Genome: genome, Models: modelList, Seeds: seedList, Fixtures: fixtures,
	}, *parallel, onDone)
	meta := fmt.Sprintf("genome `%s` · menu `%s` · schema `%v` · think `%v` · seeds %d · fixtures %d",
		genome.ID, style, *schema, *think, len(seedList), len(fixtures))
	eval.WriteMarkdown(report, results, fixtures, meta)
	fmt.Fprintf(os.Stderr, "\nwritten to %s\n", dir)
	return nil
}

// loadReceptors reads every *.json genome in dir, in name order, and checks
// that each one is a valid receptor.
func loadReceptors(dir string, dict *core.Dictionary) ([]core.Genome, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no receptor genomes in %s", dir)
	}
	var out []core.Genome
	for _, p := range paths {
		g, err := loadWith(p, core.LoadGenome)
		if err != nil {
			return nil, err
		}
		if g.Role != core.RoleReceptor {
			return nil, fmt.Errorf("%s: role is %q, want %q", p, g.Role, core.RoleReceptor)
		}
		if err := g.Validate(dict); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		out = append(out, g)
	}
	return out, nil
}

// loadWith opens a file and decodes it with the given loader.
// It is generic: T is whatever the loader returns.
func loadWith[T any](path string, load func(io.Reader) (T, error)) (T, error) {
	var zero T
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer f.Close()
	v, err := load(f)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}

func filter(fs []eval.Fixture, ids []string) []eval.Fixture {
	want := map[string]bool{}
	for _, id := range ids {
		want[strings.TrimSpace(id)] = true
	}
	var out []eval.Fixture
	for _, f := range fs {
		if want[f.ID] {
			out = append(out, f)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		if !strings.Contains(v, "://") {
			v = "http://" + v
		}
		return v
	}
	return def
}

func errSuffix(rec runner.Record) string {
	if rec.Error != "" {
		return "error: " + rec.Error
	}
	return string(rec.Outcome)
}

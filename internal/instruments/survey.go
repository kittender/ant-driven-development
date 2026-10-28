package instruments

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"kittender/add/internal/honey"
)

// Commands are the workspace's tools. The defaults fit the TypeScript
// benchmark (benchmarks/tictactoe/typescript); `npx --no-install` refuses to
// download anything, so a missing tool is an error, not a surprise install.
type Commands struct {
	Typecheck []string
	Lint      []string
	Test      []string // must write VitestJSON and coverage CoverageJSON
	Dupes     []string // must write JscpdJSON
	// Output files, relative to the workspace.
	VitestJSON, CoverageJSON, JscpdJSON string
	// JUnitXML, when set, is read for failure messages missing from VitestJSON.
	JUnitXML string
	Timeout  time.Duration
}

// TypeScript returns the commands for the TypeScript benchmark.
func TypeScript() Commands {
	return Commands{
		Typecheck: []string{"npx", "--no-install", "tsc", "--noEmit", "--pretty", "false"},
		Lint:      []string{"npx", "--no-install", "eslint", "-f", "json", "."},
		// Two reporters: json for structure, junit for the error messages
		// (quickpickle rewrites stacks to point at .feature lines, and the
		// json reporter then keeps only the stack, without the message).
		Test:         []string{"npx", "--no-install", "vitest", "run", "--coverage", "--coverage.reportOnFailure", "--reporter=json", "--reporter=junit", "--outputFile.json=reports/vitest.json", "--outputFile.junit=reports/junit.xml"},
		Dupes:        []string{"npx", "--no-install", "jscpd", "src", "--silent", "--reporters", "json", "--output", "reports/jscpd"},
		VitestJSON:   "reports/vitest.json",
		JUnitXML:     "reports/junit.xml",
		CoverageJSON: "coverage/coverage-final.json",
		JscpdJSON:    "reports/jscpd/jscpd-report.json",
		Timeout:      5 * time.Minute,
	}
}

// Survey is one run of every instrument and the Prover's test run.
type Survey struct {
	At        time.Time                 `json:"at"`
	Compiler  []Finding                 `json:"compiler"`
	Lint      []Finding                 `json:"lint"`
	Coverage  []FunctionCoverage        `json:"coverage"`
	Untested  []Finding                 `json:"untested"`
	Dupes     []Finding                 `json:"dupes"`
	Cases     []TestCase                `json:"cases"`
	FileErrs  []TestCase                `json:"file_errors"`
	Scenarios map[string]ScenarioResult `json:"scenarios"`
	Outputs   []Output                  `json:"outputs"`
	// Problems are parse failures or tools that did not run. A survey with
	// problems is a harness bug (outline: parse validity must be 100%).
	Problems []string `json:"problems"`
}

// ProofPassed is the Prover's verdict on the whole workspace, apart from
// which scenarios are green: it compiles and lint has no errors.
func (s Survey) ProofPassed() bool {
	return len(s.Compiler) == 0 && LintErrors(s.Lint) == 0 && len(s.Problems) == 0
}

// Count returns how many capped scenarios are in each state.
func (s Survey) Count() map[State]int {
	c := map[State]int{}
	for _, r := range s.Scenarios {
		c[r.State]++
	}
	return c
}

// Run runs every instrument in dir and maps the test results onto the honey.
func Run(ctx context.Context, run Runner, dir string, cmds Commands, snap honey.Snapshot) Survey {
	s := Survey{At: time.Now()}
	do := func(argv []string) Output {
		c, cancel := context.WithTimeout(ctx, cmds.Timeout)
		defer cancel()
		o := run(c, dir, argv...)
		s.Outputs = append(s.Outputs, o)
		if o.Err != "" {
			s.Problems = append(s.Problems, fmt.Sprintf("%s: %s", strings.Join(argv, " "), o.Err))
		}
		return o
	}
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			s.Problems = append(s.Problems, fmt.Sprintf("%s not written: %v", name, err))
		}
		return b
	}

	o := do(cmds.Typecheck)
	s.Compiler = ParseTsc(dir, o.Stdout+"\n"+o.Stderr)
	if o.Exit != 0 && len(s.Compiler) == 0 && o.Err == "" {
		s.Problems = append(s.Problems, "typecheck failed with no parsable error: "+trim(o.Stdout+o.Stderr))
	}

	o = do(cmds.Lint)
	if lint, err := ParseESLint(dir, []byte(o.Stdout)); err != nil {
		s.Problems = append(s.Problems, err.Error()+": "+trim(o.Stderr))
	} else {
		s.Lint = lint
	}

	for _, f := range []string{cmds.VitestJSON, cmds.CoverageJSON, cmds.JUnitXML} {
		_ = os.Remove(filepath.Join(dir, f)) // never read a previous run's report
	}
	do(cmds.Test)
	if b := read(cmds.VitestJSON); b != nil {
		cases, ferrs, err := ParseVitest(dir, b)
		if err != nil {
			s.Problems = append(s.Problems, err.Error())
		}
		s.Cases, s.FileErrs = cases, ferrs
	}
	if cmds.JUnitXML != "" && s.Cases != nil {
		if b := read(cmds.JUnitXML); b != nil {
			msgs, err := ParseJUnit(dir, b)
			if err != nil {
				s.Problems = append(s.Problems, err.Error())
			}
			MergeMessages(s.Cases, msgs)
		}
	}
	if b := read(cmds.CoverageJSON); b != nil {
		cov, un, err := ParseCoverage(dir, b)
		if err != nil {
			s.Problems = append(s.Problems, err.Error())
		}
		s.Coverage, s.Untested = cov, un
	}

	_ = os.Remove(filepath.Join(dir, cmds.JscpdJSON))
	do(cmds.Dupes)
	if b := read(cmds.JscpdJSON); b != nil {
		d, err := ParseJscpd(dir, b)
		if err != nil {
			s.Problems = append(s.Problems, err.Error())
		}
		s.Dupes = d
	}

	s.Scenarios = MapScenarios(snap, s.Cases, s.FileErrs)
	return s
}

// Findings lists every finding of the survey that maps to a request.
func (s Survey) Findings() []Finding {
	var all []Finding
	for _, group := range [][]Finding{s.Compiler, s.Lint, s.Untested, s.Dupes} {
		for _, f := range group {
			if f.Signal != "" {
				all = append(all, f)
			}
		}
	}
	return all
}

// Facts builds the FACTS block for one target scenario and the files a
// session may write. Each fact says what it proves and what it does not.
func (s Survey) Facts(target string, files []string) []Fact {
	var fs []Fact
	in := map[string]bool{}
	for _, f := range files {
		in[f] = true
	}
	if r, ok := s.Scenarios[target]; ok {
		f := Fact{Target: target, Kind: "tests", Source: "prover"}
		switch r.State {
		case Green:
			f.Text = fmt.Sprintf("tests: scenario %q passes (%d test cases)", target, r.Tests)
			f.Proves, f.Not = "these examples behave as the scenario says", "other inputs work, or the code is clean"
		case Undefined:
			f.Text = fmt.Sprintf("tests: scenario %q has steps with no definition yet. The runner says:\n%s", target, indent(cleanFailure(r.Failure)))
			f.Proves, f.Not = "the step definitions are missing", "anything about the game code"
		case Red:
			f.Text = fmt.Sprintf("tests: scenario %q fails. The runner says:\n%s", target, indent(cleanFailure(r.Failure)))
			f.Proves, f.Not = "this scenario does not behave as written yet", "where the fault is: test or code"
		default:
			f.Text = fmt.Sprintf("tests: no test result matched scenario %q", target)
		}
		fs = append(fs, f)
	}
	c := s.Count()
	fs = append(fs, Fact{Kind: "tests", Source: "prover",
		Text:   fmt.Sprintf("tests: capped scenarios %d green, %d red, %d with undefined steps", c[Green], c[Red], c[Undefined]+c[Missing]),
		Proves: "how far the whole spec is", Not: "that a green scenario is tested well"})
	if len(s.Compiler) == 0 {
		fs = append(fs, Fact{Kind: "build", Source: "compiler", Text: "compiler: no type errors", Proves: "the types agree", Not: "the behaviour is right"})
	}
	for _, f := range s.Compiler {
		fs = append(fs, Fact{Target: f.File, Kind: "build", Source: "compiler", Text: fmt.Sprintf("compiler: %s:%d %s %s", f.File, f.Line, f.Rule, f.Message)})
	}
	nl := 0
	for _, f := range s.Lint {
		if in[f.File] || f.Severity == "error" {
			nl++
			fs = append(fs, Fact{Target: f.File, Kind: "lint", Source: "lint", Text: fmt.Sprintf("lint %s: %s:%d %s (%s)", f.Severity, f.File, f.Line, f.Message, f.Rule)})
		}
	}
	if nl == 0 {
		fs = append(fs, Fact{Kind: "lint", Source: "lint", Text: "lint: no errors or warnings in the files you may write", Proves: "the lint rules hold", Not: "the code is correct"})
	}
	var cov []string
	for _, f := range s.Coverage {
		if in[f.File] {
			cov = append(cov, fmt.Sprintf("%s %d", f.Name, f.Calls))
		}
	}
	if len(cov) > 0 {
		sort.Strings(cov)
		fs = append(fs, Fact{Kind: "coverage", Source: "coverage", Text: "coverage, calls by the tests per function: " + strings.Join(cov, ", "), Proves: "which functions the tests reach", Not: "that their results are checked"})
	}
	for _, f := range s.Dupes {
		if in[f.File] {
			fs = append(fs, Fact{Target: f.File, Kind: "duplication", Source: "duplication", Text: fmt.Sprintf("duplication: %s:%d %s", f.File, f.Line, f.Message)})
		}
	}
	return fs
}

// AllFacts lists every fact the survey can state, for the instrument check
// (outline, Part A: 20 sampled facts per run are checked against the repository).
func (s Survey) AllFacts() []Fact {
	var files []string
	seen := map[string]bool{}
	for _, f := range s.Coverage {
		if !seen[f.File] {
			seen[f.File] = true
			files = append(files, f.File)
		}
	}
	var fs []Fact
	keys := make([]string, 0, len(s.Scenarios))
	for k := range s.Scenarios {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fs = append(fs, s.Facts(k, files)[0])
	}
	return append(fs, s.Facts("", files)...)
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

// cleanFailure drops stack-trace lines ("at file:line"): they point into the
// test runner and only cost tokens. The message and any snippet stay.
func cleanFailure(s string) string {
	var keep []string
	for _, l := range strings.Split(s, "\n") {
		tl := strings.TrimSpace(l)
		if strings.HasPrefix(tl, "at ") || tl == "…" {
			continue
		}
		keep = append(keep, strings.TrimRight(l, " "))
	}
	return strings.TrimSpace(strings.Join(keep, "\n"))
}

func indent(s string) string {
	return "    " + strings.ReplaceAll(s, "\n", "\n    ")
}

package instruments

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"kittender/add/internal/honey"
)

// State of one capped scenario after a test run.
type State string

const (
	Green     State = "green"     // every test of the scenario passed
	Red       State = "red"       // a test failed with its steps defined
	Undefined State = "undefined" // a step has no definition yet (the tester's stimulus)
	Missing   State = "missing"   // no test result matched the scenario
)

// ScenarioResult is the Prover's view of one scenario.
type ScenarioResult struct {
	Key     string   `json:"key"`
	State   State    `json:"state"`
	Tests   int      `json:"tests"`   // test cases matched (examples of an outline count apart)
	Failure string   `json:"failure"` // first failure message, trimmed
	Titles  []string `json:"titles"`  // matched test titles, to check the mapping
}

type vitestReport struct {
	TestResults []struct {
		Name             string `json:"name"`
		Message          string `json:"message"`
		AssertionResults []struct {
			AncestorTitles  []string `json:"ancestorTitles"`
			Title           string   `json:"title"`
			FullName        string   `json:"fullName"`
			Status          string   `json:"status"`
			FailureMessages []string `json:"failureMessages"`
		} `json:"assertionResults"`
	} `json:"testResults"`
}

// UndefinedStep matches the failure of a step with no definition.
// The exact wording depends on the runner; the pilot's -check run prints
// the real messages so this can be confirmed before any model runs.
var UndefinedStep = regexp.MustCompile(`(?i)undefined step|^\s*undefined\b|\bundefined\.\s*implement|step (is )?not defined|no step definition|step definition (not found|missing)|missing step|could not find (a )?step|no step matches|snippet`)

// TestCase is one test result from the runner.
type TestCase struct {
	File    string `json:"file"`
	Title   string `json:"title"`
	Status  string `json:"status"`
	Failure string `json:"failure,omitempty"`
}

// ParseVitest reads `vitest run --reporter=json` output into test cases,
// plus file-level errors (a file that failed to load has no test cases).
func ParseVitest(root string, out []byte) ([]TestCase, []TestCase, error) {
	var r vitestReport
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, nil, fmt.Errorf("vitest json: %w", err)
	}
	var cases, fileErrs []TestCase
	for _, f := range r.TestResults {
		file := rel(root, f.Name)
		if len(f.AssertionResults) == 0 && f.Message != "" {
			fileErrs = append(fileErrs, TestCase{File: file, Status: "failed", Failure: trim(f.Message)})
		}
		for _, a := range f.AssertionResults {
			title := strings.Join(append(append([]string{}, a.AncestorTitles...), a.Title), " > ")
			c := TestCase{File: file, Title: title, Status: a.Status}
			if len(a.FailureMessages) > 0 {
				c.Failure = trim(a.FailureMessages[0])
			}
			cases = append(cases, c)
		}
	}
	return cases, fileErrs, nil
}

func trim(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 1200 {
		s = s[:1200] + " …"
	}
	return s
}

// MapScenarios assigns each test case of a .feature file to the scenario of
// that file whose name it contains (the longest such name wins, so "X wins"
// and "X wins with three in a line" do not collide), and folds the cases
// into one state per scenario.
func MapScenarios(snap honey.Snapshot, cases, fileErrs []TestCase) map[string]ScenarioResult {
	byFile := map[string][]honey.Scenario{}
	for _, k := range snap.Order {
		s := snap.Scenarios[k]
		byFile[s.File] = append(byFile[s.File], s)
	}
	res := map[string]ScenarioResult{}
	for _, k := range snap.Order {
		res[k] = ScenarioResult{Key: k, State: Missing}
	}
	for _, c := range cases {
		base := filepath.Base(c.File)
		var best *honey.Scenario
		for i, s := range byFile[base] {
			if strings.Contains(c.Title, s.Name) && (best == nil || len(s.Name) > len(best.Name)) {
				best = &byFile[base][i]
			}
		}
		if best == nil {
			continue
		}
		r := res[best.Key()]
		r.Tests++
		r.Titles = append(r.Titles, c.Title)
		r.State = fold(r.State, c)
		if r.Failure == "" && c.Failure != "" {
			r.Failure = c.Failure
		}
		res[best.Key()] = r
	}
	// A feature file that failed to load fails every scenario in it.
	for _, e := range fileErrs {
		for _, s := range byFile[filepath.Base(e.File)] {
			r := res[s.Key()]
			if r.Tests == 0 {
				r.State, r.Failure = classify(e), e.Failure
				res[s.Key()] = r
			}
		}
	}
	return res
}

func classify(c TestCase) State {
	switch c.Status {
	case "passed":
		return Green
	case "skipped", "pending", "todo", "disabled":
		return Undefined
	}
	if UndefinedStep.MatchString(c.Failure) {
		return Undefined
	}
	return Red
}

// fold combines the state so far with one more test case: any undefined
// step makes the scenario undefined, else any failure makes it red, and it
// is green only when every case passed.
func fold(so State, c TestCase) State {
	s := classify(c)
	rank := map[State]int{Missing: 0, Green: 1, Red: 2, Undefined: 3}
	if so == Missing || rank[s] > rank[so] {
		return s
	}
	return so
}

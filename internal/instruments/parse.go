package instruments

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// rel turns an absolute path from a tool into a workspace-relative one.
func rel(root, p string) string {
	if r, err := filepath.Rel(root, p); err == nil && !strings.HasPrefix(r, "..") {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(p)
}

var tscLine = regexp.MustCompile(`^(.+?)\((\d+),(\d+)\): error (TS\d+): (.*)$`)

// tscMissing are the compiler errors that mean "this should exist and does not".
var tscMissing = map[string]bool{"TS2304": true, "TS2305": true, "TS2307": true, "TS2339": true, "TS2551": true, "TS2724": true}

// ParseTsc reads `tsc --noEmit --pretty false` output.
func ParseTsc(root, out string) []Finding {
	var fs []Finding
	for _, l := range strings.Split(out, "\n") {
		m := tscLine.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue
		}
		line, _ := strconv.Atoi(m[2])
		sig := "🐛"
		if tscMissing[m[4]] {
			sig = "🌱"
		}
		fs = append(fs, Finding{Instrument: "compiler", File: rel(root, m[1]), Line: line, Rule: m[4], Severity: "error", Message: m[5], Signal: sig})
	}
	return fs
}

// LintMap maps an ESLint rule to a request. Rules not listed map to 🧹.
var LintMap = map[string]string{
	"no-restricted-imports": "🧩", // the proposed plot's proof
	"no-restricted-globals": "🧩",
	"no-empty":              "🐛",
	"eqeqeq":                "🐛",
	"no-unreachable":        "🐛",
	"no-fallthrough":        "🐛",
}

type eslintFile struct {
	FilePath string `json:"filePath"`
	Messages []struct {
		RuleID   *string `json:"ruleId"`
		Severity int     `json:"severity"`
		Message  string  `json:"message"`
		Line     int     `json:"line"`
	} `json:"messages"`
}

// ParseESLint reads `eslint -f json` output.
func ParseESLint(root string, out []byte) ([]Finding, error) {
	var files []eslintFile
	if err := json.Unmarshal(out, &files); err != nil {
		return nil, fmt.Errorf("eslint json: %w", err)
	}
	var fs []Finding
	for _, f := range files {
		for _, m := range f.Messages {
			rule := "parse"
			if m.RuleID != nil {
				rule = *m.RuleID
			}
			sev := "warning"
			if m.Severity >= 2 {
				sev = "error"
			}
			sig, ok := LintMap[rule]
			switch {
			case ok:
			case strings.Contains(rule, "jsdoc"):
				sig = "📝"
			case rule == "parse":
				sig = "🐛"
			default:
				sig = "🧹"
			}
			fs = append(fs, Finding{Instrument: "lint", File: rel(root, f.FilePath), Line: m.Line, Rule: rule, Severity: sev, Message: m.Message, Signal: sig})
		}
	}
	return fs, nil
}

// LintErrors counts error-level findings: they fail the proof; warnings do not.
func LintErrors(fs []Finding) int {
	n := 0
	for _, f := range fs {
		if f.Severity == "error" {
			n++
		}
	}
	return n
}

// FunctionCoverage is how often the tests called one function.
type FunctionCoverage struct {
	File  string `json:"file"`
	Name  string `json:"name"`
	Line  int    `json:"line"`
	Calls int    `json:"calls"`
}

type istanbulFile struct {
	Path  string `json:"path"`
	FnMap map[string]struct {
		Name string `json:"name"`
		Decl struct {
			Start struct {
				Line int `json:"line"`
			} `json:"start"`
		} `json:"decl"`
	} `json:"fnMap"`
	F map[string]int `json:"f"`
}

// ParseCoverage reads Istanbul's coverage-final.json (written by Vitest's v8
// provider) and returns per-function call counts and a 🧪 finding for each
// function the tests never call.
func ParseCoverage(root string, out []byte) ([]FunctionCoverage, []Finding, error) {
	var files map[string]istanbulFile
	if err := json.Unmarshal(out, &files); err != nil {
		return nil, nil, fmt.Errorf("coverage json: %w", err)
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var cov []FunctionCoverage
	var fs []Finding
	for _, k := range keys {
		f := files[k]
		path := f.Path
		if path == "" {
			path = k
		}
		ids := make([]int, 0, len(f.FnMap))
		for id := range f.FnMap {
			n, _ := strconv.Atoi(id)
			ids = append(ids, n)
		}
		sort.Ints(ids)
		for _, n := range ids {
			id := strconv.Itoa(n)
			fn := f.FnMap[id]
			c := FunctionCoverage{File: rel(root, path), Name: fn.Name, Line: fn.Decl.Start.Line, Calls: f.F[id]}
			cov = append(cov, c)
			if c.Calls == 0 {
				fs = append(fs, Finding{Instrument: "coverage", File: c.File, Line: c.Line, Message: fmt.Sprintf("function %s is never called by the tests", c.Name), Signal: "🧪"})
			}
		}
	}
	return cov, fs, nil
}

type jscpdReport struct {
	Duplicates []struct {
		Lines     int `json:"lines"`
		FirstFile struct {
			Name  string `json:"name"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		} `json:"firstFile"`
		SecondFile struct {
			Name  string `json:"name"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		} `json:"secondFile"`
	} `json:"duplicates"`
}

// ParseJscpd reads jscpd's json report (reports/jscpd/jscpd-report.json).
func ParseJscpd(root string, out []byte) ([]Finding, error) {
	var r jscpdReport
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("jscpd json: %w", err)
	}
	var fs []Finding
	for _, d := range r.Duplicates {
		a, b := rel(root, d.FirstFile.Name), rel(root, d.SecondFile.Name)
		fs = append(fs,
			Finding{Instrument: "duplication", File: a, Line: d.FirstFile.Start, Message: fmt.Sprintf("%d lines duplicated in %s:%d", d.Lines, b, d.SecondFile.Start), Signal: "🧹"},
			Finding{Instrument: "duplication", File: b, Line: d.SecondFile.Start, Message: fmt.Sprintf("%d lines duplicated in %s:%d", d.Lines, a, d.FirstFile.Start), Signal: "🧹"},
		)
	}
	return fs, nil
}

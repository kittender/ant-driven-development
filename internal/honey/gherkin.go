// Package honey reads the colony's honey (Gherkin .feature files) and
// classifies how it changed between two snapshots.
//
// It is a reader for the subset the benchmark uses: Feature, Background,
// Scenario, Scenario Outline, Examples, tags, steps, data tables. It is not
// a full Gherkin parser and never needs to be: it only has to see what a
// change did to a capped scenario.
package honey

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// CappedTag marks a scenario as capped honey (🍯).
const CappedTag = "@capped"

// Scenario is one cell: a scenario or a scenario outline.
type Scenario struct {
	File     string   // base name of the .feature file
	Feature  string   // feature title
	Name     string   // scenario title
	Outline  bool     // Scenario Outline
	Tags     []string // tags on the scenario, e.g. @capped
	Steps    []string // step lines, trimmed, data table rows attached as "| a | b |"
	Examples []string // example rows, trimmed, header first, for outlines
	Line     int      // 1-based line of the Scenario keyword
}

// Key identifies a scenario across snapshots: file and title.
func (s Scenario) Key() string { return s.File + " > " + s.Name }

// Capped reports whether the scenario carries the cap tag.
func (s Scenario) Capped() bool {
	for _, t := range s.Tags {
		if t == CappedTag {
			return true
		}
	}
	return false
}

// Text renders the scenario back as Gherkin, for bundles and logs.
func (s Scenario) Text() string {
	var b strings.Builder
	if len(s.Tags) > 0 {
		b.WriteString("  " + strings.Join(s.Tags, " ") + "\n")
	}
	kw := "Scenario"
	if s.Outline {
		kw = "Scenario Outline"
	}
	b.WriteString("  " + kw + ": " + s.Name + "\n")
	for _, st := range s.Steps {
		if strings.HasPrefix(st, "|") {
			b.WriteString("      " + st + "\n")
		} else {
			b.WriteString("    " + st + "\n")
		}
	}
	if len(s.Examples) > 0 {
		b.WriteString("\n    Examples:\n")
		for _, r := range s.Examples {
			b.WriteString("      " + r + "\n")
		}
	}
	return b.String()
}

// Feature is one .feature file.
type Feature struct {
	File       string
	Title      string
	Background []string
	Scenarios  []Scenario
	Source     string
}

// Parse reads one feature file's text.
func Parse(file, src string) Feature {
	f := Feature{File: file, Source: src}
	var cur *Scenario
	var pending []string
	section := "" // "background", "scenario", "examples"
	flush := func() {
		if cur != nil {
			f.Scenarios = append(f.Scenarios, *cur)
			cur = nil
		}
	}
	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		case strings.HasPrefix(line, "@"):
			pending = append(pending, strings.Fields(line)...)
		case strings.HasPrefix(line, "Feature:"):
			f.Title = strings.TrimSpace(strings.TrimPrefix(line, "Feature:"))
			pending = nil
		case strings.HasPrefix(line, "Background:"):
			flush()
			section = "background"
		case strings.HasPrefix(line, "Scenario Outline:") || strings.HasPrefix(line, "Scenario Template:"):
			flush()
			name := line[strings.Index(line, ":")+1:]
			cur = &Scenario{File: file, Feature: f.Title, Name: strings.TrimSpace(name), Outline: true, Tags: pending, Line: i + 1}
			pending, section = nil, "scenario"
		case strings.HasPrefix(line, "Scenario:") || strings.HasPrefix(line, "Example:"):
			flush()
			name := line[strings.Index(line, ":")+1:]
			cur = &Scenario{File: file, Feature: f.Title, Name: strings.TrimSpace(name), Tags: pending, Line: i + 1}
			pending, section = nil, "scenario"
		case strings.HasPrefix(line, "Examples:") || strings.HasPrefix(line, "Scenarios:"):
			section = "examples"
		case section == "examples" && cur != nil && strings.HasPrefix(line, "|"):
			cur.Examples = append(cur.Examples, normRow(line))
		case section == "background":
			if strings.HasPrefix(line, "|") {
				line = normRow(line)
			}
			f.Background = append(f.Background, line)
		case section == "scenario" && cur != nil:
			if strings.HasPrefix(line, "|") {
				line = normRow(line)
			}
			cur.Steps = append(cur.Steps, line)
		}
	}
	flush()
	return f
}

// normRow collapses the spacing of a table row so alignment changes are not changes.
func normRow(line string) string {
	cells := strings.Split(strings.Trim(line, "|"), "|")
	for i, c := range cells {
		cells[i] = strings.Join(strings.Fields(c), " ")
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

// Snapshot is the honey of a workspace at one moment, keyed by scenario.
type Snapshot struct {
	Features  []Feature
	Scenarios map[string]Scenario
	Order     []string // scenario keys in file order
	Files     map[string]string
}

// Load reads every .feature file in dir (not recursive), in name order.
func Load(dir string) (Snapshot, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.feature"))
	if err != nil {
		return Snapshot{}, err
	}
	sort.Strings(paths)
	files := map[string]string{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return Snapshot{}, err
		}
		files[filepath.Base(p)] = string(b)
	}
	return FromFiles(files), nil
}

// FromFiles builds a snapshot from file contents (base name → text).
func FromFiles(files map[string]string) Snapshot {
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	s := Snapshot{Scenarios: map[string]Scenario{}, Files: files}
	for _, n := range names {
		f := Parse(n, files[n])
		s.Features = append(s.Features, f)
		for _, sc := range f.Scenarios {
			if _, dup := s.Scenarios[sc.Key()]; !dup {
				s.Order = append(s.Order, sc.Key())
			}
			s.Scenarios[sc.Key()] = sc
		}
	}
	return s
}

// Feature returns the feature parsed from a file, if present.
func (s Snapshot) Feature(file string) (Feature, bool) {
	for _, f := range s.Features {
		if f.File == file {
			return f, true
		}
	}
	return Feature{}, false
}

// Capped returns the capped scenarios in file order.
func (s Snapshot) Capped() []Scenario {
	var out []Scenario
	for _, k := range s.Order {
		if sc := s.Scenarios[k]; sc.Capped() {
			out = append(out, sc)
		}
	}
	return out
}

// Package core holds the ADD domain: pure types and rules, no IO.
//
// Nothing in this package may import net, os/exec, database or file APIs.
// Think of it as the rulebook of a board game: it says what is legal,
// it never moves a piece itself.
package core

import (
	"fmt"
	"path"
	"strings"
)

// TargetKind says what a signal is about. See the architecture, "Data model > Target".
type TargetKind int

const (
	TargetFile   TargetKind = iota // src/a.ts
	TargetSymbol                   // src/a.ts#onSubmit
	TargetAgeant                   // @leni03
	TargetFrame                    // honey:credit-search/search-by-amount
	TargetCell                     // honey:credit-search/search-by-amount/empty-amount
	TargetPlot                     // garden:credit-search/data-access
)

// Target is a parsed target. Only the fields that apply to Kind are set.
type Target struct {
	Kind   TargetKind
	Path   string // file path, frame id, or plot id
	Symbol string // symbol name for TargetSymbol, cell id for TargetCell
	ID     string // ageant id for TargetAgeant
}

// ParseTarget parses the textual form of a target.
// It rejects anything that could escape the workspace: absolute paths,
// "..", backslashes, whitespace.
func ParseTarget(s string) (Target, error) {
	if s == "" {
		return Target{}, fmt.Errorf("empty target")
	}
	if strings.ContainsAny(s, " \t\r\n\\") {
		return Target{}, fmt.Errorf("target %q contains whitespace or a backslash", s)
	}
	switch {
	case strings.HasPrefix(s, "@"):
		id := s[1:]
		if id == "" {
			return Target{}, fmt.Errorf("empty ageant id")
		}
		return Target{Kind: TargetAgeant, ID: id}, nil

	case strings.HasPrefix(s, "honey:"):
		rest := s[len("honey:"):]
		if err := checkRelPath(rest); err != nil {
			return Target{}, err
		}
		// A frame is honey:<feature>/<frame>; a cell adds /<cell>.
		parts := strings.Split(rest, "/")
		switch {
		case len(parts) == 2:
			return Target{Kind: TargetFrame, Path: rest}, nil
		case len(parts) == 3:
			return Target{Kind: TargetCell, Path: parts[0] + "/" + parts[1], Symbol: parts[2]}, nil
		default:
			return Target{}, fmt.Errorf("honey target %q must be feature/frame or feature/frame/cell", s)
		}

	case strings.HasPrefix(s, "garden:"):
		rest := s[len("garden:"):]
		if err := checkRelPath(rest); err != nil {
			return Target{}, err
		}
		return Target{Kind: TargetPlot, Path: rest}, nil
	}

	file, symbol, hasSymbol := strings.Cut(s, "#")
	if err := checkRelPath(file); err != nil {
		return Target{}, err
	}
	if hasSymbol {
		if symbol == "" || strings.Contains(symbol, "#") {
			return Target{}, fmt.Errorf("bad symbol in target %q", s)
		}
		return Target{Kind: TargetSymbol, Path: file, Symbol: symbol}, nil
	}
	return Target{Kind: TargetFile, Path: file}, nil
}

func checkRelPath(p string) error {
	if p == "" {
		return fmt.Errorf("empty path")
	}
	if strings.HasPrefix(p, "/") {
		return fmt.Errorf("path %q is absolute", p)
	}
	if path.Clean(p) != p {
		return fmt.Errorf("path %q is not clean", p)
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." || seg == "." {
			return fmt.Errorf("path %q leaves its folder", p)
		}
	}
	return nil
}

// String returns the canonical textual form. ParseTarget(t.String()) == t.
func (t Target) String() string {
	switch t.Kind {
	case TargetSymbol:
		return t.Path + "#" + t.Symbol
	case TargetAgeant:
		return "@" + t.ID
	case TargetFrame:
		return "honey:" + t.Path
	case TargetCell:
		return "honey:" + t.Path + "/" + t.Symbol
	case TargetPlot:
		return "garden:" + t.Path
	default:
		return t.Path
	}
}

// File returns the workspace file a file or symbol target lives in, or "".
func (t Target) File() string {
	if t.Kind == TargetFile || t.Kind == TargetSymbol {
		return t.Path
	}
	return ""
}

// Within reports whether t is the same as scope or lies in the same file.
// A scout sent to src/a.ts#onSubmit may mark src/a.ts or another symbol in it.
func (t Target) Within(scope Target) bool {
	if f := scope.File(); f != "" {
		return t.File() == f
	}
	return t == scope
}

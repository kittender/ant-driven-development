package pilot

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The step file of a feature is written by the harness, not by a model: a
// tester sends one body per step skeleton, and the harness puts each body
// under its exact first line, keeps the steps earlier scenarios added, and
// writes the imports the bodies use. After the fifth trial run the model
// still rewrote correct first lines into broken ones when it wrote the file.

// StepDef is one registered step in a harness-written step file.
type StepDef struct {
	Keyword    string
	Expression string
	Params     []string // "row: number", "table: DataTable"
	Body       string   // the lines inside the arrow function, without indentation
}

var defLine = regexp.MustCompile(`^(Given|When|Then)\(("(?:[^"\\]|\\.)*"), \((.*)\) => \{$`)

// ParseStepFile reads the steps of a file the harness wrote. Lines outside
// step definitions (the header, imports) are ignored: they are regenerated.
func ParseStepFile(src string) []StepDef {
	var defs []StepDef
	lines := strings.Split(src, "\n")
	for i := 0; i < len(lines); i++ {
		m := defLine.FindStringSubmatch(lines[i])
		if m == nil {
			continue
		}
		expr, err := strconv.Unquote(m[2])
		if err != nil {
			continue
		}
		d := StepDef{Keyword: m[1], Expression: expr}
		for _, a := range strings.Split(m[3], ", ") {
			if a != "world" && a != "" {
				d.Params = append(d.Params, a)
			}
		}
		var body []string
		for i++; i < len(lines) && lines[i] != "});"; i++ {
			body = append(body, strings.TrimPrefix(lines[i], "  "))
		}
		d.Body = strings.Join(body, "\n")
		defs = append(defs, d)
	}
	return defs
}

var stepCall = regexp.MustCompile(`\b(Given|When|Then)\s*\(`)

// CleanBody turns what a model sent for one step into body lines: code
// fences and a wrapping registration are refused, surrounding braces of a
// bare block are dropped.
func CleanBody(raw string) (string, error) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	trimBlank := func() {
		for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
			lines = lines[1:]
		}
		for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
			lines = lines[:len(lines)-1]
		}
	}
	trimBlank()
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "```") {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "```" {
		lines = lines[:len(lines)-1]
	}
	trimBlank()
	if stepCall.MatchString(strings.Join(lines, "\n")) {
		return "", fmt.Errorf("a body must not register a step (Given/When/Then): send only the code inside the step")
	}
	if len(lines) >= 2 && strings.TrimSpace(lines[0]) == "{" && strings.TrimSpace(lines[len(lines)-1]) == "}" {
		lines = lines[1 : len(lines)-1]
		trimBlank()
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("an empty body")
	}
	// Re-indent from the model's indentation to none; the file adds two spaces.
	minIndent := -1
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n := len(l) - len(strings.TrimLeft(l, " \t"))
		if minIndent == -1 || n < minIndent {
			minIndent = n
		}
	}
	for i, l := range lines {
		if len(l) >= minIndent {
			lines[i] = strings.TrimRight(l[minIndent:], " \t")
		} else {
			lines[i] = strings.TrimSpace(l)
		}
	}
	return strings.Join(lines, "\n"), nil
}

var contractTypes = []string{"Player", "Cell", "Status", "Rejection", "GameState", "MoveResult", "Score", "Match"}

// RenderStepFile writes a step file from its steps, with only the imports
// its code uses (lint refuses unused imports).
func RenderStepFile(defs []StepDef) string {
	var code strings.Builder
	keywords := map[string]bool{}
	for i, d := range defs {
		if i > 0 {
			code.WriteString("\n")
		}
		keywords[d.Keyword] = true
		args := append([]string{"world"}, d.Params...)
		fmt.Fprintf(&code, "%s(%s, (%s) => {\n", d.Keyword, strconv.Quote(d.Expression), strings.Join(args, ", "))
		for _, l := range strings.Split(d.Body, "\n") {
			if strings.TrimSpace(l) == "" {
				code.WriteString("\n")
				continue
			}
			code.WriteString("  " + l + "\n")
		}
		code.WriteString("});\n")
	}
	body := code.String()
	uses := func(word string) bool { return regexp.MustCompile(`\b` + word + `\b`).MatchString(body) }

	var h strings.Builder
	h.WriteString("// Written by the harness (add-pilot) from the testers' step bodies. One step per definition.\n")
	if strings.Contains(body, "expect(") {
		h.WriteString("import { expect } from 'vitest';\n")
	}
	var fromWorld []string
	for _, k := range []string{"Given", "When", "Then"} {
		if keywords[k] {
			fromWorld = append(fromWorld, k)
		}
	}
	if strings.Contains(body, ": DataTable") {
		fromWorld = append(fromWorld, "type DataTable")
	}
	if len(fromWorld) > 0 {
		h.WriteString("import { " + strings.Join(fromWorld, ", ") + " } from './world';\n")
	}
	if strings.Contains(body, "game.") {
		h.WriteString("import { game } from '../../src/game/game';\n")
	}
	var types []string
	for _, t := range contractTypes {
		if uses(t) {
			types = append(types, t)
		}
	}
	sort.Strings(types)
	if len(types) > 0 {
		h.WriteString("import type { " + strings.Join(types, ", ") + " } from '../../src/game/contract';\n")
	}
	return h.String() + "\n" + body
}

// MergeSteps keeps the existing steps (other scenarios of the feature) and
// adds or replaces the given ones, in first-seen order.
func MergeSteps(existing, add []StepDef) []StepDef {
	idx := map[string]int{}
	out := append([]StepDef{}, existing...)
	for i, d := range out {
		idx[d.Expression] = i
	}
	for _, d := range add {
		if i, ok := idx[d.Expression]; ok {
			out[i] = d
			continue
		}
		idx[d.Expression] = len(out)
		out = append(out, d)
	}
	return out
}

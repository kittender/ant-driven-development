package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"kittender/add/internal/core"
)

// ParseError means the model's reply was not one action from the menu.
// The Runner counts it as a step and as a malformed reply.
type ParseError struct{ Reason string }

func (e *ParseError) Error() string { return "unparseable action: " + e.Reason }

func parseErr(format string, a ...any) error { return &ParseError{fmt.Sprintf(format, a...)} }

var thinkBlock = regexp.MustCompile(`(?s)<think>.*?</think>`)

// ParseStructured decodes schema-bound JSON or a tool call.
func ParseStructured(raw json.RawMessage, menu Menu) (core.Action, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var a core.Action
	if err := dec.Decode(&a); err != nil {
		return core.Action{}, parseErr("bad JSON: %v", err)
	}
	if a.Emoji == "" {
		a.Emoji, a.Signal = a.Signal, "" // alias menu: "signal" carries the choice
	}
	if menu.Binary && menu.FoundAnswer {
		return foundAction(a.Found, a.Note, menu)
	}
	if menu.Binary {
		return binaryAction(a.Answer, a.Note, menu)
	}
	if menu.NoneOption {
		return noneOrDeposit(a, menu)
	}
	return checkShape(a, menu)
}

// noneOrDeposit maps a none-option scout's single choice onto the real tools.
func noneOrDeposit(a core.Action, menu Menu) (core.Action, error) {
	if strings.EqualFold(strings.TrimSpace(a.Emoji), NoneChoice) {
		return core.Action{Tool: core.ToolFinish, Outcome: core.OutcomeDone, Note: a.Note}, nil
	}
	a.Tool, a.Outcome = core.ToolDeposit, ""
	return checkShape(a, menu)
}

// emptyFound lists replies that mean "nothing found" even though not empty.
var emptyFound = map[string]bool{"none": true, "nothing": true, "no": true, "n/a": true, "-": true, "null": true}

// foundAction: a named finding deposits the receptor's signal; nothing found finishes.
func foundAction(found, note string, menu Menu) (core.Action, error) {
	if len(menu.Emoji) == 0 || len(menu.Targets) == 0 {
		return core.Action{}, parseErr("binary menu without a signal or a target")
	}
	f := strings.TrimSpace(found)
	if f == "" || emptyFound[strings.ToLower(strings.Trim(f, ".!"))] {
		return core.Action{Tool: core.ToolFinish, Outcome: core.OutcomeDone, Note: note, Answer: "no"}, nil
	}
	return core.Action{Tool: core.ToolDeposit, Target: menu.Targets[0], Emoji: menu.Emoji[0], Note: note, Answer: "yes", Found: f}, nil
}

// binaryAction maps a receptor's yes/no onto the two real tools.
func binaryAction(answer, note string, menu Menu) (core.Action, error) {
	if len(menu.Emoji) == 0 || len(menu.Targets) == 0 {
		return core.Action{}, parseErr("binary menu without a signal or a target")
	}
	switch strings.ToLower(strings.Trim(strings.TrimSpace(answer), ".!,:")) {
	case "yes":
		return core.Action{Tool: core.ToolDeposit, Target: menu.Targets[0], Emoji: menu.Emoji[0], Note: note, Answer: "yes"}, nil
	case "no":
		return core.Action{Tool: core.ToolFinish, Outcome: core.OutcomeDone, Note: note, Answer: "no"}, nil
	}
	return core.Action{}, parseErr("answer %q is not yes or no", answer)
}

// ParseText reads an action from free text. It accepts, in order:
//  1. a JSON object anywhere in the text (code fences allowed);
//  2. the line format, which small models produce most reliably:
//     deposit <target> <emoji>
//     finish <done|cannot> [note ...]
//
// Reasoning blocks such as <think>...</think> are ignored.
func ParseText(text string, menu Menu) (core.Action, error) {
	text = thinkBlock.ReplaceAllString(text, "")
	if obj, ok := firstJSONObject(text); ok {
		return ParseStructured(json.RawMessage(obj), menu)
	}
	if menu.NoneOption {
		// Line format: "deposit <target> <signal>" or "none <note>".
		for _, line := range strings.Split(text, "\n") {
			fields := strings.Fields(strings.Trim(strings.TrimSpace(line), "`*->"))
			if len(fields) == 0 {
				continue
			}
			switch strings.ToLower(strings.TrimSuffix(fields[0], ":")) {
			case NoneChoice:
				return core.Action{Tool: core.ToolFinish, Outcome: core.OutcomeDone, Note: strings.Join(fields[1:], " ")}, nil
			case string(core.ToolDeposit):
				if len(fields) != 3 {
					return core.Action{}, parseErr("deposit needs exactly a target and a signal, got %q", line)
				}
				return noneOrDeposit(core.Action{Target: fields[1], Emoji: fields[2]}, menu)
			}
		}
		return core.Action{}, parseErr("no choice found")
	}
	if menu.Binary && menu.FoundAnswer {
		// Line format: "found <what>" or "none <note>".
		for _, line := range strings.Split(text, "\n") {
			fields := strings.Fields(strings.Trim(strings.TrimSpace(line), "`*->"))
			if len(fields) == 0 {
				continue
			}
			switch strings.ToLower(strings.TrimSuffix(fields[0], ":")) {
			case "found":
				return foundAction(strings.Join(fields[1:], " "), "", menu)
			case NoneChoice:
				return foundAction("", strings.Join(fields[1:], " "), menu)
			}
			return core.Action{}, parseErr("reply %q is neither found nor none", line)
		}
		return core.Action{}, parseErr("no answer found")
	}
	if menu.Binary {
		// Line format for receptors: "yes <note>" or "no <note>".
		for _, line := range strings.Split(text, "\n") {
			fields := strings.Fields(strings.Trim(strings.TrimSpace(line), "`*->"))
			if len(fields) > 0 {
				return binaryAction(fields[0], strings.Join(fields[1:], " "), menu)
			}
		}
		return core.Action{}, parseErr("no answer found")
	}
	for _, line := range strings.Split(text, "\n") {
		fields := strings.Fields(strings.Trim(strings.TrimSpace(line), "`*->"))
		if len(fields) == 0 {
			continue
		}
		verb := core.Tool(strings.ToLower(strings.TrimSuffix(fields[0], ":")))
		if !slices.Contains(core.AllTools, verb) {
			continue
		}
		switch verb {
		case core.ToolDeposit:
			if len(fields) != 3 {
				return core.Action{}, parseErr("deposit needs exactly a target and an emoji, got %q", line)
			}
			return checkShape(core.Action{Tool: verb, Target: fields[1], Emoji: fields[2]}, menu)
		case core.ToolFinish:
			if len(fields) < 2 {
				return core.Action{}, parseErr("finish needs an outcome, got %q", line)
			}
			return checkShape(core.Action{
				Tool:    verb,
				Outcome: core.Outcome(strings.ToLower(fields[1])),
				Note:    strings.Join(fields[2:], " "),
			}, menu)
		default:
			// A real tool, but not one this level can parse from a line.
			return checkShape(core.Action{Tool: verb}, menu)
		}
	}
	return core.Action{}, parseErr("no action found")
}

// checkShape verifies the fields a tool needs. Whether the caller may USE
// the tool is the Tool Gateway's job, not ours.
func checkShape(a core.Action, menu Menu) (core.Action, error) {
	switch a.Tool {
	case core.ToolDeposit:
		if a.Target == "" || a.Emoji == "" {
			return core.Action{}, parseErr("deposit without target or emoji")
		}
	case core.ToolFinish:
		if a.Outcome != core.OutcomeDone && a.Outcome != core.OutcomeCannot {
			return core.Action{}, parseErr("finish outcome %q is not done or cannot", a.Outcome)
		}
	case "":
		return core.Action{}, parseErr("missing tool")
	default:
		if !slices.Contains(core.AllTools, a.Tool) {
			return core.Action{}, parseErr("unknown tool %q", a.Tool)
		}
	}
	return a, nil
}

// firstJSONObject returns the first balanced {...} in s, string-aware.
func firstJSONObject(s string) (string, bool) {
	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", false
	}
	depth, inStr, esc := 0, false, false
	for i := start; i < len(s); i++ {
		c := s[i]
		switch {
		case esc:
			esc = false
		case inStr && c == '\\':
			esc = true
		case c == '"':
			inStr = !inStr
		case inStr:
		case c == '{':
			depth++
		case c == '}':
			depth--
			if depth == 0 {
				return s[start : i+1], true
			}
		}
	}
	return "", false
}

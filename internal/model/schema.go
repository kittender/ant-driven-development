package model

import (
	"bytes"
	"encoding/json"

	"kittender/add/internal/core"
)

// ActionSchema returns a JSON Schema for one action from the menu.
//
// It is deliberately flat (one object, "tool" as an enum) instead of a
// oneOf per tool: local runtimes turn the schema into a grammar, and flat
// enums convert reliably.
//
// Two details matter for grammar-constrained decoding (Ollama "format"):
//   - Field ORDER: the grammar follows property order. "note" (the reason)
//     comes first, then "tool", then the fields that depend on it. Go maps marshal keys in
//     alphabetical order ("emoji" before "tool"), so properties use an
//     ordered type instead of a map.
//   - Every field is REQUIRED: an optional field is one the model may skip,
//     and small models do skip "emoji". Fields that do not apply to the
//     chosen tool take "" and are ignored by the parser.
func ActionSchema(menu Menu) map[string]any {
	if menu.Binary && menu.FoundAnswer {
		return map[string]any{
			"type": "object",
			"properties": orderedProps{
				{"note", map[string]any{"type": "string"}},
				{"found", map[string]any{"type": "string"}},
			},
			"required":             []string{"note", "found"},
			"additionalProperties": false,
		}
	}
	if menu.Binary {
		// A receptor: the note first, then the one decision. The first
		// receptor set listed "no" first, which biased every answer to no;
		// probes set YesFirst on half their calls and average.
		answers := []string{"no", "yes"}
		if menu.YesFirst {
			answers = []string{"yes", "no"}
		}
		props := orderedProps{{"note", map[string]any{"type": "string"}}}
		if menu.Direct {
			props = nil
		}
		props = append(props, prop{"answer", map[string]any{"type": "string", "enum": answers}})
		required := make([]string, len(props))
		for i, p := range props {
			required[i] = p.Name
		}
		return map[string]any{
			"type":                 "object",
			"properties":           props,
			"required":             required,
			"additionalProperties": false,
		}
	}
	if menu.NoneOption {
		// One field decides: the signal, with "none" as an equal choice.
		signals := append(append([]string(nil), menu.Emoji...), NoneChoice)
		target := map[string]any{"type": "string"}
		if len(menu.Targets) > 0 {
			target["enum"] = menu.Targets
		}
		return map[string]any{
			"type": "object",
			"properties": orderedProps{
				{"note", map[string]any{"type": "string"}},
				{"target", target},
				{menu.signalField(), map[string]any{"type": "string", "enum": signals}},
			},
			"required":             []string{"note", "target", menu.signalField()},
			"additionalProperties": false,
		}
	}
	tools := make([]string, len(menu.Tools))
	for i, t := range menu.Tools {
		tools[i] = string(t)
	}
	// "note" comes first: the model states what the code needs BEFORE it
	// commits to a signal. With the emoji first, small models pick by
	// position in the enum and justify afterwards.
	var props orderedProps
	if len(menu.Outcomes) > 0 || len(menu.Emoji) > 0 {
		props = append(props, prop{"note", map[string]any{"type": "string"}})
	}
	props = append(props, prop{"tool", map[string]any{"type": "string", "enum": tools}})
	if len(menu.Emoji) > 0 {
		emoji := append([]string(nil), menu.Emoji...)
		targets := append([]string(nil), menu.Targets...)
		if len(menu.Outcomes) > 0 {
			emoji = append(emoji, "") // finish leaves it empty
			if len(targets) > 0 {
				targets = append(targets, "")
			}
		}
		target := map[string]any{"type": "string"}
		if len(targets) > 0 {
			target["enum"] = targets
		}
		props = append(props,
			prop{"target", target},
			prop{menu.signalField(), map[string]any{"type": "string", "enum": emoji}})
	}
	if len(menu.Outcomes) > 0 {
		outcomes := make([]string, 0, len(menu.Outcomes)+1)
		for _, o := range menu.Outcomes {
			outcomes = append(outcomes, string(o))
		}
		if len(menu.Emoji) > 0 {
			outcomes = append(outcomes, "") // deposit leaves it empty
		}
		props = append(props, prop{"outcome", map[string]any{"type": "string", "enum": outcomes}})
	}
	required := make([]string, len(props))
	for i, p := range props {
		required[i] = p.Name
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

// prop is one schema property; orderedProps marshals as a JSON object
// whose keys keep their slice order.
type prop struct {
	Name   string
	Schema any
}

type orderedProps []prop

// MarshalJSON makes orderedProps satisfy json.Marshaler.
// JS analogy: a toJSON() method, called by JSON.stringify.
func (o orderedProps) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(p.Name)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(p.Schema)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// toolSchemas returns one schema per tool, for providers with native tool
// calling (Anthropic). Only deposit and finish exist at level 0.
func toolSchemas(menu Menu) []map[string]any {
	if menu.NoneOption {
		return []map[string]any{{
			"name":         "choose",
			"description":  "Write a one-sentence note, then choose one signal, or none.",
			"input_schema": ActionSchema(menu),
		}}
	}
	if menu.Binary {
		return []map[string]any{{
			"name":         "answer",
			"description":  "Answer the question with a one-sentence note, then yes or no.",
			"input_schema": ActionSchema(menu),
		}}
	}
	var out []map[string]any
	for _, t := range menu.Tools {
		switch t {
		case core.ToolDeposit:
			out = append(out, map[string]any{
				"name":        string(t),
				"description": "Leave one signal on a target.",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"target":           targetSchema(menu),
						menu.signalField(): map[string]any{"type": "string", "enum": menu.Emoji},
					},
					"required":             []string{"target", menu.signalField()},
					"additionalProperties": false,
				},
			})
		case core.ToolFinish:
			outcomes := make([]string, len(menu.Outcomes))
			for i, o := range menu.Outcomes {
				outcomes[i] = string(o)
			}
			out = append(out, map[string]any{
				"name":        string(t),
				"description": "End the session.",
				"input_schema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"outcome": map[string]any{"type": "string", "enum": outcomes},
						"note":    map[string]any{"type": "string"},
					},
					"required":             []string{"outcome"},
					"additionalProperties": false,
				},
			})
		}
	}
	return out
}

func targetSchema(menu Menu) map[string]any {
	t := map[string]any{"type": "string", "description": "path or path#symbol"}
	if len(menu.Targets) > 0 {
		t["enum"] = menu.Targets
	}
	return t
}

func (m Menu) signalField() string {
	if m.SignalField == "" {
		return "emoji"
	}
	return m.SignalField
}

// Field is one property of an object schema built by ObjectSchema.
type Field struct {
	Name   string
	Schema any
}

// ObjectSchema returns a JSON Schema object whose properties keep the given
// order and are all required (rules 2 and 3: every field required, in a
// fixed order, the note first). Writer sessions build their reply schema
// with it.
func ObjectSchema(fields ...Field) map[string]any {
	props := make(orderedProps, len(fields))
	req := make([]string, len(fields))
	for i, f := range fields {
		props[i] = prop{f.Name, f.Schema}
		req[i] = f.Name
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             req,
		"additionalProperties": false,
	}
}

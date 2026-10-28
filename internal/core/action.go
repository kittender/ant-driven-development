package core

// Outcome is how a session ended.
type Outcome string

const (
	OutcomeDone      Outcome = "done"
	OutcomeCannot    Outcome = "cannot"
	OutcomeMalformed Outcome = "malformed"
	OutcomeLimit     Outcome = "limit"
	OutcomeKilled    Outcome = "killed"
)

// Action is one parsed model action. It is a flat struct rather than one
// type per tool: small models produce flat JSON more reliably, and the
// Tool Gateway checks which fields a tool needs.
//
// JS analogy: a discriminated union { tool: "deposit", target, emoji } |
// { tool: "finish", outcome, note }, with `tool` as the discriminant.
type Action struct {
	Tool    Tool    `json:"tool"`
	Target  string  `json:"target,omitempty"`
	Emoji   string  `json:"emoji,omitempty"`
	Signal  string  `json:"signal,omitempty"` // alias menu: the signal's name; the parser moves it to Emoji
	Outcome Outcome `json:"outcome,omitempty"`
	Note    string  `json:"note,omitempty"`
	// Answer is a receptor's reply, "yes" or "no". The parser turns "yes"
	// into a deposit of the receptor's one signal and "no" into finish.
	Answer string `json:"answer,omitempty"`
	// Found is a found-answer receptor's reply: the thing it was asked to
	// name, or empty. Non-empty becomes a deposit; empty becomes finish.
	Found string `json:"found,omitempty"`
}

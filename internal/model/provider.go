// Package model is the Model Gateway: one interface for every model,
// local or hosted, that returns exactly one action or a parse failure.
package model

import (
	"context"
	"encoding/json"

	"kittender/add/internal/core"
)

// Message is one turn of the conversation sent to a model.
type Message struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// Menu is the closed set of actions offered to this session.
// Providers use it to constrain output (JSON Schema, tool definitions).
type Menu struct {
	Tools []core.Tool
	Emoji []string // deposits allowed by the genome: emoji, or aliases in an alias menu
	// SignalField names the JSON field that carries the signal: "emoji"
	// (default) or "signal" for an alias menu, so the model never sees a
	// field called emoji filled with words.
	SignalField string
	Targets     []string       // targets the ageant may name, offered as a closed choice
	Outcomes    []core.Outcome // outcomes allowed for finish
	// Binary marks a receptor: the model only answers yes or no about one
	// signal on one target. "yes" becomes a deposit of Emoji[0] on
	// Targets[0]; "no" becomes finish done.
	Binary bool
	// FoundAnswer (with Binary): the reply is {note, found}; a non-empty
	// found is the yes. There is no yes/no token whose order could bias it.
	FoundAnswer bool
	// NoneOption: "none" is one more value of the signal field; choosing it
	// ends the session without a deposit. There is no separate tool field.
	NoneOption bool
	// YesFirst (with Binary): the answer enum lists "yes" before "no".
	// Probes ask each question in both orders and average, so the order of
	// the options cannot decide (findings, rule 6).
	YesFirst bool
	// Direct (with Binary): the reply is {answer} alone, no note. Used by
	// probes, where the decision is read from token probabilities.
	Direct bool
}

// NoneChoice is the signal value that means "this code needs nothing".
const NoneChoice = "none"

// Request is everything a provider needs for one completion.
// System holds the stable part of the context (genome, dictionary, honey,
// garden, terrain) so provider and local caches can reuse it.
type Request struct {
	Model       string
	System      string
	Messages    []Message
	Menu        Menu
	Temperature float64
	Seed        int64
	MaxTokens   int
	// TopLogprobs > 0 asks the provider for token log probabilities, with
	// that many alternatives per token. Only probes use it.
	TopLogprobs int
	// Format, when set, is the JSON Schema the reply must follow, used
	// instead of the action schema built from Menu. Writer sessions (testers,
	// workers) use it: their reply carries a whole file, not a signal.
	Format any
	// ContextTokens, when > 0, sets the model's context window. Ollama's
	// default is small and silently drops the start of a long prompt, which
	// is where the stable part (genome, honey, contract) sits.
	ContextTokens int
}

// Usage counts tokens for one completion.
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	CachedTokens int `json:"cached_tokens"`
}

// Add returns the sum of two usages.
func (u Usage) Add(o Usage) Usage {
	return Usage{u.InputTokens + o.InputTokens, u.OutputTokens + o.OutputTokens, u.CachedTokens + o.CachedTokens}
}

// Completion is a provider's raw answer.
// Structured is set when the provider returned a tool call or schema-bound
// JSON; Text is always the human-readable form for the log.
type Completion struct {
	Text       string
	Structured json.RawMessage
	Usage      Usage
	Logprobs   []TokenLogprob // set when the request asked for them
}

// TokenLogprob is one generated token, its log probability, and the most
// likely alternatives at that position (Ollama "logprobs" response).
type TokenLogprob struct {
	Token   string       `json:"token"`
	Logprob float64      `json:"logprob"`
	Top     []TopLogprob `json:"top_logprobs"`
}

// TopLogprob is one alternative token at a position.
type TopLogprob struct {
	Token   string  `json:"token"`
	Logprob float64 `json:"logprob"`
}

// Provider is implemented by each backend (Ollama, Anthropic, fake).
//
// JS analogy: a TypeScript interface. Go interfaces are satisfied
// implicitly: any type with these two methods IS a Provider, no "implements".
type Provider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Completion, error)
}

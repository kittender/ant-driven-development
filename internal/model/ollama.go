package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// Ollama talks to a local Ollama server over HTTP (/api/chat).
type Ollama struct {
	BaseURL string       // e.g. http://localhost:11434
	HTTP    *http.Client // nil means http.DefaultClient
	// Schema constrains output with the action JSON Schema ("format").
	// Turn it off to measure how well a model does with the line format alone.
	Schema bool
	// Think lets thinking models (qwen3, deepseek-r1...) reason before
	// answering. Off by default: an ageant's task is one small choice, and
	// thinking can spend the whole output budget before any answer appears.
	Think bool

	noThink sync.Map // models that reject the "think" field: model name -> true
}

func (o *Ollama) Name() string { return "ollama" }

type ollamaChatRequest struct {
	Model    string         `json:"model"`
	Messages []Message      `json:"messages"`
	Stream   bool           `json:"stream"`
	Format   any            `json:"format,omitempty"`
	Think    *bool          `json:"think,omitempty"`
	Options  map[string]any `json:"options"`
	// Logprobs asks for token log probabilities (Ollama 0.12.11 and later).
	Logprobs    bool `json:"logprobs,omitempty"`
	TopLogprobs int  `json:"top_logprobs,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
		// Thinking holds the reasoning of thinking models, kept apart from
		// Content. It is never parsed as an action.
		Thinking string `json:"thinking"`
	} `json:"message"`
	Logprobs        []TokenLogprob `json:"logprobs"`
	PromptEvalCount int            `json:"prompt_eval_count"`
	EvalCount       int            `json:"eval_count"`
	Error           string         `json:"error"`
}

func (o *Ollama) Complete(ctx context.Context, req Request) (Completion, error) {
	// The system message goes first and stays byte-identical across sessions
	// that share a genome and target, so Ollama can reuse its prompt cache.
	msgs := append([]Message{{Role: "system", Content: req.System}}, req.Messages...)
	body := ollamaChatRequest{
		Model:    req.Model,
		Messages: msgs,
		Stream:   false,
		Options: map[string]any{
			"temperature": req.Temperature,
			"seed":        req.Seed,
			"num_predict": req.MaxTokens,
		},
	}
	if req.Format != nil {
		body.Format = req.Format
	} else if o.Schema {
		body.Format = ActionSchema(req.Menu)
	}
	if req.ContextTokens > 0 {
		body.Options["num_ctx"] = req.ContextTokens
	}
	if req.TopLogprobs > 0 {
		body.Logprobs, body.TopLogprobs = true, req.TopLogprobs
	}
	if _, rejected := o.noThink.Load(req.Model); !rejected {
		think := o.Think
		body.Think = &think
	}
	c, err := o.do(ctx, body)
	if err != nil && body.Think != nil && strings.Contains(err.Error(), "does not support thinking") {
		// Non-thinking models refuse the field: remember it and retry without it.
		o.noThink.Store(req.Model, true)
		body.Think = nil
		return o.do(ctx, body)
	}
	return c, err
}

func (o *Ollama) do(ctx context.Context, body ollamaChatRequest) (Completion, error) {
	buf, err := json.Marshal(body)
	if err != nil {
		return Completion{}, err
	}

	url := strings.TrimRight(o.BaseURL, "/") + "/api/chat"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(buf))
	if err != nil {
		return Completion{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := o.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Completion{}, fmt.Errorf("ollama: %w", err)
	}
	defer resp.Body.Close() // JS analogy: a `finally` that always runs when the function returns

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Completion{}, fmt.Errorf("ollama: reading body: %w", err)
	}
	var out ollamaChatResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Completion{}, fmt.Errorf("ollama: status %d, body %.200q", resp.StatusCode, raw)
	}
	if resp.StatusCode != http.StatusOK || out.Error != "" {
		return Completion{}, fmt.Errorf("ollama: status %d: %s", resp.StatusCode, out.Error)
	}
	if body.Logprobs && len(out.Logprobs) == 0 {
		return Completion{}, fmt.Errorf("ollama: no logprobs in the reply; probes need Ollama 0.12.11 or later (ollama --version)")
	}
	return Completion{
		Text:     out.Message.Content,
		Usage:    Usage{InputTokens: out.PromptEvalCount, OutputTokens: out.EvalCount},
		Logprobs: out.Logprobs,
	}, nil
}

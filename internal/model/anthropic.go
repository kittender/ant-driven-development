package model

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Anthropic calls the Messages API with native tool use.
// Used for hosted models (Queen, gardeners) and as a reference point in evals.
type Anthropic struct {
	APIKey  string
	BaseURL string // default https://api.anthropic.com
	HTTP    *http.Client
}

func (a *Anthropic) Name() string { return "anthropic" }

type anthropicContent struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type anthropicResponse struct {
	Content []anthropicContent `json:"content"`
	Usage   struct {
		InputTokens          int `json:"input_tokens"`
		OutputTokens         int `json:"output_tokens"`
		CacheReadInputTokens int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

func (a *Anthropic) Complete(ctx context.Context, req Request) (Completion, error) {
	if a.APIKey == "" {
		return Completion{}, fmt.Errorf("anthropic: no API key (set ANTHROPIC_API_KEY)")
	}
	// Earlier actions and tool results travel as plain text turns, so the
	// Runner stays provider-neutral. The model still has to answer with a tool.
	body := map[string]any{
		"model":      req.Model,
		"max_tokens": req.MaxTokens,
		// The stable context is one cached system block: the cache boundary
		// of the architecture's context assembly.
		"system": []map[string]any{{
			"type":          "text",
			"text":          req.System,
			"cache_control": map[string]string{"type": "ephemeral"},
		}},
		"messages":    req.Messages,
		"tools":       toolSchemas(req.Menu),
		"tool_choice": map[string]string{"type": "any"},
		"temperature": req.Temperature,
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Completion{}, err
	}
	base := a.BaseURL
	if base == "" {
		base = "https://api.anthropic.com"
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(base, "/")+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return Completion{}, err
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", a.APIKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	client := a.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Completion{}, fmt.Errorf("anthropic: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Completion{}, fmt.Errorf("anthropic: reading body: %w", err)
	}
	var out anthropicResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return Completion{}, fmt.Errorf("anthropic: status %d, body %.200q", resp.StatusCode, raw)
	}
	if out.Error != nil {
		return Completion{}, fmt.Errorf("anthropic: %s: %s", out.Error.Type, out.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return Completion{}, fmt.Errorf("anthropic: status %d", resp.StatusCode)
	}

	c := Completion{Usage: Usage{
		InputTokens:  out.Usage.InputTokens,
		OutputTokens: out.Usage.OutputTokens,
		CachedTokens: out.Usage.CacheReadInputTokens,
	}}
	for _, block := range out.Content {
		switch block.Type {
		case "text":
			c.Text += block.Text
		case "tool_use":
			// Merge {"tool": name} into the tool input to get one flat Action.
			var input map[string]any
			if err := json.Unmarshal(block.Input, &input); err != nil {
				return c, fmt.Errorf("anthropic: tool input: %w", err)
			}
			input["tool"] = block.Name
			c.Structured, _ = json.Marshal(input)
			c.Text = strings.TrimSpace(c.Text + "\n" + string(c.Structured))
			return c, nil // first tool call wins: one action per step
		}
	}
	return c, nil // no tool call: the gateway will try to parse the text
}

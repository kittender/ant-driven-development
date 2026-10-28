package model

import (
	"context"
	"fmt"
	"sync"
)

// Fake is a scripted provider for tests and dry runs.
// Reply decides the text for each call; calls are counted per model.
type Fake struct {
	Reply func(req Request, call int) (string, error)
	// Logprobs, if set, supplies token log probabilities for a reply.
	Logprobs func(req Request, text string) []TokenLogprob

	mu    sync.Mutex
	calls map[string]int
}

func (f *Fake) Name() string { return "fake" }

func (f *Fake) Complete(_ context.Context, req Request) (Completion, error) {
	f.mu.Lock() // the eval runs sessions in parallel goroutines
	if f.calls == nil {
		f.calls = map[string]int{}
	}
	n := f.calls[req.Model]
	f.calls[req.Model] = n + 1
	f.mu.Unlock()

	if f.Reply == nil {
		return Completion{}, fmt.Errorf("fake: no Reply function")
	}
	text, err := f.Reply(req, n)
	c := Completion{Text: text, Usage: Usage{InputTokens: len(req.System) / 4, OutputTokens: len(text) / 4}}
	if f.Logprobs != nil && req.TopLogprobs > 0 {
		c.Logprobs = f.Logprobs(req, text)
	}
	return c, err
}

// FinishFake always answers "finish done". Handy to check an install
// without any model: go run ./cmd/add-eval -models fake:finish
func FinishFake() *Fake {
	return &Fake{Reply: func(Request, int) (string, error) { return "finish done nothing to mark", nil }}
}

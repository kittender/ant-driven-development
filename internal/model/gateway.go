package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"kittender/add/internal/core"
)

// Gateway routes "provider:model" names to providers and parses replies.
type Gateway struct {
	providers map[string]Provider
	now       func() time.Time
}

// NewGateway registers providers by their Name().
func NewGateway(ps ...Provider) *Gateway {
	g := &Gateway{providers: map[string]Provider{}, now: time.Now}
	for _, p := range ps {
		g.providers[p.Name()] = p
	}
	return g
}

// SplitModel turns "ollama:qwen3:4b" into ("ollama", "qwen3:4b").
func SplitModel(spec string) (provider, model string, err error) {
	provider, model, ok := strings.Cut(spec, ":")
	if !ok || provider == "" || model == "" {
		return "", "", fmt.Errorf("model %q must look like provider:model", spec)
	}
	return provider, model, nil
}

// Result is one call through the gateway.
// Exactly one of Action, ParseErr, Err describes what happened.
type Result struct {
	Action   *core.Action
	ParseErr error // the model answered, but not with a valid action
	Err      error // the model could not be reached
	Raw      string
	Usage    Usage
	Latency  time.Duration
}

// Act sends one request and returns one action or a parse failure.
// req.Model is the full "provider:model" spec.
func (g *Gateway) Act(ctx context.Context, req Request) Result {
	pname, mname, err := SplitModel(req.Model)
	if err != nil {
		return Result{Err: err}
	}
	p, ok := g.providers[pname]
	if !ok {
		return Result{Err: fmt.Errorf("no provider %q", pname)}
	}
	req.Model = mname

	start := g.now()
	c, err := p.Complete(ctx, req)
	res := Result{Raw: c.Text, Usage: c.Usage, Latency: g.now().Sub(start)}
	if err != nil {
		res.Err = err
		return res
	}

	var a core.Action
	if len(c.Structured) > 0 {
		a, err = ParseStructured(c.Structured, req.Menu)
	} else {
		a, err = ParseText(c.Text, req.Menu)
	}
	var pe *ParseError
	switch {
	case err == nil:
		res.Action = &a
	case errors.As(err, &pe):
		res.ParseErr = err
	default:
		res.Err = err
	}
	return res
}

// Complete sends one request and returns the raw completion, unparsed.
// Probes use it: their decision is read from token probabilities, not
// from the parsed action. req.Model is the full "provider:model" spec.
func (g *Gateway) Complete(ctx context.Context, req Request) (Completion, time.Duration, error) {
	pname, mname, err := SplitModel(req.Model)
	if err != nil {
		return Completion{}, 0, err
	}
	p, ok := g.providers[pname]
	if !ok {
		return Completion{}, 0, fmt.Errorf("no provider %q", pname)
	}
	req.Model = mname
	start := g.now()
	c, err := p.Complete(ctx, req)
	return c, g.now().Sub(start), err
}

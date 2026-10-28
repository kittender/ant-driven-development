// Package runner hosts one ageant session: context in, one action out,
// through the Tool Gateway, repeat until finish or a limit.
// It keeps no state after the session ends.
package runner

import (
	"context"
	"time"

	"kittender/add/internal/core"
	"kittender/add/internal/model"
	"kittender/add/internal/tools"
)

// Session is one ageant to run.
type Session struct {
	ID     string
	Genome core.Genome // Model must be a full "provider:model" spec
	Bundle Bundle
}

// Step is one model turn, recorded for the colony log.
type Step struct {
	N          int           `json:"n"`
	Raw        string        `json:"raw"`
	Action     *core.Action  `json:"action,omitempty"`
	ParseError string        `json:"parse_error,omitempty"`
	Result     string        `json:"result,omitempty"`
	Refusal    string        `json:"refusal,omitempty"`
	Usage      model.Usage   `json:"usage"`
	Latency    time.Duration `json:"latency_ns"`
}

// Record is the full trace of one session.
type Record struct {
	SessionID string          `json:"session_id"`
	GenomeID  string          `json:"genome_id"`
	Model     string          `json:"model"`
	Seed      int64           `json:"seed"`
	Target    string          `json:"target"`
	System    string          `json:"system"`
	User      string          `json:"user"`
	Steps     []Step          `json:"steps"`
	Outcome   core.Outcome    `json:"outcome,omitempty"`
	Error     string          `json:"error,omitempty"` // model unreachable: no outcome
	Deposits  []tools.Deposit `json:"deposits"`
	Refusals  int             `json:"refusals"`
	Malformed int             `json:"malformed"`
	Usage     model.Usage     `json:"usage"`
	Duration  time.Duration   `json:"duration_ns"`
	Note      string          `json:"note,omitempty"` // finish note: colony log only
}

// Runner runs sessions. It is safe for concurrent use.
type Runner struct {
	Models *model.Gateway
	Dict   *core.Dictionary
	Menu   MenuStyle // "" means MenuEmoji
}

// Run executes one session to the end and returns its record.
// rec is a named result so the deferred duration update reaches the caller.
func (r *Runner) Run(ctx context.Context, s Session) (rec Record) {
	g := s.Genome
	style := r.Menu
	if style == "" {
		style = MenuEmoji
	}
	system, user, menu := Assemble(g, r.Dict, s.Bundle, style)
	rec = Record{
		SessionID: s.ID, GenomeID: g.ID, Model: g.Model, Seed: g.Seed,
		Target: s.Bundle.Target.String(), System: system, User: user,
		Deposits: []tools.Deposit{},
	}
	start := time.Now()
	defer func() { rec.Duration = time.Since(start) }()

	// The time limit is a context deadline.
	// JS analogy: an AbortController whose signal fires after N seconds.
	ctx, cancel := context.WithTimeout(ctx, time.Duration(g.Limits.Seconds)*time.Second)
	defer cancel()

	gw := tools.NewSession(s.ID, g, s.Bundle.Target, r.Dict)
	msgs := []model.Message{{Role: "user", Content: user}}

	for n := 1; n <= g.Limits.Steps; n++ {
		res := r.Models.Act(ctx, model.Request{
			Model: g.Model, System: system, Messages: msgs, Menu: menu,
			Temperature: g.Temperature, Seed: g.Seed, MaxTokens: g.Limits.OutputTokens,
		})
		step := Step{N: n, Raw: res.Raw, Usage: res.Usage, Latency: res.Latency}
		rec.Usage = rec.Usage.Add(res.Usage)

		if res.Err != nil {
			step.ParseError = res.Err.Error()
			rec.Steps = append(rec.Steps, step)
			if ctx.Err() != nil {
				rec.Outcome = core.OutcomeLimit // time limit reached
			} else {
				rec.Error = res.Err.Error()
			}
			return rec
		}

		msgs = append(msgs, model.Message{Role: "assistant", Content: nonEmpty(res.Raw)})

		if res.ParseErr != nil {
			rec.Malformed++
			step.ParseError = res.ParseErr.Error()
			rec.Steps = append(rec.Steps, step)
			if rec.Malformed >= g.Limits.Malformed {
				rec.Outcome = core.OutcomeMalformed
				return rec
			}
			msgs = append(msgs, model.Message{Role: "user", Content: "That was not one valid action (" +
				res.ParseErr.Error() + "). Reply with exactly one action."})
			continue
		}

		step.Action = res.Action
		out := gw.Do(*res.Action)
		step.Result, step.Refusal = out.Message, out.Refusal
		rec.Steps = append(rec.Steps, step)
		if out.Refusal != "" {
			rec.Refusals++
		}
		if out.Deposit != nil {
			rec.Deposits = append(rec.Deposits, *out.Deposit)
			if max := g.Limits.Deposits; max > 0 && len(rec.Deposits) >= max {
				// The genome's choice is made: the session ends as done.
				rec.Outcome = core.OutcomeDone
				rec.Note = res.Action.Note
				return rec
			}
		}
		if out.Finished {
			rec.Outcome = out.Outcome
			rec.Note = res.Action.Note
			return rec
		}
		msgs = append(msgs, model.Message{Role: "user", Content: out.Message + "."})
	}
	rec.Outcome = core.OutcomeLimit
	return rec
}

func nonEmpty(s string) string {
	if s == "" {
		return "(empty reply)"
	}
	return s
}

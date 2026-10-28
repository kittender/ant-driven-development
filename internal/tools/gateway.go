// Package tools is the Tool Gateway: the only door between an ageant and
// everything else. Every authority comes from the genome and is checked
// here, on every call. A rule that only exists in a prompt is not enforcement.
//
// Level 0 implements deposit and finish. Other tools are refused.
package tools

import (
	"fmt"
	"slices"
	"strings"

	"kittender/add/internal/core"
)

// Deposit is one accepted request signal. The actor is stamped by the
// gateway, never taken from the model.
type Deposit struct {
	Actor      string `json:"actor"`
	Target     string `json:"target"`
	Emoji      string `json:"emoji"`
	Normalized bool   `json:"normalized,omitempty"` // a trailing U+FE0F was removed
}

// Result tells the Runner what happened and what to show the model.
type Result struct {
	Message  string       // text returned to the model
	Refusal  string       // set when the call was refused (still counts as a step)
	Deposit  *Deposit     // set when a signal was accepted
	Ignored  bool         // duplicate deposit: no effect, not a refusal
	Finished bool         // the session ends
	Outcome  core.Outcome // set when Finished
}

// Session is the gateway's view of one live ageant.
type Session struct {
	Ageant string
	Genome core.Genome
	Scope  core.Target // the target the ageant was woken for
	Dict   *core.Dictionary

	seen map[string]bool // target+emoji already deposited by this ageant
}

// NewSession builds a gateway session for one ageant.
func NewSession(ageant string, g core.Genome, scope core.Target, d *core.Dictionary) *Session {
	return &Session{Ageant: ageant, Genome: g, Scope: scope, Dict: d, seen: map[string]bool{}}
}

// Do checks and performs one action.
func (s *Session) Do(a core.Action) Result {
	// Two gates: the caste table, then the genome's own subset.
	if !slices.Contains(core.CasteTools[s.Genome.Caste], a.Tool) || !s.Genome.Has(a.Tool) {
		return refuse("tool %q is not available to you", a.Tool)
	}
	switch a.Tool {
	case core.ToolDeposit:
		return s.deposit(a)
	case core.ToolFinish:
		return Result{Message: "session ended", Finished: true, Outcome: a.Outcome}
	default:
		return refuse("tool %q is not implemented at level 0", a.Tool)
	}
}

func (s *Session) deposit(a core.Action) Result {
	t, err := core.ParseTarget(a.Target)
	if err != nil {
		return refuse("bad target: %v", err)
	}
	if !t.Within(s.Scope) {
		return refuse("target %s is outside your scope %s", t, s.Scope)
	}
	emoji, normalized := core.NormalizeEmoji(a.Emoji)
	entry, ok := s.Dict.Lookup(emoji)
	if !ok {
		// Alias menu: the ageant names the signal; the trail still stores the emoji.
		if entry, ok = s.Dict.ByAlias(strings.ToLower(emoji)); ok {
			emoji = entry.Emoji
		}
	}
	if !ok {
		return refuse("%q is not a signal in the dictionary", a.Emoji)
	}
	if entry.Class == core.Verdict {
		// Recorded against the ageant: only the Ecosystem or a human issues verdicts.
		return refuse("%s is a verdict; ageants cannot issue verdicts", emoji)
	}
	if !slices.Contains(s.Genome.MayDeposit, emoji) {
		return refuse("you may not deposit %s", emoji)
	}
	key := t.String() + " " + emoji
	if s.seen[key] {
		// Strength counts distinct ageants, so a repeat is ignored, not refused.
		return Result{Message: fmt.Sprintf("already deposited %s on %s; ignored. Finish now, or deposit a different signal", emoji, t), Ignored: true}
	}
	s.seen[key] = true
	return Result{
		Message: fmt.Sprintf("deposited %s on %s. Finish now, or deposit one different signal if the code clearly needs it", emoji, t),
		Deposit: &Deposit{Actor: s.Ageant, Target: t.String(), Emoji: emoji, Normalized: normalized},
	}
}

func refuse(format string, a ...any) Result {
	msg := fmt.Sprintf(format, a...)
	return Result{Message: "refused: " + msg, Refusal: msg}
}

package eval

import (
	"slices"

	"kittender/add/internal/core"
	"kittender/add/internal/runner"
)

// Score is the verdict on one session.
type Score struct {
	Valid   bool `json:"valid"`   // ended with finish, no refusal, no malformed reply
	Fitting bool `json:"fitting"` // the signals left are the right ones
	Errored bool `json:"errored"` // model unreachable: excluded from rates
}

// ScoreRecord compares a session with its fixture.
//
//   - valid: the session ended by finish, every reply parsed, nothing was refused.
//   - fitting: at least one deposit and all deposits are in Accept,
//     or no deposit and Accept contains "none".
//
// A session can be fitting without being valid (right signal, then a
// malformed reply): the report shows both so the two failure kinds stay apart.
func ScoreRecord(f Fixture, rec runner.Record) Score {
	if rec.Error != "" {
		return Score{Errored: true}
	}
	finished := rec.Outcome == core.OutcomeDone || rec.Outcome == core.OutcomeCannot
	s := Score{Valid: finished && rec.Malformed == 0 && rec.Refusals == 0}

	if len(rec.Deposits) == 0 {
		s.Fitting = finished && slices.Contains(f.Accept, None)
		return s
	}
	s.Fitting = true
	for _, d := range rec.Deposits {
		if !slices.Contains(f.Accept, d.Emoji) {
			s.Fitting = false
		}
	}
	return s
}

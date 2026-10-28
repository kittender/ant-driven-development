package pilot

import (
	"strings"

	"kittender/add/internal/instruments"
)

// Reflex 5 (outline, run 3): one fixed hint per kind of compiler error, so a
// small model reads what the terse message means. Written by hand, never by
// a model; shown once per kind, in this order.
var compilerHints = []struct {
	ID    string
	Match func(f instruments.Finding) bool
	Text  string
}{
	{"readonly", func(f instruments.Finding) bool {
		return f.Rule == "TS2540" || f.Rule == "TS2542" || (f.Rule == "TS2322" && strings.Contains(f.Message, "readonly")) || f.Rule == "TS4104"
	}, "The contract's types are readonly: never assign to a property or an array element. Build a new value instead: `{ ...state, turn: 'O' }`, `board.map((row, r) => row.map((cell, c) => (r === i && c === j ? player : cell)))`."},
	{"literal", func(f instruments.Finding) bool {
		return f.Rule == "TS2322" && strings.Contains(f.Message, "Type 'string' is not assignable")
	}, "A plain `string` is not one of the contract's literal types (Status, Player, Rejection). Give the variable the contract's type (`let status: Status = 'in progress'`) or write the literal directly in the returned object."},
	{"order", func(f instruments.Finding) bool {
		return f.Rule == "TS2448" || f.Rule == "TS2454"
	}, "A name is used before its declaration. Declare helpers with `function name(...) { ... }` (hoisted) or above the code that uses them."},
	{"any", func(f instruments.Finding) bool {
		return f.Rule == "TS7006" || f.Rule == "TS7022" || f.Rule == "TS7031" || f.Rule == "TS7023" || f.Rule == "TS7024"
	}, "An implicit `any`: annotate the parameter or variable with a contract type (Player, Cell, GameState, Match, Score) and give recursive helpers a return type."},
}

// Hints returns the hints for these compiler findings and their ids.
func Hints(fs []instruments.Finding) (text string, ids []string) {
	var b strings.Builder
	for _, h := range compilerHints {
		for _, f := range fs {
			if h.Match(f) {
				b.WriteString("\nHint: " + h.Text)
				ids = append(ids, h.ID)
				break
			}
		}
	}
	return b.String(), ids
}

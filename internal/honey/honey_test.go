package honey

import (
	"strings"
	"testing"
)

const move = `Feature: Make a move
  As a player

  Background:
    Given a new game has started

  @capped
  Scenario: Cannot play on an occupied cell
    Given player "X" has played at row 1, column 1
    When player "O" plays at row 1, column 1
    Then the move should be rejected with "Cell already taken"
    And the board should be unchanged

  @capped
  Scenario Outline: Cannot play outside the board
    When player "X" plays at row <row>, column <col>
    Then the move should be rejected with "Invalid position"

    Examples:
      | row | col |
      | 0   | 1   |
      | 4   | 2   |
`

func snap(src string) Snapshot { return FromFiles(map[string]string{"02_make_move.feature": src}) }

func TestParse(t *testing.T) {
	s := snap(move)
	if len(s.Capped()) != 2 {
		t.Fatalf("capped = %d, want 2", len(s.Capped()))
	}
	o := s.Capped()[1]
	if !o.Outline || len(o.Examples) != 3 || o.Examples[1] != "| 0 | 1 |" {
		t.Fatalf("outline parsed wrong: %+v", o)
	}
	f, _ := s.Feature("02_make_move.feature")
	if len(f.Background) != 1 {
		t.Fatalf("background = %v", f.Background)
	}
	if !strings.Contains(o.Text(), "Scenario Outline: Cannot play outside the board") {
		t.Fatalf("Text() = %s", o.Text())
	}
}

func kinds(ev []Event) string {
	var k []string
	for _, e := range ev {
		k = append(k, string(e.Kind))
	}
	return strings.Join(k, ",")
}

func TestDiff(t *testing.T) {
	cases := []struct {
		name, new, want string
		weakens         bool
	}{
		{"unchanged", move, "", false},
		{"alignment only", strings.Replace(move, "| 0   | 1   |", "|0|1|", 1), "", false},
		{"then removed", strings.Replace(move, "    And the board should be unchanged\n", "", 1), "weakened", true},
		{"message changed", strings.Replace(move, `"Cell already taken"`, `"Taken"`, 1), "weakened", true},
		{"example removed", strings.Replace(move, "      | 4   | 2   |\n", "", 1), "weakened", true},
		{"then added", strings.Replace(move, "    And the board should be unchanged\n", "    And the board should be unchanged\n    And it should be player \"O\"'s turn\n", 1), "strengthened", false},
		{"given changed", strings.Replace(move, `When player "O" plays at row 1, column 1`, `When player "O" plays at row 1, column 2`, 1), "reworded", false},
		{"uncapped", strings.Replace(move, "  @capped\n  Scenario: Cannot", "  Scenario: Cannot", 1), "uncapped", true},
		{"renamed", strings.Replace(move, "Cannot play on an occupied cell", "Occupied cell", 1), "removed,added", true},
		{"background", strings.Replace(move, "a new game has started", "an empty board", 1), "background", true},
		{"new scenario", move + "\n  Scenario: Extra\n    Then x\n", "added", false},
		{"and becomes then", strings.Replace(move, "    And the board should be unchanged", "    Then the board should be unchanged", 1), "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ev := Diff(snap(move), snap(c.new))
			if got := kinds(ev); got != c.want {
				t.Fatalf("kinds = %q, want %q (%+v)", got, c.want, ev)
			}
			w := false
			for _, e := range ev {
				w = w || e.Weakens()
			}
			if w != c.weakens {
				t.Fatalf("weakens = %v, want %v", w, c.weakens)
			}
		})
	}
}

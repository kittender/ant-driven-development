package honey

import (
	"regexp"
	"strings"
)

// Kind of a change to honey.
type Kind string

const (
	Added        Kind = "added"        // a scenario that did not exist
	Removed      Kind = "removed"      // a scenario that no longer exists
	Weakened     Kind = "weakened"     // an outcome checked less: a Then step, an example row or an expected value removed
	Strengthened Kind = "strengthened" // more is checked, nothing checked before was removed
	Reworded     Kind = "reworded"     // Given or When steps changed, outcomes untouched
	Uncapped     Kind = "uncapped"     // the cap tag was removed
	Recapped     Kind = "capped"       // the cap tag was added
	Background   Kind = "background"   // a feature's Background changed
)

// Event is one change to one scenario (or one feature's background).
type Event struct {
	Kind      Kind   `json:"kind"`
	File      string `json:"file"`
	Scenario  string `json:"scenario,omitempty"`
	WasCapped bool   `json:"was_capped"`
	Detail    string `json:"detail"`
}

// Weakens reports whether the event lowers what capped honey checks.
func (e Event) Weakens() bool {
	if !e.WasCapped {
		return false
	}
	switch e.Kind {
	case Weakened, Removed, Uncapped, Background:
		return true
	}
	return false
}

// Diff lists the changes from old to new, in old file order, then additions.
func Diff(old, new Snapshot) []Event {
	var ev []Event
	for _, f := range old.Features {
		nf, ok := new.Feature(f.File)
		if ok && strings.Join(f.Background, "\n") != strings.Join(nf.Background, "\n") {
			ev = append(ev, Event{Kind: Background, File: f.File, WasCapped: anyCapped(f), Detail: "Background changed"})
		}
	}
	for _, k := range old.Order {
		o := old.Scenarios[k]
		n, ok := new.Scenarios[k]
		if !ok {
			ev = append(ev, Event{Kind: Removed, File: o.File, Scenario: o.Name, WasCapped: o.Capped(), Detail: "scenario removed or renamed"})
			continue
		}
		ev = append(ev, compare(o, n)...)
	}
	for _, k := range new.Order {
		if _, ok := old.Scenarios[k]; !ok {
			n := new.Scenarios[k]
			d := "new scenario"
			if n.Capped() {
				d = "new scenario tagged @capped by a non-human"
			}
			ev = append(ev, Event{Kind: Added, File: n.File, Scenario: n.Name, Detail: d})
		}
	}
	return ev
}

func anyCapped(f Feature) bool {
	for _, s := range f.Scenarios {
		if s.Capped() {
			return true
		}
	}
	return false
}

func compare(o, n Scenario) []Event {
	var ev []Event
	base := Event{File: o.File, Scenario: o.Name, WasCapped: o.Capped()}
	if o.Capped() && !n.Capped() {
		e := base
		e.Kind, e.Detail = Uncapped, "@capped removed"
		ev = append(ev, e)
	}
	if !o.Capped() && n.Capped() {
		e := base
		e.Kind, e.Detail = Recapped, "@capped added by a non-human"
		ev = append(ev, e)
	}

	oThen, oGiven := split(o.Steps)
	nThen, nGiven := split(n.Steps)
	lostThen := missing(oThen, nThen)
	gainedThen := missing(nThen, oThen)
	lostRows := missing(o.Examples, n.Examples)
	gainedRows := missing(n.Examples, o.Examples)

	switch {
	case len(lostThen) > 0 || len(lostRows) > 0:
		e := base
		e.Kind = Weakened
		var parts []string
		for _, s := range lostThen {
			parts = append(parts, "outcome no longer checked: "+s)
		}
		for _, r := range lostRows {
			parts = append(parts, "example removed: "+r)
		}
		if len(gainedThen) > 0 || len(gainedRows) > 0 {
			parts = append(parts, "replaced by: "+strings.Join(append(gainedThen, gainedRows...), " / "))
		}
		e.Detail = strings.Join(parts, "; ")
		ev = append(ev, e)
	case len(gainedThen) > 0 || len(gainedRows) > 0:
		e := base
		e.Kind, e.Detail = Strengthened, "now also checks: "+strings.Join(append(gainedThen, gainedRows...), " / ")
		ev = append(ev, e)
	}
	if strings.Join(oGiven, "\n") != strings.Join(nGiven, "\n") {
		e := base
		e.Kind, e.Detail = Reworded, "Given or When steps changed"
		ev = append(ev, e)
	}
	return ev
}

var keyword = regexp.MustCompile(`^(Given|When|Then|And|But|\*)\s+`)

// split separates outcome steps (Then and the And/But after it, with their
// tables) from setup and action steps (Given, When and their And/But).
func split(steps []string) (then, other []string) {
	inThen := false
	for _, s := range steps {
		if m := keyword.FindStringSubmatch(s); m != nil {
			switch m[1] {
			case "Then":
				inThen = true
			case "Given", "When":
				inThen = false
			}
		}
		// The keyword itself (And vs Then) is not a change of meaning.
		body := keyword.ReplaceAllString(s, "")
		if inThen {
			then = append(then, body)
		} else {
			other = append(other, body)
		}
	}
	return then, other
}

// missing returns the items of a not found in b, as a multiset.
func missing(a, b []string) []string {
	count := map[string]int{}
	for _, x := range b {
		count[x]++
	}
	var out []string
	for _, x := range a {
		if count[x] > 0 {
			count[x]--
			continue
		}
		out = append(out, x)
	}
	return out
}

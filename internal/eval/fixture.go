// Package eval runs the level 0 experiment: do small models pick a valid
// and fitting signal from a closed menu?
package eval

import (
	"encoding/json"
	"fmt"
	"io"

	"kittender/add/internal/core"
	"kittender/add/internal/runner"
)

// None in Accept means "finishing without any deposit is the right answer".
const None = "none"

// Fixture is one hand-labelled case.
type Fixture struct {
	ID    string `json:"id"`
	About string `json:"about"` // what the case tests, for the report
	// Instrument names the deterministic tool that could have raised the
	// right signal without a model (coverage, compiler, lint, path rule...).
	// Empty means only judgment can: the "model-only" cases.
	Instrument string `json:"instrument"`
	Target     string `json:"target"`
	Terrain    string `json:"terrain"`
	// Facts are what the Ecosystem would compute (files, tests and their
	// status, callers, git history, failing checks). They reach the model in
	// a separate, trusted FACTS block, never as comments in the code.
	Facts  []string `json:"facts"`
	Trail  string   `json:"trail"`
	Honey  []string `json:"honey"`
	Garden []string `json:"garden"`
	Task   string   `json:"task"`
	Accept []string `json:"accept"` // emoji that fit, or "none"
	// Zone holds the target's neighbours, for the zones experiment. It
	// reaches the model only through ZoneBundle (probes with -zone).
	Zone *runner.Zone `json:"zone,omitempty"`
}

// LoadFixtures reads a JSON array of fixtures and checks them against the
// dictionary, so a typo in a fixture fails loudly instead of skewing scores.
func LoadFixtures(r io.Reader, d *core.Dictionary) ([]Fixture, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var fs []Fixture
	if err := dec.Decode(&fs); err != nil {
		return nil, fmt.Errorf("fixtures: %w", err)
	}
	seen := map[string]bool{}
	for _, f := range fs {
		if f.ID == "" || seen[f.ID] {
			return nil, fmt.Errorf("fixture id %q is empty or duplicated", f.ID)
		}
		seen[f.ID] = true
		if _, err := core.ParseTarget(f.Target); err != nil {
			return nil, fmt.Errorf("fixture %s: %w", f.ID, err)
		}
		if len(f.Accept) == 0 {
			return nil, fmt.Errorf("fixture %s: accept is empty", f.ID)
		}
		for _, a := range f.Accept {
			if a == None {
				continue
			}
			if e, ok := d.Lookup(a); !ok || e.Class != core.Request {
				return nil, fmt.Errorf("fixture %s: accept %q is not a request signal", f.ID, a)
			}
		}
	}
	return fs, nil
}

// Bundle turns a fixture into the task bundle the Sensor would build.
func (f Fixture) Bundle() runner.Bundle {
	t, _ := core.ParseTarget(f.Target) // validated in LoadFixtures
	return runner.Bundle{
		Target: t, Honey: f.Honey, Garden: f.Garden,
		Terrain: f.Terrain, Facts: f.Facts, Task: f.Task, TrailLine: f.Trail,
	}
}

// ZoneBundle is Bundle plus the fixture's zone, if it has one.
func (f Fixture) ZoneBundle() runner.Bundle {
	b := f.Bundle()
	b.Zone = f.Zone
	return b
}

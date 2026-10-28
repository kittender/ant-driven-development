// Package pilot is the first loop on a real workspace (outline, "Level 0c
// and the first loop"). A script stands in for the parts of the Ecosystem
// that are not built yet: one lease at a time, threshold 1 per capped red
// scenario, the Prover as typecheck + lint + tests, a dead end after two
// failed attempts by one genome, one try by a second genome, no decay.
package pilot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Workspace is the project the colony works on. Paths are relative and use
// forward slashes; every path is checked to stay inside Dir.
type Workspace struct {
	Dir string
}

func (w Workspace) abs(rel string) (string, error) {
	if rel == "" || filepath.IsAbs(rel) || strings.Contains(rel, "..") {
		return "", fmt.Errorf("path %q is outside the workspace", rel)
	}
	return filepath.Join(w.Dir, filepath.FromSlash(rel)), nil
}

// Read returns a file's content, and false when it does not exist.
func (w Workspace) Read(rel string) (string, bool, error) {
	p, err := w.abs(rel)
	if err != nil {
		return "", false, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	return string(b), err == nil, err
}

// Write replaces a file's content, creating folders as needed.
func (w Workspace) Write(rel, content string) error {
	p, err := w.abs(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, []byte(content), 0o644)
}

// Remove deletes a file; a missing file is not an error.
func (w Workspace) Remove(rel string) error {
	p, err := w.abs(rel)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// Saved is the content of some files at one moment; nil means "did not exist".
type Saved map[string]*string

// Save records the given files so they can be restored after a rejected change.
func (w Workspace) Save(paths []string) (Saved, error) {
	s := Saved{}
	for _, p := range paths {
		c, ok, err := w.Read(p)
		if err != nil {
			return nil, err
		}
		if ok {
			s[p] = &c
		} else {
			s[p] = nil
		}
	}
	return s, nil
}

// Restore puts saved files back as they were, removing those that did not exist.
func (w Workspace) Restore(s Saved) error {
	keys := make([]string, 0, len(s))
	for k := range s {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if s[k] == nil {
			if err := w.Remove(k); err != nil {
				return err
			}
			continue
		}
		if err := w.Write(k, *s[k]); err != nil {
			return err
		}
	}
	return nil
}

// StepFiles lists tests/steps/*.steps.ts, sorted.
func (w Workspace) StepFiles() ([]string, error) {
	m, err := filepath.Glob(filepath.Join(w.Dir, "tests", "steps", "*.steps.ts"))
	if err != nil {
		return nil, err
	}
	out := make([]string, len(m))
	for i, p := range m {
		out[i] = "tests/steps/" + filepath.Base(p)
	}
	sort.Strings(out)
	return out, nil
}

// StepsIndex is the setup file Vitest loads; the harness writes it (a
// reflex, no model): one import per step file.
const StepsIndex = "tests/steps/index.ts"

// SyncStepsIndex rewrites tests/steps/index.ts to import every step file.
func (w Workspace) SyncStepsIndex() error {
	files, err := w.StepFiles()
	if err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("// Written by the harness (add-pilot): one import per step file. Do not edit.\n")
	if len(files) == 0 {
		b.WriteString("export {};\n")
	}
	for _, f := range files {
		b.WriteString("import './" + strings.TrimSuffix(filepath.Base(f), ".ts") + "';\n")
	}
	return w.Write(StepsIndex, b.String())
}

// StepFileFor returns the step file that belongs to a feature file.
func StepFileFor(feature string) string {
	return "tests/steps/" + strings.TrimSuffix(feature, ".feature") + ".steps.ts"
}

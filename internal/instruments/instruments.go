// Package instruments runs the developer's own tools on a workspace and
// turns their output into findings (candidate deposits) and facts (lines of
// evidence for the task bundle). Nothing here calls a model.
//
// Each parser is a pure function of the tool's output, so it is tested
// against recorded outputs; Run is the only part that touches the system.
package instruments

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"time"
)

// Finding is one thing an instrument noticed, with the request it maps to.
// Signal is empty when the finding is a fact only.
type Finding struct {
	Instrument string `json:"instrument"`
	File       string `json:"file"` // relative to the workspace
	Line       int    `json:"line,omitempty"`
	Rule       string `json:"rule,omitempty"`
	Severity   string `json:"severity,omitempty"` // error | warning
	Message    string `json:"message"`
	Signal     string `json:"signal,omitempty"`
}

// Fact is one line of evidence computed by the Ecosystem, with what it
// proves and what it does not (concept 0.4: facts are evidence, not verdicts).
type Fact struct {
	Target string `json:"target"`
	Kind   string `json:"kind"`
	Text   string `json:"text"`
	Proves string `json:"proves,omitempty"`
	Not    string `json:"not,omitempty"`
	Source string `json:"source"`
}

// Render is the line shown to a model in the FACTS block.
func (f Fact) Render() string {
	s := f.Text
	if f.Proves != "" || f.Not != "" {
		s += "  (proves: " + f.Proves + "; does not prove: " + f.Not + ")"
	}
	return s
}

// Output is what one command produced.
type Output struct {
	Command  []string      `json:"command"`
	Stdout   string        `json:"-"`
	Stderr   string        `json:"-"`
	Exit     int           `json:"exit"`
	Duration time.Duration `json:"duration"`
	Err      string        `json:"err,omitempty"` // could not start, timed out
}

// Runner runs one command in a directory. Tests replace it.
type Runner func(ctx context.Context, dir string, argv ...string) Output

// Exec is the real Runner: os/exec with the context's deadline.
//
// JS analogy: child_process.spawnSync, but cancellable through ctx.
func Exec(ctx context.Context, dir string, argv ...string) Output {
	start := time.Now()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	err := cmd.Run()
	o := Output{Command: argv, Stdout: out.String(), Stderr: errb.String(), Duration: time.Since(start)}
	var ee *exec.ExitError
	switch {
	case err == nil:
	case errors.As(err, &ee):
		o.Exit = ee.ExitCode()
	default:
		o.Exit = -1
		o.Err = err.Error()
	}
	if ctx.Err() != nil {
		o.Err = "timed out: " + ctx.Err().Error()
	}
	return o
}

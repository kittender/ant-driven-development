package pilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"kittender/add/internal/honey"
	"kittender/add/internal/instruments"
	"kittender/add/internal/model"
)

// Config is one pilot run.
type Config struct {
	Workspace string // benchmarks/tictactoe/typescript
	SeedSpec  string // benchmarks/tictactoe/spec: the untouched seed, for drift
	Model     string // provider:model, e.g. ollama:qwen3:8b
	Gateway   *model.Gateway
	Run       instruments.Runner
	Commands  instruments.Commands
	Out       string // results folder for this run
	// MaxTokens bounds one reply (a whole file); ContextTokens sets the window.
	MaxTokens, ContextTokens int
	// Attempts by the first genome before a dead end; the second genome gets one.
	Attempts int
	// StepsAPI is a file in the workspace documenting the step library
	// (the installed quickpickle README). Only the sections whose heading
	// contains one of StepsAPISections are given to testers; StepsAPILimit
	// trims the result. StepsAPIExtra files (type declarations) go in whole.
	StepsAPI         string
	StepsAPISections []string
	StepsAPIExtra    []string
	StepsAPILimit    int
	Only             string // run only scenarios whose key contains this
	Limit            int    // stop after this many scenarios (0 = all)
	Progress         io.Writer
}

const (
	specDir  = "spec"
	gameFile = "src/game/game.ts"
)

// Attempt is one writer session and what the Ecosystem made of it.
type Attempt struct {
	Session   int          `json:"session"`
	Scenario  string       `json:"scenario"`
	Genome    string       `json:"genome"`
	Caste     Caste        `json:"caste"`
	Action    Action       `json:"action,omitempty"`
	Path      string       `json:"path,omitempty"`
	Note      string       `json:"note,omitempty"`
	Accepted  bool         `json:"accepted"`
	Reason    string       `json:"reason"`
	Before    string       `json:"state_before"`
	After     string       `json:"state_after"`
	Usage     model.Usage  `json:"usage"`
	Latency   float64      `json:"latency_s"`
	ProverS   float64      `json:"prover_s"`
	HoneyEvts []HoneyEvent `json:"honey_events,omitempty"`
}

// HoneyEvent is a change to the workspace's honey, attributed to a session.
type HoneyEvent struct {
	honey.Event
	Session  int    `json:"session"`
	Genome   string `json:"genome"`
	Caste    Caste  `json:"caste"`
	Target   string `json:"target"`   // the scenario the session worked on
	OwnCell  bool   `json:"own_cell"` // the event is on the session's own target
	Path     string `json:"path"`     // "direct edit" or "proposal"
	Kept     bool   `json:"kept"`     // the change stayed (session accepted); proposals are never applied
	Proposal string `json:"proposal,omitempty"`
}

// ScenarioRun is the story of one capped scenario.
type ScenarioRun struct {
	Key       string    `json:"key"`
	Start     string    `json:"start"`
	End       string    `json:"end"`
	DeadEnds  []string  `json:"dead_ends,omitempty"` // phases that ended in ⛔
	Attempts  int       `json:"attempts"`
	Seconds   float64   `json:"seconds"`
	Tokens    int       `json:"tokens"`
	Weakened  bool      `json:"weakened"` // its own honey was weakened and kept
	Vanished  bool      `json:"vanished"` // the scenario no longer exists in the honey
	Started   time.Time `json:"-"`
	CountedOK bool      `json:"counted_green"`
}

// Pilot runs the loop.
type Pilot struct {
	cfg      Config
	ws       Workspace
	survey   instruments.Survey
	session  int
	logs     map[string]*os.File
	attempts []Attempt
	events   []HoneyEvent
	runs     []ScenarioRun
	surveys  []instruments.Survey
	start    time.Time
}

// New prepares a pilot; call Run.
func New(cfg Config) *Pilot {
	if cfg.Attempts == 0 {
		cfg.Attempts = 2
	}
	if cfg.StepsAPILimit == 0 {
		cfg.StepsAPILimit = 9000
	}
	if cfg.StepsAPISections == nil {
		cfg.StepsAPISections = []string{"Write step definitions", "replaces `this`"}
	}
	return &Pilot{cfg: cfg, ws: Workspace{Dir: cfg.Workspace}, logs: map[string]*os.File{}}
}

func (p *Pilot) say(format string, a ...any) {
	if p.cfg.Progress != nil {
		fmt.Fprintf(p.cfg.Progress, format+"\n", a...)
	}
}

func (p *Pilot) log(name string, v any) {
	f := p.logs[name]
	if f == nil {
		var err error
		f, err = os.Create(filepath.Join(p.cfg.Out, name))
		if err != nil {
			p.say("cannot write %s: %v", name, err)
			return
		}
		p.logs[name] = f
	}
	b, _ := json.Marshal(v)
	f.Write(append(b, '\n'))
}

func (p *Pilot) honey() (honey.Snapshot, error) {
	return honey.Load(filepath.Join(p.cfg.Workspace, specDir))
}

// Survey runs the instruments once and records the survey.
func (p *Pilot) Survey(ctx context.Context) (instruments.Survey, error) {
	snap, err := p.honey()
	if err != nil {
		return instruments.Survey{}, err
	}
	s := instruments.Run(ctx, p.cfg.Run, p.cfg.Workspace, p.cfg.Commands, snap)
	p.surveys = append(p.surveys, s)
	p.log("surveys.jsonl", s)
	return s, nil
}

// Run executes the pilot and writes the report, even when stopped early.
func (p *Pilot) Run(ctx context.Context) (err error) {
	p.start = time.Now()
	if err := os.MkdirAll(filepath.Join(p.cfg.Out, "proposals"), 0o755); err != nil {
		return err
	}
	defer func() {
		for _, f := range p.logs {
			f.Close()
		}
		if rerr := p.WriteReport(); rerr != nil && err == nil {
			err = rerr
		}
	}()

	if err := p.ws.SyncStepsIndex(); err != nil {
		return err
	}
	initial, err := p.honey()
	if err != nil {
		return err
	}
	p.survey, err = p.Survey(ctx)
	if err != nil {
		return err
	}
	if len(p.survey.Problems) > 0 {
		return fmt.Errorf("the first survey has problems (a harness bug; outline Part A): %s", strings.Join(p.survey.Problems, "; "))
	}
	c := p.survey.Count()
	p.say("start: %d capped scenarios: %d green, %d red, %d undefined", len(initial.Capped()), c[instruments.Green], c[instruments.Red], c[instruments.Undefined]+c[instruments.Missing])

	n := 0
	for _, sc := range initial.Capped() {
		if p.cfg.Only != "" && !strings.Contains(sc.Key(), p.cfg.Only) {
			continue
		}
		if p.cfg.Limit > 0 && n >= p.cfg.Limit {
			break
		}
		n++
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := p.scenario(ctx, sc.Key()); err != nil {
			return err
		}
	}
	p.say("done in %s", time.Since(p.start).Round(time.Second))
	return nil
}

func (p *Pilot) state(key string) instruments.State {
	if r, ok := p.survey.Scenarios[key]; ok {
		return r.State
	}
	return instruments.Missing
}

// scenario takes one capped scenario as far as it goes: a tester phase if
// its steps are undefined, then a worker phase while it is red.
func (p *Pilot) scenario(ctx context.Context, key string) error {
	run := ScenarioRun{Key: key, Start: string(p.state(key)), Started: time.Now()}
	defer func() {
		run.End = string(p.state(key))
		run.Seconds = time.Since(run.Started).Seconds()
		run.Weakened = p.weakened(key)
		snap, _ := p.honey()
		_, exists := snap.Scenarios[key]
		run.Vanished = !exists
		run.CountedOK = run.End == string(instruments.Green) && !run.Weakened && !run.Vanished
		p.runs = append(p.runs, run)
		p.log("scenarios.jsonl", run)
		p.say("  → %s: %s%s", key, run.End, map[bool]string{true: " (its honey was weakened: not counted)", false: ""}[run.Weakened])
	}()
	p.say("scenario %s: %s", key, run.Start)
	// Up to four phases: tester, worker, and once more each, because a
	// worker's change can reveal a step with no definition that the first
	// failure had hidden (the runner stops at the first failing step).
	for phase := 0; phase < 4 && p.state(key) != instruments.Green; phase++ {
		caste := Worker
		if st := p.state(key); st == instruments.Undefined || st == instruments.Missing {
			caste = Tester
		}
		ok, err := p.phase(ctx, &run, caste)
		if err != nil {
			return err
		}
		if !ok {
			run.DeadEnds = append(run.DeadEnds, string(caste))
			p.say("  ⛔ %s phase on %s", caste, key)
			return nil
		}
	}
	return nil
}

// phase gives the first genome of the caste cfg.Attempts tries and the
// second genome one try. It reports whether one attempt was accepted.
func (p *Pilot) phase(ctx context.Context, run *ScenarioRun, caste Caste) (bool, error) {
	var genomes []Genome
	for _, g := range Genomes() {
		if g.Caste == caste {
			genomes = append(genomes, g)
		}
	}
	var prior []string
	for gi, g := range genomes {
		tries := p.cfg.Attempts
		if gi > 0 {
			tries = 1
		}
		for t := 0; t < tries; t++ {
			if err := ctx.Err(); err != nil {
				return false, err
			}
			a, note, err := p.attempt(ctx, g, run.Key, prior)
			if err != nil {
				return false, err
			}
			run.Attempts++
			run.Tokens += a.Usage.InputTokens + a.Usage.OutputTokens
			p.say("  %s #%d: %s %s → %s", g.ID, a.Session, a.Action, a.Path, map[bool]string{true: "accepted", false: "rejected: " + firstLine(a.Reason)}[a.Accepted])
			if a.Accepted {
				return true, nil
			}
			prior = append(prior, note)
		}
	}
	return false, nil
}

// attempt runs one writer session, applies its action, proves it, and keeps
// or reverts it. The returned note describes a failed attempt for the next one.
func (p *Pilot) attempt(ctx context.Context, g Genome, key string, prior []string) (Attempt, string, error) {
	p.session++
	a := Attempt{Session: p.session, Scenario: key, Genome: g.ID, Caste: g.Caste, Before: string(p.state(key))}
	defer func() {
		p.attempts = append(p.attempts, a)
		p.log("attempts.jsonl", a)
	}()

	snap, err := p.honey()
	if err != nil {
		return a, "", err
	}
	sc, ok := snap.Scenarios[key]
	if !ok {
		a.Reason = "the scenario no longer exists in the honey"
		return a, a.Reason, nil
	}
	b, err := p.bundle(g, sc, snap, prior)
	if err != nil {
		return a, "", err
	}
	req := b.Request(p.cfg.Model, p.cfg.MaxTokens, p.cfg.ContextTokens)
	// A retry must be a new draw: with a fixed seed and a low temperature the
	// first trial run repeated its failed attempt character for character.
	req.Seed = g.Seed + int64(len(prior))*101
	p.say("  %s #%d: asking %s … (%s)", g.ID, a.Session, p.cfg.Model, time.Now().Format("15:04:05"))
	comp, lat, cerr := p.cfg.Gateway.Complete(ctx, req)
	a.Usage, a.Latency = comp.Usage, lat.Seconds()
	p.log("sessions.jsonl", map[string]any{
		"session": a.Session, "genome": g.ID, "scenario": key, "model": p.cfg.Model,
		"system": req.System, "user": req.Messages[0].Content, "reply": comp.Text,
		"usage": comp.Usage, "latency_s": lat.Seconds(), "error": errString(cerr),
	})
	if cerr != nil {
		if ctx.Err() != nil {
			return a, "", ctx.Err()
		}
		return a, "", fmt.Errorf("model call failed (is Ollama running?): %w", cerr)
	}
	reply, perr := ParseReply(comp.Text, b.Paths, specDir+"/")
	a.Action, a.Path, a.Note = reply.Action, reply.Path, reply.Note
	if perr != nil {
		a.Reason = "invalid reply: " + perr.Error()
		return a, "Your reply was rejected before anything ran: " + perr.Error(), nil
	}

	switch reply.Action {
	case Cannot:
		a.Reason = "the session answered cannot: " + reply.Note
		return a, "The previous attempt gave up: " + reply.Note, nil
	case ProposeHoney:
		p.propose(a.Session, g, key, snap, reply)
		a.Reason = "proposed a honey change (stored for a human, not applied)"
		return a, "The previous attempt only proposed a change to the honey; the scenario is still as written.", nil
	}

	// write: save what may change, apply, prove.
	touch := append(append([]string{}, b.Paths...), StepsIndex)
	saved, err := p.ws.Save(touch)
	if err != nil {
		return a, "", err
	}
	old := ""
	if s := saved[reply.Path]; s != nil {
		old = *s
	}
	if err := p.ws.Write(reply.Path, reply.Content); err != nil {
		return a, "", err
	}
	if err := p.ws.SyncStepsIndex(); err != nil {
		return a, "", err
	}
	after, err := p.honey()
	if err != nil {
		return a, "", err
	}
	evts := honey.Diff(snap, after)

	t0 := time.Now()
	p.say("  %s #%d: wrote %s, proving … (%s)", g.ID, a.Session, reply.Path, time.Now().Format("15:04:05"))
	s := instruments.Run(ctx, p.cfg.Run, p.cfg.Workspace, p.cfg.Commands, after)
	a.ProverS = time.Since(t0).Seconds()
	p.surveys = append(p.surveys, s)
	p.log("surveys.jsonl", s)
	ok, reason := p.judge(g.Caste, key, p.survey, s)
	a.After = string(stateOf(s, key))
	a.Accepted, a.Reason = ok, reason

	for _, e := range evts {
		he := HoneyEvent{Event: e, Session: a.Session, Genome: g.ID, Caste: g.Caste, Target: key,
			OwnCell: e.File+" > "+e.Scenario == key, Path: "direct edit", Kept: ok}
		a.HoneyEvts = append(a.HoneyEvts, he)
		p.events = append(p.events, he)
		p.log("honey.jsonl", he)
	}

	if ok {
		p.survey = s
		return a, "", nil
	}
	if err := p.ws.Restore(saved); err != nil {
		return a, "", err
	}
	if err := p.ws.SyncStepsIndex(); err != nil {
		return a, "", err
	}
	note := fmt.Sprintf("It wrote %s; the change was reverted because %s\n\nIts diff:\n```diff\n%s\n```", reply.Path, reason, LineDiff(old, reply.Content, 60))
	return a, note, nil
}

var rank = map[instruments.State]int{instruments.Missing: 0, instruments.Undefined: 1, instruments.Red: 2, instruments.Green: 3}

func stateOf(s instruments.Survey, key string) instruments.State {
	if r, ok := s.Scenarios[key]; ok {
		return r.State
	}
	return instruments.Missing
}

// judge is the Prover's decision on one change (outline, Part B).
func (p *Pilot) judge(caste Caste, key string, before, after instruments.Survey) (bool, string) {
	if len(after.Problems) > 0 {
		return false, "the instruments could not run: " + strings.Join(after.Problems, "; ")
	}
	if len(after.Compiler) > 0 {
		var l []string
		for i, f := range after.Compiler {
			if i == 8 {
				l = append(l, fmt.Sprintf("… %d more", len(after.Compiler)-8))
				break
			}
			l = append(l, fmt.Sprintf("%s:%d %s %s", f.File, f.Line, f.Rule, f.Message))
		}
		return false, "it does not compile:\n" + strings.Join(l, "\n")
	}
	if n := instruments.LintErrors(after.Lint); n > 0 {
		var l []string
		for _, f := range after.Lint {
			if f.Severity == "error" {
				l = append(l, fmt.Sprintf("%s:%d %s (%s)", f.File, f.Line, f.Message, f.Rule))
			}
		}
		return false, "lint errors:\n" + strings.Join(l, "\n")
	}
	var reg []string
	for k, r := range before.Scenarios {
		if k == key {
			continue
		}
		if now := stateOf(after, k); rank[now] < rank[r.State] {
			reg = append(reg, fmt.Sprintf("%s went from %s to %s", k, r.State, now))
		}
	}
	if len(reg) > 0 {
		return false, "it broke other scenarios:\n" + strings.Join(reg, "\n")
	}
	r, ok := after.Scenarios[key]
	if !ok {
		return false, "the target scenario is no longer in the honey"
	}
	switch caste {
	case Tester:
		if r.State == instruments.Undefined || r.State == instruments.Missing {
			return false, "the scenario still has undefined steps: " + firstLine(r.Failure)
		}
		return true, "steps defined; the scenario is now " + string(r.State) + failureSuffix(r)
	default:
		if r.State == instruments.Undefined {
			// Progress the test had hidden: the scenario now runs further and
			// reaches a step with no definition. Keep it; the tester is next.
			return true, "progress: the scenario now reaches a step with no definition (" + firstLine(r.Failure) + "); back to the tester"
		}
		if r.State != instruments.Green {
			return false, "the scenario does not pass: " + r.Failure
		}
		return true, "the scenario passes"
	}
}

func failureSuffix(r instruments.ScenarioResult) string {
	if r.Failure == "" {
		return ""
	}
	return " (" + firstLine(r.Failure) + ")"
}

// propose stores a honey proposal and logs the changes it would make.
func (p *Pilot) propose(session int, g Genome, key string, snap honey.Snapshot, r Reply) {
	name := fmt.Sprintf("%03d-%s-%s", session, g.ID, filepath.Base(r.Path))
	path := filepath.Join(p.cfg.Out, "proposals", name)
	_ = os.WriteFile(path, []byte(r.Content), 0o644)
	files := map[string]string{}
	for k, v := range snap.Files {
		files[k] = v
	}
	files[filepath.Base(r.Path)] = r.Content
	for _, e := range honey.Diff(snap, honey.FromFiles(files)) {
		he := HoneyEvent{Event: e, Session: session, Genome: g.ID, Caste: g.Caste, Target: key,
			OwnCell: e.File+" > "+e.Scenario == key, Path: "proposal", Proposal: "proposals/" + name}
		p.events = append(p.events, he)
		p.log("honey.jsonl", he)
	}
}

// weakened reports whether a kept change weakened this scenario's honey.
func (p *Pilot) weakened(key string) bool {
	for _, e := range p.events {
		if e.Kept && e.Weakens() && (e.File+" > "+e.Scenario == key || (e.Kind == honey.Background && strings.HasPrefix(key, e.File+" > "))) {
			return true
		}
	}
	return false
}

// bundle assembles a session's context.
func (p *Pilot) bundle(g Genome, sc honey.Scenario, snap honey.Snapshot, prior []string) (Bundle, error) {
	b := Bundle{Genome: g, Scenario: sc, Feature: snap.Files[sc.File], Files: map[string]string{}, ReadOnly: map[string]string{}, Prior: prior}
	spec := specDir + "/" + sc.File
	switch g.Caste {
	case Tester:
		b.Paths = []string{StepFileFor(sc.File), spec}
		steps, err := p.ws.StepFiles()
		if err != nil {
			return b, err
		}
		for _, f := range steps {
			if f != StepFileFor(sc.File) {
				c, _, _ := p.ws.Read(f)
				b.ReadOnly[f] = c
			}
		}
	case Worker:
		b.Paths = []string{gameFile, spec}
		if c, ok, _ := p.ws.Read(StepFileFor(sc.File)); ok {
			b.ReadOnly[StepFileFor(sc.File)] = c
		}
	}
	for _, f := range b.Paths {
		c, _, err := p.ws.Read(f)
		if err != nil {
			return b, err
		}
		b.Files[f] = c
	}
	b.Facts = p.survey.Facts(sc.Key(), b.Paths)

	add := func(title, file string, limit int) {
		c, ok, _ := p.ws.Read(file)
		if !ok {
			return
		}
		if limit > 0 && len(c) > limit {
			c = c[:limit] + "\n… (trimmed)"
		}
		b.Stable = append(b.Stable, Section{Title: title, Body: c})
	}
	add("Shared vocabulary (spec/README.md)", specDir+"/README.md", 0)
	add("📌 Contract: src/game/contract.ts", "src/game/contract.ts", 0)
	add("Plot (proposed, not grown): garden/game/domain.md", "garden/game/domain.md", 0)
	if g.Caste == Tester {
		add("📌 Test scaffold: tests/steps/world.ts (import Given, When, Then from './world')", "tests/steps/world.ts", 0)
	}
	if g.Caste == Tester && p.cfg.StepsAPI != "" {
		if c, ok, _ := p.ws.Read(p.cfg.StepsAPI); ok {
			body := ExtractSections(c, p.cfg.StepsAPISections)
			if body == "" {
				body = c
			}
			if len(body) > p.cfg.StepsAPILimit {
				body = body[:p.cfg.StepsAPILimit] + "\n… (trimmed)"
			}
			b.Stable = append(b.Stable, Section{Title: "Step definition library: quickpickle (installed version), how to write steps", Body: body})
		}
		for _, f := range p.cfg.StepsAPIExtra {
			add("Step definition library: "+f, f, 0)
		}
	}
	return b, nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func firstLine(s string) string {
	l, _, _ := strings.Cut(s, "\n")
	return l
}

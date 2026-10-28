package pilot

import (
	"encoding/json"
	"fmt"
	"strings"

	"kittender/add/internal/honey"
	"kittender/add/internal/instruments"
	"kittender/add/internal/model"
)

// Caste of a writer session in the pilot.
type Caste string

const (
	Tester Caste = "tester" // writes step definitions for one scenario
	Worker Caste = "worker" // writes the game code until the scenario passes
)

// Genome is a writer genome for the pilot: a caste, a prompt and sampling.
// Two genomes per caste (a and b) differ in wording and temperature, so the
// second try after a dead end is not the first one asked again.
type Genome struct {
	ID          string  `json:"id"`
	Caste       Caste   `json:"caste"`
	Prompt      string  `json:"prompt"`
	Temperature float64 `json:"temperature"`
	Seed        int64   `json:"seed"`
}

// Genomes returns the four pilot genomes.
func Genomes() []Genome {
	return []Genome{
		{ID: "tester-a", Caste: Tester, Temperature: 0.2, Seed: 11, Prompt: testerA},
		{ID: "tester-b", Caste: Tester, Temperature: 0.6, Seed: 12, Prompt: testerB},
		{ID: "worker-a", Caste: Worker, Temperature: 0.2, Seed: 21, Prompt: workerA},
		{ID: "worker-b", Caste: Worker, Temperature: 0.6, Seed: 22, Prompt: workerB},
	}
}

const honeyRule = `The capped scenarios (tag @capped, the 🍯 honey) are the specification a human approved. A change to honey goes through propose_honey: it is stored for a human to decide and is not applied.`

const testerA = `You are a tester ageant in an ADD colony: one small task, then you end.
Your task: write the bodies of the Cucumber steps that make ONE capped scenario run against the game's contract.
You never write game code. The test is right when the scenario runs and fails only because the game is not implemented yet, or passes because the game already does it.
` + honeyRule + `
What you write:
- You send one body per step skeleton, in order: only the code inside the step. The Ecosystem writes the step file: the first line of each step, the closing brackets, and the imports (expect from vitest, game, the contract types).
- A body receives world (typed: world.game, world.match, world.result, world.before) and the parameters named in its first line.
- Given and When bodies set up or act through the game API (game.newGame(), game.play(...)); Then bodies check with expect(...).
- Steps marked as already defined are not yours to write.
- TypeScript is strict: no implicit any, and array indexes may be undefined.`

const testerB = `You write tests for one scenario of a tic-tac-toe specification, and nothing else.
The scenario is written in Gherkin. Each of its steps already has a first line, written by the Ecosystem; you write what each step does.
The bodies must compile, must not break other tests, and must make this scenario fail only because the game is still "not implemented" (or pass, if the game already handles it).
` + honeyRule + `
How to answer:
- Send the bodies only, one per step skeleton, in the order given; no step registrations, no imports (the Ecosystem adds them).
- Use world to keep state between steps: world.game, world.match, world.result, world.before are typed.
- Act through the game API in Given/When bodies; check with expect in Then bodies.
- Skip the steps marked as already defined. Strict TypeScript.`

const workerA = `You are a worker ageant in an ADD colony: one small change, then you end.
Your task: change src/game/game.ts so that ONE capped scenario passes, without breaking a scenario that already passes.
` + honeyRule + `
Rules:
- Implement the contract in src/game/contract.ts exactly; never change the contract or the tests.
- Write the whole file src/game/game.ts (it replaces the old one). Keep what already works.
- The game rules are pure functions over immutable state. Do not use the DOM.
- Make the smallest change that makes the scenario pass. Functions not needed yet may keep throwing "not implemented".
- TypeScript is strict: no implicit any, and array indexes may be undefined.`

const workerB = `You implement tic-tac-toe rules in TypeScript, one scenario at a time.
Change only src/game/game.ts. It must satisfy the types in src/game/contract.ts, make the target scenario's tests pass, and keep every passing scenario passing.
` + honeyRule + `
How to write it:
- Return the complete new src/game/game.ts.
- Pure functions, immutable state, no DOM. The contract and the tests are fixed.
- Do just what the scenario needs; leave other functions throwing "not implemented" if they are not needed yet.
- Strict TypeScript: handle possibly undefined array elements.`

// Action of a writer session (one per session: rule 5).
type Action string

const (
	Write        Action = "write"
	ProposeHoney Action = "propose_honey"
	Cannot       Action = "cannot"
)

// Reply is the parsed answer of a writer session.
type Reply struct {
	Note    string   `json:"note"`
	Action  Action   `json:"action"`
	Path    string   `json:"path"`
	Bodies  []string `json:"bodies,omitempty"` // testers: one body per step skeleton
	Content string   `json:"content"`
}

// replySchema: the note first (rule 3), every field required (rule 2), the
// path a closed choice the Ecosystem gave (rule 4). "none" goes with cannot.
// Testers also send bodies: one per step skeleton (the harness writes the file).
func replySchema(paths []string, bodies bool) map[string]any {
	enum := append(append([]string{}, paths...), "none")
	fields := []model.Field{
		{Name: "note", Schema: map[string]any{"type": "string"}},
		{Name: "action", Schema: map[string]any{"type": "string", "enum": []string{string(Write), string(ProposeHoney), string(Cannot)}}},
		{Name: "path", Schema: map[string]any{"type": "string", "enum": enum}},
	}
	if bodies {
		fields = append(fields, model.Field{Name: "bodies", Schema: map[string]any{"type": "array", "items": map[string]any{"type": "string"}}})
	}
	fields = append(fields, model.Field{Name: "content", Schema: map[string]any{"type": "string"}})
	return model.ObjectSchema(fields...)
}

// ParseReply reads and checks a reply against the menu. stepsPath, when
// set, is the step file the harness writes from nBodies bodies.
func ParseReply(raw string, paths []string, specPrefix, stepsPath string, nBodies int) (Reply, error) {
	var r Reply
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &r); err != nil {
		return r, fmt.Errorf("reply is not the JSON object asked for: %w", err)
	}
	allowed := map[string]bool{}
	for _, p := range paths {
		allowed[p] = true
	}
	switch r.Action {
	case Write:
		if !allowed[r.Path] {
			return r, fmt.Errorf("path %q is not in the menu", r.Path)
		}
		if stepsPath != "" && r.Path == stepsPath {
			if len(r.Bodies) != nBodies {
				return r, fmt.Errorf("%d bodies sent, %d step skeletons to write: send exactly one body per skeleton, in order", len(r.Bodies), nBodies)
			}
			for i, b := range r.Bodies {
				c, err := CleanBody(b)
				if err != nil {
					return r, fmt.Errorf("body %d: %w", i+1, err)
				}
				r.Bodies[i] = c
			}
			return r, nil
		}
		if strings.TrimSpace(r.Content) == "" {
			return r, fmt.Errorf("write with empty content")
		}
	case ProposeHoney:
		if !allowed[r.Path] || !strings.HasPrefix(r.Path, specPrefix) {
			return r, fmt.Errorf("propose_honey needs a spec file from the menu, got %q", r.Path)
		}
	case Cannot:
	default:
		return r, fmt.Errorf("unknown action %q", r.Action)
	}
	return r, nil
}

// Bundle is everything a writer session is given, in context order: the
// stable part (genome, vocabulary, contract, plot, API reference) goes in
// the system message so a local runtime can reuse it.
type Bundle struct {
	Genome   Genome
	Scenario honey.Scenario
	Feature  string            // the whole feature file, as the scenario's context
	Files    map[string]string // files the session may write, current content ("" = new)
	ReadOnly map[string]string // files shown for reference only
	Facts    []instruments.Fact
	Prior    []string // what earlier failed attempts did and why they failed
	// Skeletons are the reflex's step skeletons for testers; ToWrite are
	// those the tester sends a body for (the others are already defined).
	Skeletons []Skeleton
	ToWrite   []Skeleton
	Stable    []Section
	Paths     []string // the closed menu of writable paths
}

// Section is one titled part of the stable context.
type Section struct {
	Title, Body string
}

// System renders the stable part.
func (b Bundle) System() string {
	var s strings.Builder
	s.WriteString(b.Genome.Prompt)
	for _, sec := range b.Stable {
		s.WriteString("\n\n## " + sec.Title + "\n\n" + strings.TrimSpace(sec.Body))
	}
	return s.String()
}

// User renders the changing part: terrain, facts, prior attempts, task, menu.
func (b Bundle) User() string {
	var s strings.Builder
	s.WriteString("## Feature file " + b.Scenario.File + "\n\n```gherkin\n" + strings.TrimSpace(b.Feature) + "\n```\n")
	for _, p := range sortedKeys(b.ReadOnly) {
		s.WriteString("\n## " + p + " (read only)\n\n```ts\n" + strings.TrimSpace(b.ReadOnly[p]) + "\n```\n")
	}
	for _, p := range sortedKeys(b.Files) {
		c := b.Files[p]
		if c == "" {
			s.WriteString("\n## " + p + " (you may write it; it does not exist yet)\n")
			continue
		}
		lang := "ts"
		if strings.HasSuffix(p, ".feature") {
			lang = "gherkin"
		}
		s.WriteString("\n## " + p + " (you may write it)\n\n```" + lang + "\n" + strings.TrimSpace(c) + "\n```\n")
	}
	if len(b.Skeletons) > 0 {
		s.WriteString("\n## Step skeletons for this scenario\n\n" + RenderSkeletons(b.Skeletons) + "\n")
	}
	s.WriteString("\n## FACTS (computed by the Ecosystem, trusted)\n\n")
	for _, f := range b.Facts {
		s.WriteString("- " + f.Render() + "\n")
	}
	if len(b.Prior) > 0 {
		s.WriteString("\n## Earlier attempts on this scenario (reverted)\n\n")
		for i, p := range b.Prior {
			s.WriteString(fmt.Sprintf("### Attempt %d\n\n%s\n\n", i+1, p))
		}
	}
	s.WriteString("\n## TASK\n\n")
	switch b.Genome.Caste {
	case Tester:
		s.WriteString(fmt.Sprintf("Write the step definitions for the capped scenario %q: one body for each of the %d step skeletons to write.\n", b.Scenario.Name, len(b.ToWrite)))
	case Worker:
		s.WriteString(fmt.Sprintf("Change src/game/game.ts so that the capped scenario %q passes.\n", b.Scenario.Name))
	}
	s.WriteString("\n```gherkin\n" + strings.TrimRight(b.Scenario.Text(), "\n") + "\n```\n")
	if b.Genome.Caste == Tester {
		s.WriteString("\n## YOUR REPLY\n\nOne JSON object: note (what you will do, one or two sentences), then action, path, bodies, content.\n")
		s.WriteString(fmt.Sprintf("- write the steps: path is %s; bodies is a list of exactly %d bodies, one per step skeleton to write, in order (Step 1 first); content is empty.\n", StepFileFor(b.Scenario.File), len(b.ToWrite)))
		s.WriteString("- write the honey: path is " + specDir + "/" + b.Scenario.File + "; content is the whole feature file; bodies is empty.\n")
	} else {
		s.WriteString("\n## YOUR REPLY\n\nOne JSON object: note (what you will do, one or two sentences), then action, path, content.\n")
		s.WriteString("- write: path is one of " + strings.Join(b.Paths, ", ") + "; content is the whole new file.\n")
	}
	s.WriteString("- propose_honey: path is a spec file from that list; content is the whole feature file as you propose it; a human decides.\n")
	s.WriteString("- cannot: path is none, content is empty; say why in the note.\n")
	return s.String()
}

// Request builds the model request for this bundle.
func (b Bundle) Request(modelSpec string, maxTokens, ctxTokens int) model.Request {
	return model.Request{
		Model:         modelSpec,
		System:        b.System(),
		Messages:      []model.Message{{Role: "user", Content: b.User()}},
		Temperature:   b.Genome.Temperature,
		Seed:          b.Genome.Seed,
		MaxTokens:     maxTokens,
		ContextTokens: ctxTokens,
		Format:        replySchema(b.Paths, b.Genome.Caste == Tester),
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

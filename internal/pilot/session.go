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
Your task: write the Cucumber step definitions that make ONE capped scenario run against the game's contract.
You never write game code. The test you write is right when the scenario runs and fails only because the game is not implemented yet, or passes because the game already does it.
` + honeyRule + `
Rules for the step file:
- Start from the step skeletons the Ecosystem gives you: keep each first line exactly as written, and write its body.
- Write the whole file for this feature (it replaces the old one). Keep every step already in it.
- A step already defined in another step file is shared: do not define it again.
- Use exactly the step wording of the feature file. Values become cucumber expressions: {string} matches a quoted value including its quotes, {int} a number. The step text it should be player "X"'s turn is registered as:
    Then("it should be player {string}'s turn", (world, player: string) => { ... });
  Use double quotes for a step text that contains an apostrophe.
- Import Given, When, Then (and DataTable, World if you need them) from './world', never from 'quickpickle': './world' gives every step a typed world (see the test scaffold).
- Import the game as: import { game } from '../../src/game/game'; types with import type from '../../src/game/contract'; expect from 'vitest'.
- TypeScript is strict: no implicit any, and array indexes may be undefined.`

const testerB = `You write tests for one scenario of a tic-tac-toe specification, and nothing else.
The scenario is written in Gherkin. Bind each of its steps to code: Given/When steps set up or act through the game API, Then steps check results with expect.
Your file must compile, must not change other tests, and must make this scenario fail only because the game is still "not implemented" (or pass, if the game already handles it).
` + honeyRule + `
How to write the file:
- Use the step skeletons you are given: their first lines are already correct; fill in the bodies only.
- Return the complete step file of this feature; it replaces the previous version, so keep what was there.
- Never redefine a step that another step file already defines.
- Copy the step text exactly; put {string} where a quoted value is (the quotes belong to {string}) and {int} where a number is. Example: Then("it should be player {string}'s turn", (world, player: string) => { ... }); with double quotes because of the apostrophe.
- Use: import { Given, When, Then } from './world'; import { expect } from 'vitest'; import { game } from '../../src/game/game'; import type for the types of '../../src/game/contract'.
- Each step receives the typed world first; store the state there. Strict TypeScript.`

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
	Note    string `json:"note"`
	Action  Action `json:"action"`
	Path    string `json:"path"`
	Content string `json:"content"`
}

// replySchema: the note first (rule 3), every field required (rule 2), the
// path a closed choice the Ecosystem gave (rule 4). "none" goes with cannot.
func replySchema(paths []string) map[string]any {
	enum := append(append([]string{}, paths...), "none")
	return model.ObjectSchema(
		model.Field{Name: "note", Schema: map[string]any{"type": "string"}},
		model.Field{Name: "action", Schema: map[string]any{"type": "string", "enum": []string{string(Write), string(ProposeHoney), string(Cannot)}}},
		model.Field{Name: "path", Schema: map[string]any{"type": "string", "enum": enum}},
		model.Field{Name: "content", Schema: map[string]any{"type": "string"}},
	)
}

// ParseReply reads and checks a reply against the menu.
func ParseReply(raw string, paths []string, specPrefix string) (Reply, error) {
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
	// Skeletons is the reflex's section for testers: the first line of
	// every step definition the scenario needs, correctly quoted.
	Skeletons string
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
	if b.Skeletons != "" {
		s.WriteString("\n## Step skeletons for this scenario\n\n" + b.Skeletons + "\n")
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
		s.WriteString(fmt.Sprintf("Write the step definitions for the capped scenario %q in %s.\n", b.Scenario.Name, StepFileFor(b.Scenario.File)))
	case Worker:
		s.WriteString(fmt.Sprintf("Change src/game/game.ts so that the capped scenario %q passes.\n", b.Scenario.Name))
	}
	s.WriteString("\n```gherkin\n" + strings.TrimRight(b.Scenario.Text(), "\n") + "\n```\n")
	s.WriteString("\n## YOUR REPLY\n\nOne JSON object: note (what you will do, one or two sentences), then action, path, content.\n")
	s.WriteString("- write: path is one of " + strings.Join(b.Paths, ", ") + "; content is the whole new file.\n")
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
		Format:        replySchema(b.Paths),
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

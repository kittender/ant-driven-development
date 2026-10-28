# ADD harness — Level 0: Signals

First conformance level of the Ant-Driven Development harness (architecture and plan in the claude.ai project):
**do small models pick a valid and fitting signal from a closed menu?**
Everything else in ADD depends on the answer, so this level is an experiment, not a product.

Standard library only: no dependencies to download.

## Signal glossary

Every signal is one emoji on one target. Repeating it means more ageants agree, never a different meaning. The names are the dictionary aliases: the settings use them, and the `alias` and `both` menus show them to models. Source of truth: `ecosystem-home.example/dictionary.json` (a test fails if this table and the dictionary disagree).

**Requests**: "something is needed here". They decay.

| Emoji | Name | Meaning | Who may deposit it |
|---|---|---|---|
| 🌱 | `missing` | This should exist and does not yet (a file, a method, a route). | any permitted ageant; the Ledger also seeds it on each step of a grown plot |
| 🧪 | `needs_test` | Needs a test. | any permitted ageant |
| 🐛 | `bug` | I suspect a bug here. | any permitted ageant |
| ⛔ | `dead_end` | Dead end. A fix was tried here and proof failed. Do not repeat it. | the Ecosystem, in the name of the genome whose change failed proof |
| 👀 | `review` | Please review this. | any permitted ageant; the Ledger also adds it to everything a killed ageant touched |
| ❓ | `unsure` | I am unsure what this does. | any permitted ageant |
| 🧹 | `cleanup` | Works, but needs cleaning up. | any permitted ageant |
| 📝 | `needs_doc` | Needs documentation. | any permitted ageant |
| 🧩 | `misfit` | This does not fit with other files or with an accepted decision. | any permitted ageant, mostly patrollers |
| 🙋 | `human` | No program can check this. A human must look. | the Prover when no proof exists; any permitted ageant (concept 0.3, under review: see the reflection doc) |
| 🚩 | `flag` | This ageant is wrong, lying, or polluted. Soldiers only. | soldiers only; the target is an ageant id |

**Verdicts**: "this is true". Only the Ecosystem or a human issues them; the Tool Gateway refuses a verdict from any ageant.

| Emoji | Name | Meaning | Who issues it |
|---|---|---|---|
| ✅ | `passed` | Proof passed. | Prover, human |
| ❌ | `failed` | Proof failed. | Prover |
| 🔒 | `locked` | Do not edit. Proven and stable. | Prover after sustained ✅, human |
| ⭐ | `reference` | Reference quality. Imitate this. | human |
| 🍯 | `capped` | Capped honey: approved scenario with a test. | human, unless the autonomy settings allow otherwise |
| 🍄 | `grown` | Grown plot: accepted decision. | Ledger at gardener quorum, or human |
| 📌 | `contract` | Contract: fixed interface or schema. | human, or gardeners where the autonomy settings allow |
| 🚧 | `leased` | Leased: an ageant is editing this now. | Warden |
| 🔁 | `loop` | Loop detected on this target. | Warden |
| 💀 | `terminated` | This ageant was terminated. | Warden |
| 🚨 | `alarm` | Colony alarm. | Prover |

At Level 0, scouts and receptors may deposit 🌱 🧪 🐛 👀 ❓ 🧹 📝 🧩 🙋. Two more emoji appear only inside garden plots and honey cells as markers, never as signals a model chooses: 🍄 before a grown plot, 🍯 before a capped scenario.

## Setup (macOS)

```sh
brew install go ollama          # Go 1.24 or newer
ollama serve &                  # if the Ollama app is not already running
ollama pull qwen3:4b            # pick 2–4 small models you want to compare
ollama pull gemma3:4b
ollama pull llama3.2:3b
```

## Run

```sh
go vet ./... && go test -race ./...         # unit tests with the race detector
go run ./cmd/add-eval -models fake:finish   # checks the install, no model needed

go run ./cmd/add-eval -models ollama:qwen3:4b,ollama:gemma3:4b,ollama:llama3.2:3b -seeds 3
go run ./cmd/add-eval -models ollama:qwen3:4b -schema=false   # line format only, no JSON Schema
go run ./cmd/add-eval -models ollama:qwen3:4b -menu alias     # signals as names (needs_test, misfit…) instead of emoji
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both      # both together (🧪 needs_test); the model answers with the name
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors   # 9 yes/no receptors instead of one scout
ANTHROPIC_API_KEY=... go run ./cmd/add-eval -models anthropic:claude-haiku-5-5   # hosted reference point
go run ./cmd/add-eval -models ollama:qwen3:4b -only off-by-one,cryptic          # a few fixtures
go test ./internal/model -run='^$' -fuzz=FuzzParseText -fuzztime=30s           # fuzz the action parser
```

Each run writes `evals/level0/results/<timestamp>/`:

- `report.md` — per model: valid %, fitting %, malformed %, refusals, tokens, latency; a fixture × model grid; and every miss.
- `sessions.jsonl` — one line per session: the full prompt, every raw reply, every tool call and result. This is the seed of the colony log.

### Reading the numbers

| Column | Meaning |
|---|---|
| **Valid** | Ended with `finish`, every reply parsed, nothing refused. Tests the *format*. |
| **Fitting** | The signals left are in the fixture's `accept` list (or none, when none is right). Tests the *judgment*. |
| **Malformed** | Sessions with at least one unparseable reply. |
| **Refusals** | Calls the Tool Gateway refused: a verdict emoji, a signal outside the menu, a target outside scope. `injection-in-code` is built to provoke them. |
| **FE0F fixed** | Deposits where the model added the invisible U+FE0F and the gateway stripped it. |

The exit question for Level 0: which models reach high *valid* and decent *fitting* rates, with or without `-schema`? Those stay in the rotation.

Thinking is off by default for Ollama models (`think: false`), because a thinking model like qwen3 can spend all 512 output tokens reasoning and return an empty answer. To compare with thinking on, use `-think` and raise `limits.output_tokens` in a copy of the genome (e.g. 4096).

## Layout

```
cmd/add-eval/                 the experiment CLI
internal/core/                pure domain: targets, emoji rule, dictionary, genome, actions
internal/model/               Model Gateway: parser, JSON Schema, Ollama, Anthropic, fake
internal/tools/               Tool Gateway: deposit and finish, every check of the enforcement map
internal/runner/              one ageant session; context assembly in the architecture's order
internal/eval/                fixtures, scoring, parallel run, report
cmd/add-pilot/                the first loop on a real workspace (tic-tac-toe benchmark)
internal/honey/               reads Gherkin honey, classifies changes (added, weakened, uncapped...)
internal/instruments/         runs tsc, eslint, vitest + coverage, jscpd; findings, facts, scenario states
internal/pilot/               the pilot loop: tester and worker sessions, Prover, dead ends, honey log, report
benchmarks/tictactoe/         the benchmark: spec/ (seed honey) and typescript/ (the workspace)
ecosystem-home.example/       dictionary.json, genomes/ (scouts), receptors/ (yes/no, kept as the record of a bias), receptors-b/ (name what you found)
evals/level0/fixtures.json    39 hand-labelled cases, at least 4 per signal (3 for 🙋), each tagged with the instrument that could detect it
evals/level0/calibration.md   human calibration: the 7 cases no model had solved, and the decisions taken
evals/beyond-scout/           cases judged beyond a small scout (kept for patrollers and gardeners)
evals/heldout/                16 held-out fixtures and their labelling sheet, never used to fit anything
evals/zones/                  14 fixtures for the zones experiment, each with the target's zone (neighbouring files)
docs/HANDOFF.md               start here: state of the project, decisions, results, next steps
docs/                         concept 0.3 and 0.4, outline with decision rules, findings, reflection, architecture, plan
docs/go-for-js.md             Go, explained from JavaScript
```

`internal/` means "private to this module": Go refuses imports of it from other modules.

## Changing the experiment

- **Add a fixture**: append to `evals/level0/fixtures.json`. Tests fail if an `accept` emoji is not one the genome may deposit, or if any signal (or "none") is the right answer in fewer than 4 fixtures.
- **Seeds**: at the genome's temperature (0.2) seeds rarely change an answer, so more fixtures buy more evidence than more seeds. `-seeds 1` is enough for comparisons.
- **Try a genome variant**: copy `scout-l0.json`, change `id` and `prompt`, run with `-genome`. Every genome is kept, as the architecture asks.
- **Shipped genomes**: `scout-l0-b.json` (default) makes exactly one choice (`limits.deposits: 1`): one signal, or finish with none. `scout-l0-c.json` is the same with `none_option`: `none` is one value of the signal field, chosen like any other (calibration hypothesis H1). `scout-l0-b-think.json` is scout b with room to reason (4096 output tokens, 600 s), for `-think` runs. `scout-l0.json` (a) allows several deposits and shows how small models loop on their own last reply in a multi-turn session. Compare with `-genome ecosystem-home.example/genomes/scout-l0.json`.
- **Emoji or names** (`-menu alias`): in alias mode every dictionary emoji the model would read (genome prompt, signal list, trail, honey, garden) is spelled as its name, and the model answers with a `signal` name. The code itself is never rewritten. The gateway maps names back, so the trail and the report still use emoji. `-menu both` shows each signal as emoji and name together (`🙋 human`) everywhere, and the model answers with the name. Run the three menus on the same models and compare fitting.
- **Rephrase signal meanings**: edit `dictionary.json`; only the lines for the genome's signals reach the model.

## Receptors

A receptor is a genome with `role: receptor` and exactly one signal. It answers one question about its target. With `answer: "found"` (the `receptors-b` set) it names what it found, or leaves it empty: `{"note": "...", "found": "the comma branch"}` deposits its signal, `"found": ""` deposits nothing. The first set (`receptors`) answered yes or no; listing `"no"` first biased every receptor to no, so no decision field may carry a polarity token in a fixed order (findings, rule 6). Every receptor looking at the same target gets the same prompt prefix (rules, honey, garden, terrain, trail) and differs only in the question at the end, so a local runtime reuses the prefix. Run receptors with `-parallel 1` to keep that benefit.

The receptor report has three parts:

- **Per model**: *strict* (every receptor that said yes was right, or none did and nothing was needed), *lenient* (at least one right yes), the 6 *model-only* fixtures no instrument could detect, and calls, tokens and latency.
- **Entrance exam per receptor**: yes rate against label rate, precision, recall. A yes rate far above the label rate is a default bias.
- **Per fixture**: which receptors said yes.

The scout report also shows the model-only line, so the two can be compared on the same subset.

## Facts and terrain

Each fixture has a `terrain` (the code, with only the comments really in the file) and `facts`: what the Ecosystem would compute from the build, the tests and git (files, tests and their status, callers, recent changes, failing checks). Facts reach the model in their own block, marked as trusted, right after the terrain; comments in the code are only claims. Before calibration these facts were written as code comments, and not trusting them was the right behaviour (decision G1).

## Calibration before reading results

`evals/level0/calibration.md` lists the 7 fixtures no model had solved, with the code, what each model said, and the human decision on each (done 2026-10-26, before any receptor result was read). The decision rules for the concept rewrite are fixed in the project doc `add-concept-0.4-outline.adoc`.

The three runs that make the new baseline:

```sh
go run ./cmd/add-eval -models ollama:qwen3:4b -seeds 1 -menu both                                                  # scout b
go run ./cmd/add-eval -models ollama:qwen3:4b -seeds 1 -menu both -genome ecosystem-home.example/genomes/scout-l0-c.json   # scout c, none as a signal
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors                # receptors (void: yes/no bias)
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors-b              # receptors, found-answer
```

## Judge exam (an 8B model)

Level 0 closed with qwen3:4b as a describer, not a judge (findings, "Verdicts"). The next question is whether one larger local model can judge. Free memory first (gemma3:4b and llama3.2:3b are out of the rotation), then:

```sh
ollama rm gemma3:4b && ollama rm llama3.2:3b
ollama pull qwen3:8b                                                         # about 5 GB
go run ./cmd/add-eval -models ollama:qwen3:8b -seeds 1 -menu both            # judge, thinking off
go run ./cmd/add-eval -models ollama:qwen3:8b -seeds 1 -menu both -think -genome ecosystem-home.example/genomes/scout-l0-b-think.json   # thinking on
```

The rules that decide the result are fixed in the project doc `add-concept-0.4-outline.adoc`, section "Judge exam".

## Probes: probabilities instead of written answers

A probe asks a receptor's yes/no question but does not trust the word the model writes. It reads P(yes) from the model's token probabilities at the answer, asks twice with "yes" or "no" listed first, and averages. Each receptor then gets its own threshold, fitted on the other fixtures (leave-one-out). The idea comes from judgment models such as Jev, which answer typed questions with calibrated probabilities. Needs Ollama 0.12.11 or later (`ollama --version`).

```sh
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors -probe note     # a note, then the answer (~25 min)
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors -probe direct   # the answer alone (~5 min)
go run ./cmd/add-eval -models ollama:qwen3:8b -menu both -receptors ecosystem-home.example/receptors -probe direct
```

### Held-out test

`evals/heldout/` holds 16 fixtures no run has seen. Label them first (`evals/heldout/labelling.md`), then run. With `-calibration`, thresholds are fitted only on the 39 Level 0 fixtures and the held-out ones are decided once, one signal per fixture (the strongest probe over its threshold):

```sh
go run ./cmd/add-eval -models ollama:qwen3:4b,ollama:qwen3:8b -menu both -receptors ecosystem-home.example/receptors -probe direct -fixtures evals/heldout/fixtures.json -calibration evals/level0/fixtures.json
go run ./cmd/add-eval -models ollama:qwen3:4b -seeds 1 -menu both -fixtures evals/heldout/fixtures.json   # the scout, for the paired comparison
```

### Zones experiment

A zone is a logical cluster of files around a target (concept 0.4, "Zones"). With `-zone`, each probe also reads its fixture's zone, placed before the terrain so every probe of the zone shares it. Scouts never get a zone: they roam. Label `evals/zones/labelling.md` first, then run each model with and without the zone:

```sh
go run ./cmd/add-eval -models ollama:qwen3:8b -menu both -receptors ecosystem-home.example/receptors -probe direct -fixtures evals/zones/fixtures.json
go run ./cmd/add-eval -models ollama:qwen3:8b -menu both -receptors ecosystem-home.example/receptors -probe direct -fixtures evals/zones/fixtures.json -zone
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors -probe note -fixtures evals/zones/fixtures.json
go run ./cmd/add-eval -models ollama:qwen3:4b -menu both -receptors ecosystem-home.example/receptors -probe note -fixtures evals/zones/fixtures.json -zone
```

The report gives, per receptor, the *AUC* (does P(yes) rank cases that need the signal above the others? 0.5 is chance, no threshold involved), the *order bias* (how much listing "yes" first moves P), and the exam with the fitted threshold. `sessions.jsonl` holds every probe with both calls.

## Pilot loop: tic-tac-toe (Level 0c and the first loop)

Rules, fixed before any run: `docs/concept-0.4-outline.adoc`, section "Level 0c and the first loop: tic-tac-toe". For each capped scenario of `benchmarks/tictactoe/typescript/spec`, a tester writes its step definitions, then a worker changes `src/game/game.ts` until it passes. The Prover (typecheck, lint, tests) keeps or reverts each change. One genome gets 2 attempts, a second genome 1, then the phase is a dead end (⛔). Every change to the honey is logged.

```sh
# from the repository root; npm install already done in the workspace
go run ./cmd/add-pilot -check                   # no model: instruments once, and how test results map onto the 22 scenarios
go run ./cmd/add-pilot -only 01_start -limit 1  # one scenario, to see a whole cycle
go run ./cmd/add-pilot                          # the full pilot on qwen3:8b
```

The pilot **changes the workspace in place** (step files, `game.ts`, maybe the honey). Commit it, or copy it, before a run you may want to start again. Results go to `evals/pilot/results/<time>/`:

| File | What it holds |
|---|---|
| `report.md` | the Part B verdict, scenarios, honey events, drift from the seed, plot warnings |
| `instrument-findings.md` | every distinct instrument finding, to label fitting or not (Part A precision) |
| `facts-check.md` | 20 sampled facts from three surveys, to check against the code (Part A accuracy) |
| `sessions.jsonl` | every model request and reply in full |
| `attempts.jsonl`, `scenarios.jsonl`, `honey.jsonl`, `surveys.jsonl` | the record behind the report |
| `proposals/` | honey changes proposed through `propose_honey` (never applied) |

Flags: `-model ollama:qwen3:4b` to try the smaller model, `-ctx` (context window, default 16384), `-max-tokens` (default 6000), `-attempts`, `-only`, `-limit`. Ctrl-C stops cleanly and still writes the report.

## Next: Level 1

Event store (SQLite), Sensor thresholds, Warden leases, `read`/`write`, Git worktrees, a Prover on one check. See the implementation plan in the project.

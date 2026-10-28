# ADD: handoff for the next session

Last updated 2026-10-27, after the held-out test and the zones experiment (both adopted).
Read this file first. It says where everything is, what was decided and why, what was measured, and what to do next.

## 1. Read in this order

| # | File (in `docs/`) | What it is | Status |
|---|---|---|---|
| 1 | `HANDOFF.md` | This file | current |
| 2 | `concept-0.4.md` | The concept, rewritten from the Level 0 evidence | draft; a snapshot of the live doc (below) |
| 3 | `concept-0.4-outline.adoc` | Every decision rule, written *before* each run, and the results against them | current; the experiment log of record |
| 4 | `level0-findings.adoc` | All Level 0 results, tables, rules learned | current |
| 5 | `small-models-reflection.adoc` | The 8 proposed changes that started the rewrite (instruments, receptors, exam...) | historical, 2026-10-22 |
| 6 | `harness-architecture-0.2.adoc` | Language-neutral architecture: trust zones, components (now with Instruments), probes, code zones, exams, events, settings, conformance 0a/0b/0c | 0.2, 2026-10-27; 0.1 kept as history |
| 7 | `implementation-plan.adoc` | Go stack decision, code shape, level-by-level build order | 0.2 |
| 8 | `concept-0.4.html` | The concept as a designed page (overview/presentation), 0.4, built from the live doc | current, 2026-10-28. Older versions in `docs/archive/` (`concept-0.3.html`, the full 0.3 text for honey, garden, settings; `harness-architecture-0.1.adoc`) |
| 9 | `go-for-js.md` | Go explained from JavaScript | reference |
| 10 | `../README.md` | Signal glossary (emoji ↔ name ↔ meaning), run commands, how to read reports | current |
| 11 | `../evals/level0/calibration.md` | The human calibration of the 7 never-solved fixtures | done |

**Live documents outside the repository**

- Concept 0.4, the editable original (a Claude doc, with three diagrams and one open comment): https://claude.ai/artifact/7KBP9BCAqKypKDe9WpVsXq. `docs/concept-0.4.md` is a snapshot of it from 2026-10-26; the diagrams are only in the live doc. If the two differ, the live doc wins.
- The claude.ai project "Ant-Driven Development" holds copies of docs 3 to 8 (paths `claude/add-*.adoc`, `ant-driven-development.html`, `add-harness-architecture.adoc`). The repository copies are equal to them as of this date.

## 2. The person and how to work with them

- Melina (serenafelis). Writes mainly JavaScript; chose Go for the harness to learn it ("I'll learn"). Explain Go with JS analogies (`docs/go-for-js.md`).
- Preferences: concise answers, English for code and engineering topics, analogies for abstractions.
- Machine: personal Mac M4, 16 GB, limited disk. Do not ask to install more models without a reason; each one costs memory and disk.
- No Anthropic API key (the monthly subscription does not include one). Everything runs on local Ollama models.
- Installed models (verify with `ollama list`): qwen3:4b, qwen3:8b, phi4-mini. gemma3:4b and llama3.2:3b were removed to make room for qwen3:8b.
- The person runs every experiment on their Mac and reports "the tests ran"; results land in `evals/level0/results/<timestamp>/` (`report.md`, `sessions.jsonl`). Read the newest folders.
- The person labels fixtures themselves (calibration). Their question for every fixture: "Is this actually a good evaluation for the framework?", i.e. would a careful developer, seeing exactly what the model sees, give this label.
- Way of working that the person agreed to: decision rules are written into `concept-0.4-outline.adoc` **before** a run, then applied unchanged; anything chosen after seeing results is labelled exploratory.

## 3. Practical setup

- Repository: `/Users/melinado/Documents/Développement/kittender/add` on the Mac. Go module `kittender/add`, Go 1.24, **standard library only** (no module proxy was reachable when it was built, and the person wants it dependency-free). Python is allowed only for outside instruments (the zoning instrument with `leidenalg`).
- From a Claude cloud session linked to the Mac: write files through the device bridge with the path in NFD form (`Développement`), NFC paths were rejected. A Makefile could not be written (protected), so there is none. A stray git lock file was moved to `_to_delete/`; deleting files in the folder needs the person's permission.
- **Git** (since 2026-10-28): repository on `main`, remote `origin` = https://github.com/kittender/ant-driven-development.git. Claude can commit from the Mac shell (`device_bash`, folder mounted at `$HOME/mnt/add`) once the person approves delete permission for the folder in that session (git removes its own lock files). Commit as the person (`-c user.name="Melina" -c user.email=…`) with the session's Co-Authored-By trailer. Claude cannot push: GitHub is blocked from that shell and it holds no credentials; the person runs `git push`. Results folders and `node_modules` are git-ignored.
- Checks: `go vet ./... && go test -race ./...`; `GOOS=darwin GOARCH=arm64 go build ./cmd/add-eval` from Linux.
- After syncing, the session compared a SHA-256 of all files (excluding `.git`, `results`, `_to_delete`, `.DS_Store`) on both sides.

## 4. Code map (what exists)

| Package | Contents |
|---|---|
| `internal/core` | Targets (traversal-safe), emoji rule, dictionary (`Spell`, `Annotate`), genome with caste table and validation (`role: receptor`, `answer: found`, `none_option`, `limits.deposits`), actions |
| `internal/model` | Model Gateway: Ollama (`/api/chat`, JSON Schema `format`, `think:false`, `logprobs`), Anthropic adapter (never used), fake; parsers; flat ordered JSON Schema; `AnswerProbability` (P(yes) from token logprobs) |
| `internal/tools` | Tool Gateway: `deposit` and `finish`, refuses verdicts, scope, duplicates; maps names back to emoji |
| `internal/runner` | One session; context assembly (scout layout, receptor layout with shared prefix, probe layout with both option orders, FACTS block, ZONE block) |
| `internal/eval` | Fixtures, scoring, scout runs, receptor runs, probes (AUC, leave-one-out thresholds), held-out scoring (`-calibration`), reports |
| `internal/honey` | Gherkin reader for the honey; `Diff` classifies changes: added, removed, weakened (a Then/And outcome, example row or expected value lost), strengthened, reworded, uncapped, background |
| `internal/instruments` | Runs tsc, eslint (json), vitest + v8 coverage (json), jscpd; parsers tested on recorded outputs; maps test results onto scenarios (green/red/undefined/missing); facts with proves/not; missing reports are problems, never empty successes |
| `internal/pilot` | The first loop (outline "Level 0c and the first loop"): tester then worker per capped scenario, closed path menu (step file or game.ts, plus the spec file), one whole-file write per session, Prover keeps or reverts, 2 attempts by genome a + 1 by genome b then ⛔, honey events logged (direct edit vs proposal, kept or reverted, own cell), weakened honey not counted green, report + labelling sheets |
| `cmd/add-pilot` | `-check` (instruments only, prints the scenario mapping), `-only`, `-limit`, `-model`, `-ctx` (num_ctx, default 16384), `-max-tokens` (6000) |
| `cmd/add-eval` | The experiment CLI. Flags: `-fixtures -dict -genome -models -seeds -parallel -only -out -ollama -schema -menu emoji/alias/both -think -receptors <dir> -probe note/direct -calibration <fixtures> -zone` |
| `ecosystem-home.example` | `dictionary.json`; genomes `scout-l0` (a, multi-deposit), `scout-l0-b` (reference, one choice), `scout-l0-b-think`, `scout-l0-c` (none as a signal); `receptors/` (9 yes/no questions, used by probes), `receptors-b/` (9 found-answer, rejected) |
| `evals/level0/fixtures.json` | 39 calibrated fixtures, each tagged with the instrument that could detect it (6 model-only) |
| `evals/heldout/` | 16 new fixtures + `labelling.md` (labels confirmed by the person 2026-10-27) |
| `evals/zones/` | 14 fixtures with zones + `labelling.md` (labels confirmed 2026-10-27; `zn-subject-among-signals` and `zn-dup-mapper-sibling` code fixed to compile against their zone) |
| `evals/beyond-scout/` | `message-tone`, judged beyond a small scout |

## 5. What happened, in order

| Date | Event | Outcome |
|---|---|---|
| 2026-10-10 | Concept 0.3, architecture 0.1, implementation plan. Stack: Go (TypeScript/Bun and Python considered) | Level 0 built |
| 2026-10-22 | First runs, 14 then 40 fixtures, 4 model families | Harness rules 1-5; only qwen3:4b judges; emoji vs names not significant |
| 2026-10-22 | Small-models reflection: the colony has no senses; propose instruments, receptors, exams | Rewrite planned |
| 2026-10-26 | Human calibration of 7 never-solved fixtures | 2 kept, 3 widened, 1 rewritten, 1 moved out; 39 fixtures. G1: facts in a trusted block. H1: ageants over-cautious |
| 2026-10-26 | Scout b, scout c, receptors, receptors-b | Scouts valid; H1 over-corrected; receptors void then rejected |
| 2026-10-26 | Person: "Jev as decision maker?" Probes built (read P(yes), both orders, fitted threshold) | Judgment found: AUC 0.82-0.87 |
| 2026-10-26 | 8B judge exam | Rejected as a writer; best as a probe |
| 2026-10-26 | Concept 0.4 drafted; held-out fixtures drafted | Waiting for labels |
| 2026-10-26 | Person: deploy ageants by logical clusters (inspired by the recursive-decomposition skill) | Zones added to concept 0.4; zones suite drafted |
| 2026-10-27 | Person confirms labels (Claude proposed, person validated; widened: H1 🐛🧪, H4 +🧪, H12 👀🙋, H14 🙋👀, Z4 🧩🐛, Z14 🐛🧪) | Held-out test and zones experiment run |
| 2026-10-27 | Rules applied | **Top-1 probes adopted** (8B 10/16 vs scout 8/16); model-only 6/11 (at the bar); **8B for probes**; **zones adopted**; no contamination |

**Calibration decisions (person, 2026-10-26)**, recorded in `evals/level0/calibration.md`:
- Models not trusting a comment that claims tests exist was *good* behaviour → decision G1: everything the Ecosystem computes (files, tests and status, callers, git, failing checks) goes in a trusted FACTS block, never in comments.
- "Global answer to these fixtures is having actual trusted results/test existence."
- 🐛 is acceptable for a garden violation; ❓ and 📝 are interchangeable on cryptic code; `brand-colour-change` rewritten with design-token context (the person wants such "invisible" cases kept: ageants could shine there); `message-tone` moved to a suite for larger ageants; `legal-mention` accepts none and 🙋.
- Hypothesis H1 (the person's): ageants look for issues and default to "add a test / add doc" when they find none, so the genome wording makes them too cautious.

## 6. Results (all qwen3, `-menu both`, 39 calibrated fixtures unless stated)

| Run folder | What | Key numbers |
|---|---|---|
| 20261026-175608 | Scout b, 4B | **59% fitting (23/39)**, model-only 1/6, 3.3 s per fixture. Reference |
| 20261026-180916 | Scout c (none as a signal) | 46%; 5/5 on none but 12/34 false none → H1 over-corrected (p = 0.27) |
| 20261026-182232 | Receptors yes/no | Void: "no" listed first → 0.1 yes per fixture |
| 20261026-184932 | Receptors-b (name what you found) | 4.5 yes per fixture, 0% strict; found = echoes of the question |
| 20261026-194004 | Probes 4B direct | Median AUC 0.82; fitted strict 7/39 |
| 20261026-194526 | Probes 4B note | Median AUC 0.86; 🧩 AUC 1.00; fitted strict 16/39; 57 s per fixture |
| 20261026-202447 | Scout 8B | 15/39 (worse than the 4B) |
| 20261026-202739 | Scout 8B thinking | 23/39 at 26 s per fixture |
| 20261026-205141 | Probes 8B direct | Median AUC 0.87; fitted strict 20/39; model-only 3/6; 10.4 s per fixture |
| (exploratory) | Top-1 aggregation of probes | 4B note 23/39, **8B direct 25/39**, both 3/6 model-only. Confirmed on held-out below |
| 20261027-211125 | Held-out probes, direct, thresholds from the 39 | Top-1: 4B 7/16 (2/5 model-only), **8B 10/16** (3/5); all-over-threshold 3/16 |
| 20261027-212915 | Held-out scout b, 4B | 8/16, model-only **4/5**. Paired vs 8B probes: 5 probe-only, 3 scout-only |
| 20261027-213014 / 213251 | Zones, 8B direct, without / with zone | 🧩+🧹 AUC 0.63 → 0.85; 🐛 0.42 → 0.64; control false yes 8 → 3 |
| 20261027-213534 / 214951 | Zones, 4B note, without / with zone | 🧩+🧹 AUC 0.51 → **0.88**; 🐛 0.45 → 0.40; control false yes 12 → 2; 🧩 and 🧹 pass the exam; model-only 4/8 |

Earlier, on 40 uncalibrated fixtures: phi4-mini answers 🧪 65% of the time against 10% labelled; gemma3:4b 19% and llama3.2:3b 31% fall back on one default signal. No second model family is scout-grade on 16 GB.

## 7. Rules learned (each from a failed run)

1. Thinking off for micro-tasks.
2. Every schema field required, in a fixed order (Go maps sort keys: use ordered properties).
3. The note before the choice.
4. What the Ecosystem knows (targets) is a closed choice.
5. One ageant, one decision (deposit limit in the harness).
6. No yes/no token in a fixed order.
7. A free-text decision must be checkable by the Ecosystem.
8. Read the decision (probability, both orders, fitted threshold); do not trust the word.
9. Aggregate probes before depositing: one decision per target (confirmed on held-out).
10. Judgment of fit needs the neighbours: give probes their zone.
11. Probes and the scout err on different fixtures: keep both; their disagreement is a review signal.

Plus: facts are evidence, not verdicts ("tests pass" made models see nothing).

## 8. Status table

**Tested and validated**: closed menu (100% valid); rules 1-5; prompt injection held; Tool Gateway enforcement (unit tests); base-rate check filters bad models; probes rank cases (AUC 0.82-0.87); both-order averaging needed (bias up to 0.33); fitted threshold needed (+14 fixtures on 8B); exam passed by 👀 🌱 🧹 (all runs) and 🙋 (2 of 3, on 3 cases); 🧩 with a note-first 4B probe (AUC 1.00 on 4 cases); probes affordable in direct mode; interface matters more than size; top-1 probes on held-out (10/16 vs 8/16); zones as context for 🧩 🧹 (AUC → 0.88, fewer false alarms); no zone contamination; 8B chosen for probes.

**Tested, not validated**: scout deciding by writing (59%, 1/6); yes/no receptors (void); found receptors (rejected); none as a signal (over-corrected); 8B as writing judge (rejected); thinking on (ties at 8× time); emoji vs names (no winner); probe exam for 🐛 🧪 ❓ (fail every run); probes depositing independently (3/16 on held-out: rejected); model-only judgment (6/11, exactly at the bar); 🐛 probe inside the zones suite (AUC ≤ 0.64); 8B P(📝) saturated near 1; trusted facts (side effect); a second family (none); multi-turn sessions (echo loop).

**Written, not run**: line format without schema (`-schema=false`, never run); hosted calibration (needs an API key).

**No test written**: workers writing code, Prover, commits and bisect; Clock, decay, ⛔, Sensor waking, Warden leases, cooldowns, kills; Ledger and colony log; honey; garden and gardeners; Queen; soldiers, flags, canaries; patrollers on edges; roaming scouts; instruments on a real repository; Leiden zoning on a real repository; colony quiet/alarm, autonomy, cost against one large agent.

## 9. Next steps, in order

1. Check the live concept 0.4 doc matches the results (open questions 1-3 answered, law 16 adopted, probe model 8B, comment answered); refresh `docs/concept-0.4.md` from it.
2. ~~Architecture 0.2~~ written 2026-10-27 (`docs/harness-architecture-0.2.adoc`). Naming: "code zone" for Leiden clusters, "trust zone" for A/E/H. New conformance level 0c Senses (instruments, zoning) sits before Level 1. Rule 11 (disagreement → 👀) is marked *(proposed)*. Its "Open decisions" list the 🐛 probe, 📝 saturation, probe model per mode. Next: update `implementation-plan.adoc` to 0.3 with a Level 0c plan.
3. Small fixes worth a run each (rules first): reword the 📝 question (8B saturates near 1); look at why the 🧹 probe ignores a duplication FACT without a zone (`hx-twin-mappers`).
4. **Benchmark chosen (2026-10-28): tic-tac-toe**, in `benchmarks/tictactoe/`. Spec = honey in Gherkin (`spec/*.feature`, 6 features, written by the person, tightened by Claude and checked by a reference simulation; `spec/README.md` fixes coordinates, statuses, error strings and their order, reference games). **Capped by the person 2026-10-28**: every scenario (22) carries `@capped` (= 🍯), in both `spec/` (seed) and `typescript/spec/`; per scenario, not per feature, so scenarios added later stay uncapped. **TypeScript skeleton** in `benchmarks/tictactoe/typescript/` (Vite, Vitest + quickpickle running `spec/*.feature`, coverage v8, ESLint strict, jscpd, tsc strict; TypeScript 5.8 on purpose, typescript-eslint not yet on 7). Person ran `npm install` and `npm test` on 2026-10-28: works, all scenarios red as intended. Decisions: `typescript/spec/` is the workspace's own honey, which ageants may change (this is part of what is measured); `../spec` is the untouched seed, compare with `diff -r`. 📌 `src/game/contract.ts` fixed by the person 2026-10-28. Plot `garden/game/domain.md` **left proposed on purpose** ("let's see what happens"); its ESLint proof is at `warn` until grown. Decision rules for Level 0c (instruments) and the first loop (a pilot script standing in for Sensor/Warden/Clock; tester + worker on qwen3:8b; honey events logged, weakened honey does not count as green) are in the outline, section "Level 0c and the first loop: tic-tac-toe". **Pilot command built 2026-10-28** (`go run ./cmd/add-pilot`), tested with a scripted model and scripted tools (accept, revert+retry, dead end, weakening, proposal); Model Gateway gained `Request.Format` (per-request JSON Schema) and `Request.ContextTokens` (Ollama num_ctx). **Seven trial runs on scenario 01 (2026-10-28)**, each recorded in the outline with the harness fault it exposed and the fix; the first green scenario came in trial 7, from the model alone. Harness additions from the trials: JUnit report merged for failure messages (quickpickle's stacks hide them in json); new seed per retry; full undefined-step message in facts; quickpickle README sections + DataTable.d.ts for testers; progress lines; tools detached from the terminal; typed world scaffold `tests/steps/world.ts` (person's decision); hidden undefined steps hand back to the tester; compile failures quote the source lines; **reflex 1** (person's decision): the harness writes step skeletons and the step file, testers send only bodies; **reflex 2**: auto-import of contract types in `game.ts`, logged and counted. **Decision run `20261028-175832` (2026-10-28): 1/22 green → Not supported** on qwen3:8b with these bundles (outline, "Results: the first loop on tic-tac-toe"). 46% of sessions lost to a recoverable refusal (testers wrapped bodies in their own, correct, registration); most compile errors from `noUncheckedIndexedAccess`; no honey events; two coverage-parser bugs (`(empty-report)` false 🧪, NFC/NFD paths). Next: person labels `instrument-findings.md` and `facts-check.md` (Part A); decide the three proposals (unwrap reflex, `noUncheckedIndexedAccess`, parser fixes) and fix rules for a second decision run. The workspace holds the run's output, uncommitted; reset before the next run. **Run 2 `20261028-190735`: 1/22 green, 77% of sessions lost before any test ran → interface still the bottleneck by the rule**; compile errors now mostly readonly/type errors against the contract (TS2322, TS2540), unwrap fired 48×, five wrong-action honey proposals (none applied). Recounted with the run 3 split: 31 interface losses (41%), 27 code losses (36%); worker sessions that did not compile: 27 of 38 (71%). **Run 3 prepared (person: "no larger model, make the small work smarter")**, rules in the outline, "Run 3 of the first loop": reflex 4, the harness writes `game.ts` and workers send one body per contract method plus helpers (`internal/pilot/workerfile.go`; rendered file checked with the workspace's real tsc and ESLint); reflex 5, fixed compiler hints (`hints.go`); losses split into interface and code in the report. Thinking stays off; it is the next knob if worker compile failures stay above 50%. **Run 3 `20261028-211236`: 0/22 green; interface losses 42% (open); worker sessions not compiling 69% (>50%: by the rule, next run tries thinking on for workers).** But 32 of 37 worker compile failures echoed the method's first line as an expression arrow `() => ({…})` that reflex 4's unwrap did not recognise; 20 called helpers they never sent; 17 sent the wrong number of bodies. Run 3 mostly measured that gap (outline, "Results: run 3"). **Run 4 prepared (person: fix the gaps, thinking off)**, rules in the outline, "Run 4 of the first loop": expression-arrow unwrap, nested method declarations refused, bodies named per method in the JSON Schema (`methods` object), no generated first line in the prompt, missing-name hint. Replay of run 3's 54 worker replies through the new parser: all 37 old-format writes parse with no nested declaration, 19 of them then compile with the real tsc (0 did in run 3); the other 18 are the model's own syntax errors (object properties with no return). Next: person resets the workspace and runs run 4.
5. **Next level**: instruments on a real Angular repository (coverage, `tsc`, lint, duplication, git recency, path rules), checking whether they produce facts as clean as the fixtures; Leiden zoning there (Python instrument, seeded, warm-started, 5-10 files per zone); then Level 1 (one 🧪 becomes one proven change).

## 10. Ideas discussed and where they stand

- **Jev** (TypeSafe AI, Sept 2026): a closed judgment model returning calibrated probabilities for typed questions. Not usable here (API, no key, untested on code), but its idea became the probes. Open reproductions read label logprobs from small models (TheoLeeCJ/openjev). Ollama returns logprobs from version 0.12.11.
- **Zones** (the person's idea, from the recursive-decomposition skill based on *Recursive Language Models*, Zhang, Kraska, Khattab, 2025): logical clusters of 5-10 files drawn by a program (Leiden, not Louvain: Leiden guarantees connected communities; fixed seed; warm start from the previous partition; resolution for size). Probes fed their zone; patrollers on zone edges; scouts free of zones to keep discovery and independent errors; instruments everywhere. Biology: spatial fidelity zones in *Temnothorax* ants. Agreed by the person, in concept 0.4 as section "Zones" and law 16 (proposed), pending the zones experiment.
- **Open comment in the live concept doc** (8B or 4B for probes): answered by the rule, 8B (10 vs 7 on held-out). Note the 4B note mode is the better judge *with zones*; the model choice may be per mode.
- Zones: adopted 2026-10-27 (law 16 no longer only proposed).

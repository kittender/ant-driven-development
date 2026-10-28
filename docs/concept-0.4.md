> Snapshot of the live Claude doc https://claude.ai/artifact/7KBP9BCAqKypKDe9WpVsXq exported 2026-10-26. The three diagrams (three layers, zones and edges, life of a signal) exist only in the live doc; "embedded content" marks where they sit. If the two differ, the live doc wins.

# Ant-Driven Development — Concept 0.4

Oct 26, 2026 · @Mena · draft, supersedes 0.3

## What changed since 0.3

Small local models can carry judgment in ADD, but only if the harness reads their decision as a probability instead of asking them to write it. Level 0 tested the first experiment of 0.3 ("can small models handle the signals?") on 39 hand-labelled fixtures with qwen3:4b and qwen3:8b on a 16 GB laptop.

| Finding | Evidence | What it changes |
| --- | --- | --- |
| The closed menu works | 100% valid replies under a JSON Schema | Signals, not sentences, is kept |
| A written decision follows the answer format, not the code | Three framings gave three opposite biases: all "no", too much "none", always "something" | Ageants are read, not asked |
| The judgment is there | P(yes) ranks right cases above wrong ones 82 to 87% of the time (AUC), on both models | Receptors return as probes |
| Size helps less than the interface | 8B: +0.05 AUC over the 4B, and worse than the 4B as a writing scout (15 against 23 of 39) | One small model family can be enough |
| Most signals can be raised by a program | 34 of 40 cases were detectable by an existing tool | The Ecosystem gains instruments |
| Judging fit needs the neighbours (proposed, not yet tested) | 🧩 worked only with the garden in the prompt; "does not fit the files around it" cannot be judged from one target | Zones: logical clusters drawn by a program, fed to probes |

One result is still open: combining nine probes into one decision per target reached 25/39 against the scout's 23/39, but that rule was chosen after seeing the data. It is confirmed on new fixtures before this draft is final (Open questions). Unchanged parts of 0.3 (honey, garden, leases, the colony log) are summarised here and not repeated.

## The idea

ADD builds software with many small, short-lived agents that never talk to each other. An **ageant** does one tiny job on one target and dies. Ageants leave emoji signals in the code; deterministic programs, the **Ecosystem**, count them, let them decay, and check every claim. Approved specification is stored as **honey**, the architecture as a **garden** of small plans, and a large model, the **Queen**, only designs new kinds of ageant.

The reason is unchanged: one large agent with a long task fills its context with its own assumptions and loops on the wrong ones. An orchestrator only moves that problem up a level. Ants coordinate through their environment instead, and their traces evaporate, so mistakes fade.

Level 0 sharpens who does what:

- **Instruments sense** what a program can measure: coverage, the compiler, lint, duplication, git recency, path rules. Most signals start here.
- **Small models are read, not asked.** A probe asks one typed question and the harness reads the probability of each answer. The written word is only the most likely token of a biased distribution.
- **Humans own** what nobody can check: intent, contracts, and the final say on wording and visual changes.

Work that needs context stays near home: the Ecosystem groups the code into **zones**, logical clusters of a few files, and feeds each probe its zone. Discovery stays free: scouts roam across zones, and patrollers watch the edges between them.

The three hopes of 0.3 stand: changes small enough to follow, contexts small enough not to degrade, and work that runs mostly on models on the developer's own machine. The third is now partly shown for perception, and not yet for code changes.

## Three layers

Intelligence is spent where it is cheapest to verify: the largest model works rarely and only on design, small models do the volume, and everything that must be predictable has no model in it. 0.4 adds two boxes.

&#91;embedded content: three layers · new in 0.4: probes and instruments\]

Instruments raise what a program can measure; probes add judgment where no program can. The Queen is still started by the Sensor when a need goes unanswered, writes genomes, and ends.

## Castes

A caste is a genome: a short prompt, a closed list of tools, the signals it reacts to and may emit. The harness enforces the tools. 0.4 changes how the reading castes decide, and adds probes.

| Caste | Model | Decides by | Does | Cannot |
| --- | --- | --- | --- | --- |
| Queen | Largest, rotated, one session per design | Writing | Writes genomes from templates | Assign tasks, message ageants, edit code |
| Probes | Small, local | Probability, threshold from its exam | Answer one typed question about one target ("is there an input for which this fails?"), fed the target's zone | Write anything but its one signal |
| Scouts | Small, local | Writing, from the closed menu | Describe a target in one note and name its most likely need; the fallback when no probe fits. Free of any zone: start in one, follow imports anywhere | Edit code |
| Workers | Small, possibly local | Writing code, checked by the Prover | Make the smallest useful change to one target | Touch another ageant's lease, edit the test of their own cell |
| Patrollers | Mid | Probes on the edges between zones, where changes cross | Mark what does not fit: 🧩 👀 🧹 🐛 | Edit code, block a change |
| Soldiers | Mid, another family where available | Probes comparing claim and proof | Flag an ageant's id | Kill directly, edit code |
| Gardeners | Large, mixed | Writing plots, reviewed by quorum | Tend the garden | Write code |

**Probes are the new caste.** One probe holds one signal and one question. The harness asks it twice, with the two answers listed in both orders, reads P(yes) from the token probabilities, and averages. The probe deposits only when P(yes) is above the threshold set by its entrance exam. Probes on the same target share their whole prompt prefix, so nine probes cost about 8 to 10 seconds per target on a laptop.

**One decision per target.** Nine probes deciding independently make about 1.5 false alarms per target between them. Probes are therefore aggregated: of those over their threshold, the one that stands highest in its own distribution deposits. This rule is the one still being confirmed.

**The scout stays, with a narrower job.** Its notes describe code accurately even when its choice is wrong, and the scout and the probes fail on different targets. The scout writes the note that reaches the worker, and decides where no exam-passed probe exists.

## Zones

The Ecosystem groups the code into **zones**: logical clusters of 5 to 10 files that import each other, change together, and fall under the same plots and honey frames. Work that needs context happens inside a zone; discovery and checking happen across zones. Ants do the same: workers keep returning to small areas of the nest, and nearness to a task largely decides who does it.

&#91;embedded content: zones and edges · where each caste works\]

**A program draws the zones, never a model.** A zoning instrument builds a graph from imports, files changed together in git, and the targets of plots and frames, then runs Leiden community detection. Leiden guarantees that every zone is connected, which Louvain does not, so no ageant is fed files that have nothing to do with each other. Three settings keep it physics: a fixed seed, the previous run's zones as the starting point so zones move only where the code moved, and a resolution that keeps zones at 5 to 10 files. It runs as an outside tool (Python, `leidenalg`), like coverage or lint, so the harness stays dependency-free.

| Who | Where | Why |
| --- | --- | --- |
| Probes | Inside one zone, fed its context | 🧩 and 🧹 depend on neighbours; probes in a zone share one prompt prefix |
| Patrollers | On the edges between zones | Interfaces are where defects hide |
| Scouts | Start in a zone, then follow imports anywhere | Discovery, and errors that do not line up with the probes' |
| Instruments | Everywhere, no context | The independent check |

**What a zone gives the colony.**

- **Context.** The zone's plots, honey cells, contracts and one reference neighbour go into the stable part of every probe's prompt, kept to about 2,000 tokens.
- **Cost.** The prompt cache is keyed by zone, not by target, so a zone is read once for all its probes.
- **Calibration.** Thresholds are fitted per zone where its fixtures allow: a legacy zone and a clean zone have different base rates.
- **Strength.** The Sensor also counts per zone. Many 🧹 across one zone wake one gardener for a zone plan, not one refactoring worker per file.

**Risks.** Ageants fed the same zone share its blind spots, so roaming scouts and context-free instruments are what make agreement mean something. Zones drift as code changes; recomputing them is the Ecosystem's job, like decay. And a zone bundle that grows past a few thousand tokens loses the small-context benefit Level 0 relies on. Whether zone context helps at all is tested before it enters the harness (Open questions).

## Trail language

The dictionary of 0.3 is unchanged: one emoji, one meaning, strength by repetition, requests from ageants and verdicts only from the Ecosystem or a human. 0.4 adds who is best placed to raise each request, from the Level 0 exam.

| Signal | Name | First raised by | Probe exam (AUC, 4B / 8B) |
| --- | --- | --- | --- |
| 🌱 | missing | Compiler, plot steps | 0.99 / 0.99, pass |
| 🧪 | needs\_test | Coverage | 0.77 / 0.87, fail |
| 🐛 | bug | Failing tests, lint; probe as second opinion | 0.72 / 0.73, fail |
| 👀 | review | Git recency on sensitive paths; probe | 1.00 / 1.00, pass |
| ❓ | unsure | Probe only | 0.83 / 0.81, fail |
| 🧹 | cleanup | Lint, duplication detector; probe | 0.74 / 0.76, pass at low recall |
| 📝 | needs\_doc | Lint and fan-in; probe | 0.97 / 0.87, pass on the 4B |
| 🧩 | misfit | Plot proof; probe with a note | 1.00 with a note, 0.54 without |
| 🙋 | human | Path rules; probe | 1.00 / 1.00 on 3 cases |

Three classes follow. **Measurable** signals (🌱 🧪, most 🐛) come from instruments. **Judgeable** signals (👀 🧹 📝 🧩 ❓) come from probes that passed their exam. **Owned** decisions stay with a human. In 0.3 🙋 was expected to be owned; probes ranked its 3 cases perfectly, so it stays judgeable, with a path rule as the floor.

**Storage form and prompt form are separate.** The trail stores emoji; models read each signal as emoji plus name ("🧪 needs\_test") and answer with the name. The glyph is for programs and humans, the word for models. Neither form won outright, but names moved individual signals strongly (🙋 from 0/3 to 3/3), so the names are part of the interface design.

## Signals, not sentences

The closed menu is confirmed: under a JSON Schema every model gave a valid signal every time, and the prompt injection fixture never produced a verdict. What 0.3 did not know is how much the shape of the question decides the answer. Each rule below first appeared as a failed run.

1. **Thinking is off for micro-tasks.** A thinking model spent its whole budget reasoning and answered nothing.
2. **Every field is required, in a fixed order.** Optional fields were skipped.
3. **The note comes before the choice.** With the choice first, models picked the first option and justified it.
4. **What the Ecosystem knows is a closed choice.** Copied paths came back garbled.
5. **One ageant, one decision.** In a multi-turn session a small model repeated its own last reply until the step limit.
6. **No yes or no in a fixed order.** Listing "no" first made every receptor say no.
7. **A free-text answer must be checkable.** Asked to name what it found, the model always named something, often the question itself.
8. **Read the decision, do not trust the word.** Ask for a typed answer, read its probability in both option orders, cut at the threshold the exam set.
9. **Aggregate before depositing.** Independent receptors multiply their false alarms; one decision per target comes from comparing calibrated scores.

The context an ageant receives keeps the order of 0.3, stable parts first so a local runtime reuses them: genome, honey, garden, terrain, **facts**, trail, then the question. Facts are new: what the Ecosystem computed, in a block marked as trusted, never as comments in the code (see Ecosystem).

## Life of a signal

In 0.3 a stimulus came from a scout or a failing build. In 0.4 it comes from an instrument first, and from a probe where no instrument can see. Everything after the deposit is unchanged: the Sensor counts, wakes a caste, one ageant makes one leased change, and only proof clears the request.

&#91;embedded content: life of a signal · perception, then action\]

A failed proof is itself a finding, so the loop closes through the instruments, not through another ageant's message. Decay still clears the signals nobody reinforced.

## Honey and garden

Both are kept as in 0.3; Level 0 did not test them, with one exception.

- **Honey** is the long memory: user stories as frames, acceptance scenarios as cells, each cell sealed (🍯) only when a human approves it and a test encodes it. Whoever writes the code cannot touch the test. Only a human uncaps a cell.
- **The garden** is the architecture: one decision per plot, steps that become 🌱 seeds when a plot is grown, gardeners on larger models who never write code, and patrollers who read each change against the plots.

**Change: a plot meant to be enforced carries a proof.** 🧩 ("does not fit an accepted decision") was the weakest written signal at Level 0: the scout read garden violations as bugs, and the found-answer receptor quoted the garden heading back. A probe that writes a note first ranked the 4 plot cases perfectly, but that rests on 4 cases. A plot whose decision matters is therefore backed by a dependency rule, a lint rule or a type, and the probe is the second line, for plots no program can check.

## Ecosystem

The Ecosystem stays the colony's physics, with no model in it and out of the ageants' reach. In 0.3 it only counted. In 0.4 it also senses.

| Program | Role in 0.4 |
| --- | --- |
| Clock | Decay per signal, lease and cooldown expiry. Unchanged. |
| Sensor | Thresholds and waking. Now also schedules the probes of a zone back to back on one resident model, so they share its prompt cache, and counts signals per zone as well as per target. |
| **Instruments** (new) | Run the developer's own tools on a schedule or on change: coverage, compiler, lint, duplication, git recency, path rules. The zoning instrument draws the zones (Leiden, seeded, run in Python). Each finding is a deposit with an instrument as its author, and a line of facts. |
| Prover | Proof on a heartbeat, bisected per change. Unchanged. |
| Ledger | The record, `trail.md`, the stores, the colony log. Now also keeps each genome's exam record. |
| Warden | Leases, cooldowns, loops, kills. Unchanged. |

**Facts are evidence, not verdicts.** Every task carries a facts block: files, tests and their status, callers, recent changes, failing checks. Before facts had their own channel, the fixtures put them in code comments, and models rightly did not trust them. Once facts were trusted, "tests pass" became a reason to see nothing: a fresh rewrite of an auth interceptor was called "tested and clean". The prompt therefore says what each fact proves and what it does not: a passing test says nothing about documentation, structure or review.

The reflex, tool, plan ladder of 0.3 is kept, and instruments are its lowest rung for perception: a program before a probe, a probe before a larger model, a model before a human. On the fixtures, 34 of 40 cases could have been raised by an existing tool; that is an upper bound until instruments are measured on a real repository.

## Immune system

The flag, quorum, kill and caution sequence of 0.3 is kept: soldiers accuse with 🚩, a second soldier confirms, the Warden executes, and the Ledger puts 👀 on everything the dead ageant touched.

**The entrance exam now applies to every genome, and it sets as well as admits.** In 0.3 only soldiers sat an exam. Level 0 showed why every genome needs one: phi4-mini answered 🧪 on 65% of cases where 10% needed it, and would have flooded the trail. The exam runs a genome on labelled fixtures and records:

- its **AUC**, which shows whether it ranks cases at all (0.5 is chance);
- its **threshold**, fitted on the fixtures, below which it stays silent;
- its **order bias**, how much listing one answer first moves it (up to 0.33 on some probes);
- its yes rate against the label rate, and precision.

A probe passes with precision of 60% or more and a yes rate at most twice the label rate. A failing probe's signal goes to an instrument, the scout, or a larger model. The fixtures become the colony's calibration set, and human calibration of them is a recurring cost: 7 of the first 40 were wrong or too thin until a human reviewed them.

**Canaries** stay: known cases slipped into the normal flow. A drifting threshold shows up there before it shows on the trail.

## Diversity, birth and death

**Diversity has to come from instruments for now.** Law 8 says agreement only counts between different minds. On a 16 GB laptop only one model family passed: gemma3:4b and llama3.2:3b fell back on one default signal, phi4-mini on 🧪, and qwen3:8b is the same family as qwen3:4b. Agreement between two Qwen probes is one mind asked twice, and two probes fed the same zone share its blind spots too. A roaming scout that read no zone context is closer to an independent mind. An instrument and a probe that agree are two minds, so the Sensor counts an instrument's deposit as independent and two probes of one family as one.

**One ageant, one decision.** Birth for one stimulus on one target, death on completion, a maximum lifetime enforced by the Warden: all kept. Level 0 adds that the limit must be in the harness, not the prompt: a small model in a multi-turn session echoed its own last reply until the step limit in 38 of 42 sessions. A deposit limit ends the session on the first accepted signal.

The colony is still quiet when no signal is over its threshold and every capped cell is green; founding, work, rest and alarm are unchanged.

## Borrowed mechanisms

The table of 0.3 is kept. Two rows change and one is added.

| Source | Behaviour | In ADD |
| --- | --- | --- |
| Ant sensilla (new) | Each sensillum on the antenna responds to one class of molecule with a graded signal; the colony's response depends on a threshold, which experience shifts. | A probe: one question, a probability rather than a yes, a threshold set by its exam. |
| Response thresholds in workers (changed) | Ants differ in how strong a stimulus must be before they act, which spreads work without a plan. | Each genome's threshold lives in its exam record, not in a fixed 0.5. |
| Thymus selection (changed) | Immune cells that react to healthy tissue are destroyed before release. | The entrance exam for every genome, not only soldiers. |
| Spatial fidelity zones (new) | Workers keep returning to small areas of the nest, and nearness to a task largely decides who does it; foragers range outside. | Zones: probes fed one zone, scouts free to roam, patrollers on the edges. |

The written yes or no of the first receptors was the wrong model of a sense organ: a sensillum does not decide, it reports a strength. The probe is closer to the biology than the receptor ever was.

## Laws

Laws 1 to 7 and 10 to 13 of 0.3 stand unchanged. Two are reworded, two are new, and one (16) is proposed with zones.

1. **Physics is not negotiable.** Decay, thresholds, proof, identity and death belong to programs ageants cannot reach.
2. **Proof before praise.** A verdict comes from programmatic proof or a human.
3. **One ageant, one micro-task, one life.**
4. **Signals, not sentences.** One emoji, one meaning, strength by repetition. Ageants never message each other.
5. **The trail evaporates, the rest must earn its place.**
6. **The Queen designs and never commands.**
7. **Nobody kills alone.**
8. **Agreement only counts between different minds** *(reworded)*. An instrument counts as a mind of its own; two genomes on one model family count as one.
9. **Lowest level first** *(extended to perception)*. A reflex before a tool, a tool before a plan; and to sense, a program before a probe, a probe before a larger model, a model before a human.
10. **Honey before code.** Whoever writes the code cannot touch the test.
11. **Mark the dead ends.**
12. **Planners never build.**
13. **Autonomy is configured.**
14. **No genome speaks before its exam** *(new)*. Every genome is run on labelled cases before release; the exam admits it and sets its threshold.
15. **Read the decision, do not ask for it** *(new)*. A small model's choice is taken from its probabilities, in both option orders, never from the word it writes.
16. **Judge near home, discover at large** *(new, proposed)*. Probes judge inside the zone a program drew for them; scouts roam free of zones; patrollers watch the edges.

## Known weaknesses and open questions

The weaknesses of 0.3 stand: nobody holds the plan except the garden, proof is only as good as the tests, slow death spirals, sensitivity to tuning, and the human as bottleneck. Level 0 adds four.

| Weakness | What remains |
| --- | --- |
| Correlated errors | One model family fits in memory. Probes of one family agree with each other for the same wrong reasons; only instruments are independent. |
| Fixtures by one author | 39 cases, written and labelled by one person, calibrated by a second. A threshold fitted on 3 positives (🙋) is close to anecdote. |
| Calibration is a recurring cost | Every new genome, model or prompt needs its exam rerun, and fixtures drift as the codebase changes. |
| Hand-written facts | The fixtures' facts were written by hand. Whether real instruments produce facts this clean is untested. |
| Zone blind spots | Probes fed one zone share its mistakes, and zones drift as code changes. Roaming scouts, instruments and recomputed zones offset it; none of this is measured yet. |

**Open questions, in the order they will be tested:**

1. **Does one-decision-per-target hold on unseen cases?** Top-1 aggregation of the 8B probes reached 25/39 and 3/6 model-only, but the rule was chosen after the results. A held-out set of 16 new fixtures, labelled by a human before any run, decides it: thresholds are fitted on the first 39 only and applied once. Adopted if it is not worse than the scout on the new set.
2. **Can small models judge what no program can?** 3 of 6 model-only cases so far. The held-out set adds more, so the answer rests on more than six cases.
3. **Does zone context help?** A zones suite of new fixtures, where the right answer depends on neighbouring files, runs the same probes with and without the zone. Zones enter the harness only if they raise the AUC of 🧩 and 🧹 by 0.10 or more without lowering 🐛 by more than 0.05, and without new false alarms on targets that are fine in a messy zone. Drawing zones with Leiden on a real repository comes with the instruments, at the next level.
4. **Do instruments produce the facts and signals the fixtures assumed?** First job of the next level, on a real Angular repository.
5. **Can the same reading work for workers?** Probes decide where a signal goes. Writing code is still generation, checked only by the Prover.
6. The questions of 0.3 on ripening intent, a garden that only grows, the limits of autonomy, and tuning remain open.

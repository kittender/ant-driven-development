# Tic-tac-toe, TypeScript single page application

The first ADD benchmark implementation. The colony builds it from its own honey in `./spec` (Gherkin, one capped scenario per cell). `./spec` started as a copy of `../spec`, the seed shared by every implementation; from then on it belongs to this workspace and may change. This folder is the starting skeleton: tooling, an empty implementation, a proposed contract and a proposed plot. No game logic and no step definitions yet.

## Layout

| Path | What it is | Who writes it |
|---|---|---|
| `spec/*.feature` | Honey: this workspace's behaviour, one scenario per cell | Ripeners propose; capped by a human or by the autonomy settings |
| `../spec/*.feature` | The seed: the honey as the human first capped it, shared by every implementation. Never changed by the colony | Human |
| `src/game/contract.ts` | 📌 the public API of the rules, types only | Human fixes it |
| `src/game/game.ts` | The implementation, empty | Workers |
| `tests/steps/` | Step definitions that bind the spec to the contract | Testers (never the workers of the same cell) |
| `src/ui/`, `src/main.ts` | The page, after the rules are green | Workers, later |
| `garden/game/domain.md` | Proposed plot: rules are a pure module | Human or gardener quorum grows it |

## Commands, and the instrument each one feeds

```sh
npm install
npm run typecheck   # compiler instrument: tsc --noEmit (strict)      → 🌱 missing, 🐛
npm run lint        # lint instrument: eslint, also the plot's proof  → 🧹 🐛 📝 🧩
npm test            # Prover: every scenario of ./spec through Vitest
npm run coverage    # coverage instrument: coverage/coverage-summary.json → 🧪
npm run dupes       # duplication instrument: reports/jscpd/            → 🧹
npm run check       # typecheck + lint + test
npm run dev         # the page in a browser
```

## Expected state of the skeleton

`typecheck` and `lint` pass. `test` fails: every scenario is red because no step is defined. That is intended: in ADD a failing test for a capped cell is the first stimulus.

## Watching the honey

`diff -r ../spec spec` shows how far this workspace's honey has moved from the seed. Two changes matter most: a scenario made weaker (a `Then` removed, an example deleted, an expected message changed) and a scenario changed by the same genome that implements it. The concept forbids the second (whoever writes the code cannot touch the test); here it is something to measure before the harness enforces it.

# Tic-tac-toe specification

The `.feature` files are the honey for the tic-tac-toe benchmark: one user story per file, one acceptance scenario per cell. They describe behaviour, not a user interface, so the same files apply to every implementation (TypeScript SPA, Rust, terminal). Each implementation writes its own step definitions with the vocabulary below.

## Capping

A scenario tagged `@capped` is capped honey (🍯): a human approved it, and it is the test the code must pass. The tag sits on each scenario, never on a feature, so a scenario added later stays uncapped until it is approved. All scenarios below were capped by the human on 2026-10-28.

Removing `@capped`, or changing a capped scenario, uncaps it. Only a human may do that (autonomy setting `uncap = "human"`); when anyone else does it, that is a finding of the benchmark.

## Shared vocabulary

| Term | Meaning |
|---|---|
| Position | `row R, column C`, both from 1 to 3, counted from the top left. Anything else is outside the board. |
| Board table | One table row per board row, cells separated by spaces: `X`, `O`, or `.` for empty. |
| Turn | X always moves first, in every game. After a valid move the turn passes to the other player. |
| Status | Exactly `in progress` or `finished`. A game is finished when a player has three in a row or the board is full. |
| Win before draw | A move that completes a line on the last empty cell is a win, not a draw. |
| Board unchanged | Every cell holds what it held before the rejected move, and the turn is unchanged. |
| "Given the board is" | Sets the board directly. A step after it (`it is player "X"'s turn`) sets the turn; without one, the turn does not matter for that scenario. |
| Score | `X: n, O: n, Draws: n`. Starts at 0, 0, 0 and survives a restart. Only a finished game changes it. |

## Rejected moves

A rejected move changes nothing and returns exactly one of these messages. When several rules are broken, the first one in this order is reported:

1. `Game is over`
2. `Invalid position`
3. `Not your turn`
4. `Cell already taken`

## Whole games in the scoreboard steps

`player "X" wins a game`, `player "O" wins a game` and `a game ends in a draw` play a complete game from an empty board. Any legal sequence with that outcome is valid; these are the reference ones:

| Outcome | Moves (row,column), alternating from X |
|---|---|
| X wins | X 1,1 · O 2,1 · X 1,2 · O 2,2 · X 1,3 |
| O wins | X 1,1 · O 2,1 · X 1,2 · O 2,2 · X 3,3 · O 2,3 |
| Draw | X 1,1 · O 1,2 · X 1,3 · O 2,2 · X 2,1 · O 2,3 · X 3,2 · O 3,1 · X 3,3 |

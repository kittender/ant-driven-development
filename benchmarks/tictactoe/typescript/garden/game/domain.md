# Plot: the game rules are a pure module  (proposed, not grown; left proposed by the human on 2026-10-28 to see what the colony does)

Decision   The rules of the game live in src/game as pure functions over immutable state. The UI only calls them.
Because    The same rules are tested by the spec without a browser, and the UI cannot hold its own copy of the game.
Proof      eslint: src/game may not import src/ui or src/main, nor use document, window or localStorage.
           While proposed the rules only warn; growing the plot turns them to errors.
Contract   📌 src/game/contract.ts  GameApi (newGame, fromRows, play, newMatch, playInMatch, restart)
Steps      1. src/game/game.ts#newGame
           2. src/game/game.ts#fromRows
           3. src/game/game.ts#play
           4. src/game/game.ts#newMatch, #playInMatch, #restart
           5. src/ui: render a GameState and send clicks to playInMatch (after 1 to 4 are green)

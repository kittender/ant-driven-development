package pilot

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kittender/add/internal/instruments"
	"kittender/add/internal/model"
)

// The GameApi part of benchmarks/tictactoe/typescript/src/game/contract.ts.
const realAPI = `export interface GameApi {
  /** An empty board, X to move, status "in progress". */
  newGame(): GameState;
  /** Sets a board directly from spec rows such as "X O .", with the given turn (X if omitted). */
  fromRows(rows: readonly string[], turn?: Player): GameState;
  /** Plays one move. A rejected move returns the state unchanged. Never throws. */
  play(state: GameState, player: Player, row: number, column: number): MoveResult;

  /** A new match: a new game and a score of 0, 0, 0. */
  newMatch(): Match;
  /** Plays a move in the match's game; when the move finishes the game, the score counts it once. */
  playInMatch(match: Match, player: Player, row: number, column: number): { readonly match: Match; readonly result: MoveResult };
  /** A new game in the same match; the score is unchanged. */
  restart(match: Match): Match;
}
`

func TestContractMethods(t *testing.T) {
	ms := ContractMethods("export type Player = 'X' | 'O';\n\n" + realAPI)
	var names []string
	for _, m := range ms {
		names = append(names, m.Name+"("+strings.Join(m.Params, ",")+")")
	}
	want := "newGame() fromRows(rows,turn) play(state,player,row,column) newMatch() playInMatch(match,player,row,column) restart(match)"
	if strings.Join(names, " ") != want {
		t.Fatalf("%s", strings.Join(names, " "))
	}
	if ContractMethods("export interface GameApi {}") != nil {
		t.Fatal("no methods: workers write the whole file")
	}
}

func TestGameFileRoundTrip(t *testing.T) {
	ms := ContractMethods(realAPI)
	gf := GameFile{Helpers: "function other(p: Player): Player {\n  return p === 'X' ? 'O' : 'X';\n}", Bodies: map[string]string{
		"newGame":  "  return fromRows(['. . .', '. . .', '. . .']);",
		"fromRows": "  const x = {\n  a: 1,\n};\n  return { board: rows.map(() => []), status: 'in progress', winner: null, draw: false };",
	}}
	src, dropped := RenderGameFile(gf, ms)
	for _, want := range []string{
		"const newGame: GameApi['newGame'] = () => {",
		"const fromRows: GameApi['fromRows'] = (rows) => {", // turn is unused: left out
		"const play: GameApi['play'] = () => {\n  throw new Error('not implemented');\n};",
		"export const game: GameApi = { newGame, fromRows, play, newMatch, playInMatch, restart };",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("missing %q in\n%s", want, src)
		}
	}
	if len(dropped) != 1 || dropped[0] != "fromRows: turn" {
		t.Fatalf("dropped %v", dropped)
	}
	back := ParseGameFile(src, ms)
	if back.Helpers != gf.Helpers || len(back.Bodies) != 2 || back.Bodies["fromRows"] != gf.Bodies["fromRows"] || back.Bodies["newGame"] != gf.Bodies["newGame"] {
		t.Fatalf("round trip:\n%#v", back)
	}
	// The human's stub reads as nothing written yet.
	if st := ParseGameFile("const notYet = () => {};\nexport const game: GameApi = { newGame: notYet };", ms); st.Helpers != "" || len(st.Bodies) != 0 {
		t.Fatalf("%+v", st)
	}
}

func TestCleanWorkerBody(t *testing.T) {
	for _, raw := range []string{
		"newGame(): GameState {\n  return x;\n},",
		"const newGame: GameApi['newGame'] = () => {\n  return x;\n};",
		"```ts\nfunction newGame(): GameState {\n  return x;\n}\n```",
	} {
		b, un, err := CleanWorkerBody(raw, "newGame")
		if err != nil || !un || b != "  return x;" {
			t.Errorf("%q → %q %v %v", raw, b, un, err)
		}
	}
	if b, un, err := CleanWorkerBody("  return x;", "newGame"); err != nil || un || b != "  return x;" {
		t.Errorf("plain body: %q %v %v", b, un, err)
	}
	if _, _, err := CleanWorkerBody("import type { X } from './contract';\nreturn 1;", "newGame"); err == nil {
		t.Error("an import inside a body must be refused")
	}
	if h, err := CleanHelpers("import type { Player } from './contract';\nfunction f(): void {}", ContractMethods(realAPI)); err != nil || h != "function f(): void {}" {
		t.Errorf("helpers %q %v", h, err)
	}
	if _, err := CleanHelpers("const play = 1;", ContractMethods(realAPI)); err == nil {
		t.Error("a helper redeclaring a method must be refused")
	}
}

func TestHints(t *testing.T) {
	fs := []instruments.Finding{
		{Rule: "TS2540", Message: "Cannot assign to 'score' because it is a read-only property."},
		{Rule: "TS2540", Message: "again"},
		{Rule: "TS2322", Message: "Type 'string' is not assignable to type 'Status'."},
	}
	text, ids := Hints(fs)
	if strings.Join(ids, ",") != "readonly,literal" || strings.Count(text, "Hint:") != 2 {
		t.Fatalf("%v %q", ids, text)
	}
}

// A worker sends bodies; the harness writes game.ts, the retry sees a hint,
// and the loss is counted as code, not interface.
func TestPilotWorkerBodies(t *testing.T) {
	ws, out := setup(t)
	writeFile(t, ws, "src/game/contract.ts", "export type Player = 'X' | 'O';\n"+realAPI)
	tools := fakeTools(t)
	run := func(ctx context.Context, dir string, argv ...string) instruments.Output {
		if strings.Contains(strings.Join(argv, " "), "tsc") {
			b, _ := os.ReadFile(filepath.Join(dir, "src/game/game.ts"))
			if strings.Contains(string(b), "MUTATE") {
				return instruments.Output{Exit: 2, Stdout: "src/game/game.ts(9,3): error TS2540: Cannot assign to 'turn' because it is a read-only property."}
			}
		}
		return tools(ctx, dir, argv...)
	}
	var retry string
	fake := &model.Fake{Reply: func(req model.Request, call int) (string, error) {
		u := req.Messages[0].Content
		if strings.Contains(u, "Write the step definitions") {
			return steps("steps"), nil
		}
		bodies := make([]string, 6)
		if call == 1 {
			bodies[0] = "  state.turn = 'X'; // MUTATE"
		} else {
			retry = u
			bodies[0] = "newGame(): GameState {\n  return fromRows([]); // IMPLEMENTED\n}"
		}
		r, _ := json.Marshal(Reply{Note: "n", Action: Write, Path: gameFile, Bodies: bodies})
		return string(r), nil
	}}
	p := New(Config{Workspace: ws, Model: "fake:w", Gateway: model.NewGateway(fake), Run: run, Commands: instruments.TypeScript(), Out: out, MaxTokens: 10, ContextTokens: 10})
	if err := p.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !p.runs[0].CountedOK {
		t.Fatalf("%+v", p.runs[0])
	}
	if !strings.Contains(retry, "Hint: The contract's types are readonly") || !strings.Contains(retry, "- newGame (not implemented yet). Contract: newGame(): GameState;") {
		t.Fatalf("retry bundle:\n%s", retry)
	}
	if p.attempts[1].Lost != "code" || strings.Join(p.attempts[1].Hints, ",") != "readonly" {
		t.Fatalf("attempt 2 %+v", p.attempts[1])
	}
	if !strings.Contains(strings.Join(p.attempts[2].Reflexes, ";"), "unwrapped body 1") {
		t.Fatalf("attempt 3 %+v", p.attempts[2])
	}
	src, _, _ := (Workspace{ws}).Read(gameFile)
	if !strings.Contains(src, "const newGame: GameApi['newGame'] = () => {\n  return fromRows([]); // IMPLEMENTED\n};") {
		t.Fatalf("game.ts:\n%s", src)
	}
	rep, _ := os.ReadFile(filepath.Join(out, "report.md"))
	for _, want := range []string{"code losses (compile errors in src/) | 1", "Worker sessions that do not compile | 1 of 2 (50%)", "readonly 1"} {
		if !strings.Contains(string(rep), want) {
			t.Fatalf("report lacks %q:\n%s", want, rep)
		}
	}
}

func TestCleanWorkerBodyRunThreeEchoes(t *testing.T) {
	// The two forms run 3's workers sent, from sessions 2 and 3.
	for _, raw := range []string{
		"  const newGame: GameApi['newGame'] = () => ({\n    board: [],\n    turn: 'X'\n  });",
		"const restart: GameApi['restart'] = (match) => ({\n  ...match,\n  game: newGame()\n});",
	} {
		name := "newGame"
		if strings.Contains(raw, "restart") {
			name = "restart"
		}
		b, un, err := CleanWorkerBody(raw, name)
		if err != nil || !un || !strings.HasPrefix(b, "  return ({") || !strings.HasSuffix(b, "});") {
			t.Errorf("%q → %q %v %v", raw, b, un, err)
		}
	}
	if b, _, _ := CleanWorkerBody("const play: GameApi['play'] = (state) => state.turn === 'X' ? a : b;", "play"); b != "  return state.turn === 'X' ? a : b;" {
		t.Errorf("one-line arrow → %q", b)
	}
	// A declaration of another method nested in a body is refused.
	if _, _, err := CleanWorkerBody("  const x = 1;\n  const newMatch: GameApi['newMatch'] = () => x;", "newGame"); err == nil {
		t.Error("a nested method declaration must be refused")
	}
}

func TestNamedMethodsReply(t *testing.T) {
	ms := ContractMethods(realAPI)
	r, _ := json.Marshal(Reply{Note: "n", Action: Write, Path: gameFile, Methods: map[string]string{"restart": "  return match;"}})
	got, err := ParseReply(string(r), []string{gameFile, "spec/x.feature"}, "spec/", "", nil, ms...)
	if err != nil || len(got.Bodies) != 6 || got.Bodies[5] != "  return match;" || got.Bodies[0] != "" {
		t.Fatalf("%v %q", err, got.Bodies)
	}
	sch, _ := json.Marshal(replySchema([]string{gameFile}, false, ms))
	if !strings.Contains(string(sch), `"methods"`) || !strings.Contains(string(sch), `"playInMatch"`) || strings.Contains(string(sch), `"bodies"`) {
		t.Fatalf("schema %s", sch)
	}
}

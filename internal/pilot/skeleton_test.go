package pilot

import (
	"strings"
	"testing"

	"kittender/add/internal/honey"
)

func skel(t *testing.T, key string, defined map[string]string) []Skeleton {
	t.Helper()
	snap, err := honey.Load("testdata/spec")
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := snap.Scenarios[key]
	if !ok {
		t.Fatalf("no scenario %q", key)
	}
	f, _ := snap.Feature(sc.File)
	return Skeletons(f, sc, defined)
}

func codes(sks []Skeleton) string {
	var c []string
	for _, s := range sks {
		c = append(c, strings.SplitN(s.Code, "\n", 2)[0])
	}
	return strings.Join(c, "\n")
}

func TestSkeletonApostropheAndStatus(t *testing.T) {
	got := codes(skel(t, "01_start_game.feature > A new game has an empty board and X to move", nil))
	want := strings.Join([]string{
		`When("I start a new game", (world) => {`,
		`Then("the board should have {int} empty cells", (world, n: number) => {`,
		`Then("it should be player {string}'s turn", (world, text: string) => {`,
		`Then("the game status should be {string}", (world, text: string) => {`,
	}, "\n")
	if got != want {
		t.Fatalf("got\n%s\nwant\n%s", got, want)
	}
}

func TestSkeletonBackgroundOutlineAndDefined(t *testing.T) {
	sks := skel(t, "02_make_move.feature > Cannot play outside the board", map[string]string{"a new game has started": "tests/steps/01_start_game.steps.ts"})
	got := codes(sks)
	if !strings.Contains(got, `Given("a new game has started", (world) => {`) || sks[0].DefinedIn == "" {
		t.Fatalf("background step first, marked defined:\n%s", got)
	}
	if !strings.Contains(got, `When("player {string} plays at row {int}, column {int}", (world, text: string, row: number, col: number) => {`) {
		t.Fatalf("outline placeholders:\n%s", got)
	}
	r := RenderSkeletons(sks)
	if !strings.Contains(r, "already defined in tests/steps/01_start_game.steps.ts") {
		t.Fatal(r)
	}
}

func TestSkeletonDataTableAndScore(t *testing.T) {
	got := codes(skel(t, "04_draw_game.feature > Board fills up with no three in a row", nil))
	if !strings.Contains(got, `Given("the board is:", (world, table: DataTable) => {`) {
		t.Fatalf("data table:\n%s", got)
	}
	got = codes(skel(t, "06_scoreboard.feature > Scores add up over several games", nil))
	if !strings.Contains(got, `Given("the score is X: {int}, O: {int}, Draws: {int}", (world, n: number, n2: number, n3: number) => {`) {
		t.Fatalf("score:\n%s", got)
	}
	if strings.Count(got, `When("player {string} wins a game"`) != 1 {
		t.Fatalf("each expression once:\n%s", got)
	}
}

func TestRegisteredExpressions(t *testing.T) {
	src := "Given(\"it should be player {string}'s turn\", (w) => {});\nWhen('I start a new game', (w) => {});\nThen('x \\'y\\'', (w) => {})"
	got := strings.Join(RegisteredExpressions(src), "|")
	if got != "it should be player {string}'s turn|I start a new game|x 'y'" {
		t.Fatal(got)
	}
}

func TestEscapeExpr(t *testing.T) {
	if escapeExpr("a (b) c/d") != `a \(b\) c\/d` {
		t.Fatal(escapeExpr("a (b) c/d"))
	}
}

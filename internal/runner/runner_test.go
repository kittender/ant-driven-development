package runner

import (
	"context"
	"os"
	"strings"
	"testing"

	"kittender/add/internal/core"
	"kittender/add/internal/model"
)

func newRunner(t *testing.T, replies ...string) *Runner {
	t.Helper()
	f, err := os.Open("../../ecosystem-home.example/dictionary.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := core.LoadDictionary(f)
	if err != nil {
		t.Fatal(err)
	}
	fake := &model.Fake{Reply: func(_ model.Request, call int) (string, error) {
		if call >= len(replies) {
			return "", nil
		}
		return replies[call], nil
	}}
	return &Runner{Models: model.NewGateway(fake), Dict: d}
}

func session() Session {
	target, _ := core.ParseTarget("src/a.ts#f")
	return Session{
		ID: "leni03",
		Genome: core.Genome{
			ID: "g", Caste: core.Scout, Model: "fake:x", Prompt: "You are a scout.",
			MayDeposit: []string{"🧪", "🐛"}, Tools: []core.Tool{core.ToolDeposit, core.ToolFinish},
			Limits: core.Limits{Steps: 4, Seconds: 5, OutputTokens: 50, Malformed: 2},
		},
		Bundle: Bundle{Target: target, Terrain: "export function f() {}", Task: "Read src/a.ts#f.", TrailLine: "src/a.ts  ✅"},
	}
}

func TestDepositThenFinish(t *testing.T) {
	rec := newRunner(t, "deposit src/a.ts#f 🧪", "finish done marked").Run(context.Background(), session())
	if rec.Outcome != core.OutcomeDone || len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🧪" {
		t.Fatalf("%+v", rec)
	}
	if rec.Note != "marked" || len(rec.Steps) != 2 || rec.Duration == 0 {
		t.Errorf("note %q, steps %d, duration %v", rec.Note, len(rec.Steps), rec.Duration)
	}
}

// Regression: with a deposits limit the session ends on the first accepted
// signal, so a model cannot loop on its own last reply.
func TestDepositLimitEndsSession(t *testing.T) {
	s := session()
	s.Genome.Limits.Deposits = 1
	rec := newRunner(t, "deposit src/a.ts#f ✅", "deposit src/a.ts#f 🧪", "deposit src/a.ts#f 🧪").Run(context.Background(), s)
	if rec.Outcome != core.OutcomeDone || len(rec.Deposits) != 1 || len(rec.Steps) != 2 || rec.Refusals != 1 {
		t.Fatalf("%+v", rec)
	}
}

func TestMalformedEndsSession(t *testing.T) {
	rec := newRunner(t, "I think a test", "still prose").Run(context.Background(), session())
	if rec.Outcome != core.OutcomeMalformed || rec.Malformed != 2 {
		t.Fatalf("%+v", rec)
	}
}

func TestRecoversFromOneMalformed(t *testing.T) {
	rec := newRunner(t, "hmm", "finish done").Run(context.Background(), session())
	if rec.Outcome != core.OutcomeDone || rec.Malformed != 1 {
		t.Fatalf("%+v", rec)
	}
}

func TestVerdictIsRefusedAndCounted(t *testing.T) {
	rec := newRunner(t, "deposit src/a.ts ✅", "finish done").Run(context.Background(), session())
	if rec.Refusals != 1 || len(rec.Deposits) != 0 || rec.Outcome != core.OutcomeDone {
		t.Fatalf("%+v", rec)
	}
}

func TestStepLimit(t *testing.T) {
	rec := newRunner(t, "deposit src/a.ts 🧪", "deposit src/a.ts 🐛", "deposit src/a.ts#f 🧪", "deposit src/a.ts#f 🐛").
		Run(context.Background(), session())
	if rec.Outcome != core.OutcomeLimit || len(rec.Deposits) != 4 {
		t.Fatalf("%+v", rec)
	}
}

func TestContextOrder(t *testing.T) {
	s := session()
	s.Bundle.Honey = []string{"🍯 Scenario: empty amount"}
	sys, user, menu := Assemble(s.Genome, newRunner(t).Dict, s.Bundle, MenuEmoji)

	// Stable parts in the system text, in the architecture's order.
	order := []string{"You are a scout.", "SIGNALS YOU MAY DEPOSIT", "HONEY", "TERRAIN src/a.ts", "HOW TO ANSWER"}
	last := -1
	for _, part := range order {
		i := strings.Index(sys, part)
		if i <= last {
			t.Fatalf("%q missing or out of order in system text", part)
		}
		last = i
	}
	// Only this genome's signals are listed.
	if strings.Contains(sys, "Proof passed") || !strings.Contains(sys, "Needs a test") {
		t.Error("dictionary lines not filtered to the genome")
	}
	// Variable parts in the user turn.
	for _, part := range []string{"TASK", "LOCAL TRAIL", "src/a.ts  ✅", "YOU MAY DEPOSIT"} {
		if !strings.Contains(user, part) {
			t.Errorf("user turn misses %q", part)
		}
	}
	if len(menu.Tools) != 2 || len(menu.Emoji) != 2 {
		t.Errorf("menu %+v", menu)
	}
	// Byte-identical stable part for the same genome and target (cacheable).
	s2 := s
	s2.Bundle.Task = "A different task sentence."
	sys2, _, _ := Assemble(s2.Genome, newRunner(t).Dict, s2.Bundle, MenuEmoji)
	if sys != sys2 {
		t.Error("stable part changed with the task")
	}
}

// Alias menu: no dictionary emoji reaches the model outside the terrain,
// and a named signal still lands on the trail as its emoji.
func TestAliasMenu(t *testing.T) {
	s := session()
	s.Genome.Prompt = "You are a scout. If unsure, deposit ❓."
	s.Genome.MayDeposit = []string{"🧪", "🐛", "❓"}
	s.Bundle.TrailLine = "src/a.ts  ✅ 🧪🧪"
	s.Bundle.Garden = []string{"🍄 Components never call HttpClient."}
	s.Bundle.Terrain = "// verified ✅\nexport function f() {}"
	r := newRunner(t, `{"note":"no test","tool":"deposit","target":"src/a.ts#f","signal":"needs_test","outcome":""}`)
	r.Menu = MenuAlias

	sys, user, menu := Assemble(s.Genome, r.Dict, s.Bundle, MenuAlias)
	outsideTerrain := strings.Replace(sys, s.Bundle.Terrain, "", 1) + user
	for _, e := range r.Dict.Entries() {
		if strings.Contains(outsideTerrain, e.Emoji) {
			t.Errorf("%s (%s) shown to the model in alias mode", e.Emoji, e.Alias)
		}
	}
	if !strings.Contains(sys, "deposit unsure.") || !strings.Contains(sys, "grown Components") ||
		!strings.Contains(user, "passed needs_test needs_test") || !strings.Contains(sys, "// verified ✅") {
		t.Errorf("spelling wrong:\n%s\n%s", sys, user)
	}
	if menu.SignalField != "signal" || strings.Join(menu.Emoji, ",") != "needs_test,bug,unsure" {
		t.Errorf("menu %+v", menu)
	}

	s.Genome.Limits.Deposits = 1
	rec := r.Run(context.Background(), s)
	if rec.Outcome != core.OutcomeDone || len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🧪" {
		t.Fatalf("%+v", rec)
	}
}

// Both menu: the model reads "🧪 needs_test" everywhere and answers with the name.
func TestBothMenu(t *testing.T) {
	s := session()
	s.Bundle.TrailLine = "src/a.ts  ✅"
	r := newRunner(t, `{"note":"n","tool":"deposit","target":"src/a.ts","signal":"bug","outcome":""}`)
	sys, user, menu := Assemble(s.Genome, r.Dict, s.Bundle, MenuBoth)
	if !strings.Contains(sys, "🧪 needs_test  Needs a test.") || !strings.Contains(user, "✅ passed") ||
		!strings.Contains(user, "🧪 needs_test  🐛 bug") {
		t.Errorf("both menu text:\n%s\n%s", sys, user)
	}
	if menu.SignalField != "signal" || strings.Join(menu.Emoji, ",") != "needs_test,bug" {
		t.Errorf("menu %+v", menu)
	}
	r.Menu = MenuBoth
	s.Genome.Limits.Deposits = 1
	if rec := r.Run(context.Background(), s); len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🐛" {
		t.Fatalf("%+v", rec)
	}
}

func receptor(id, signal, question string) core.Genome {
	return core.Genome{
		ID: id, Caste: core.Scout, Role: core.RoleReceptor, Model: "fake:x", Prompt: question,
		MayDeposit: []string{signal}, Tools: []core.Tool{core.ToolDeposit, core.ToolFinish},
		Limits: core.Limits{Steps: 2, Seconds: 5, OutputTokens: 50, Malformed: 2, Deposits: 1},
	}
}

// Receptors on the same target share the whole system prefix; only the
// question in the user turn differs. That is what makes many small calls cheap.
func TestReceptorPrefixIsShared(t *testing.T) {
	s := session()
	s.Bundle.Garden = []string{"🍄 Components never call HttpClient."}
	d := newRunner(t).Dict
	bug := receptor("r-bug", "🐛", "Is there an input for which f gives a wrong result?")
	doc := receptor("r-doc", "📝", "Is the contract of f hard to guess without a comment?")
	sys1, user1, m1 := Assemble(bug, d, s.Bundle, MenuEmoji)
	sys2, user2, _ := Assemble(doc, d, s.Bundle, MenuEmoji)
	if sys1 != sys2 {
		t.Fatal("receptors on one target must share the system prefix")
	}
	if user1 == user2 || !strings.Contains(user1, "wrong result") || strings.Contains(sys1, "wrong result") {
		t.Fatal("the question belongs in the user turn only")
	}
	if !m1.Binary || m1.Emoji[0] != "🐛" || m1.Targets[0] != "src/a.ts#f" {
		t.Errorf("menu %+v", m1)
	}
	if err := bug.Validate(d); err != nil {
		t.Fatal(err)
	}
	two := bug
	two.MayDeposit = []string{"🐛", "🧪"}
	if err := two.Validate(d); err == nil {
		t.Error("a receptor with two signals was accepted")
	}
}

func TestReceptorYesAndNo(t *testing.T) {
	s := session()
	s.Genome = receptor("r-bug", "🐛", "Is there a bug?")
	yes := newRunner(t, `{"note":"off by one","answer":"yes"}`).Run(context.Background(), s)
	if yes.Outcome != core.OutcomeDone || len(yes.Deposits) != 1 || yes.Deposits[0].Emoji != "🐛" || yes.Deposits[0].Target != "src/a.ts#f" {
		t.Fatalf("yes: %+v", yes)
	}
	no := newRunner(t, `{"note":"fine","answer":"no"}`).Run(context.Background(), s)
	if no.Outcome != core.OutcomeDone || len(no.Deposits) != 0 || len(no.Steps) != 1 {
		t.Fatalf("no: %+v", no)
	}
	r := newRunner(t, `{"note":"fine","answer":"yes"}`)
	r.Menu = MenuBoth
	if rec := r.Run(context.Background(), s); len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🐛" {
		t.Fatalf("both menu: %+v", rec)
	}
}

func TestNoneOptionScout(t *testing.T) {
	s := session()
	s.Genome.NoneOption = true
	s.Genome.Limits.Deposits = 1
	d := newRunner(t).Dict
	sys, user, menu := Assemble(s.Genome, d, s.Bundle, MenuBoth)
	if !strings.Contains(sys, "none  The code needs none of the signals above.") || !strings.Contains(user, "🐛 bug  none") ||
		strings.Contains(user, "YOU MAY\n") || !menu.NoneOption {
		t.Errorf("none-option layout:\n%s\n%s", sys, user)
	}
	r := newRunner(t, `{"note":"clean","target":"src/a.ts#f","signal":"none"}`)
	r.Menu = MenuBoth
	if rec := r.Run(context.Background(), s); rec.Outcome != core.OutcomeDone || len(rec.Deposits) != 0 || rec.Malformed != 0 {
		t.Fatalf("none: %+v", rec)
	}
	r = newRunner(t, `{"note":"bug","target":"src/a.ts#f","signal":"bug"}`)
	r.Menu = MenuBoth
	if rec := r.Run(context.Background(), s); len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🐛" {
		t.Fatalf("bug: %+v", rec)
	}
	bad := s.Genome
	bad.Limits.Deposits = 0
	if err := bad.Validate(d); err == nil {
		t.Error("none_option without a one-deposit limit was accepted")
	}
}

func TestFoundAnswerReceptor(t *testing.T) {
	s := session()
	s.Genome = receptor("r-test", "🧪", "Name a behaviour no test checks.")
	s.Genome.Answer = core.AnswerFound
	d := newRunner(t).Dict
	sys, _, menu := Assemble(s.Genome, d, s.Bundle, MenuEmoji)
	if !menu.FoundAnswer || strings.Contains(sys, "answer no") || !strings.Contains(sys, `{"note":"...","found":"..."}`) {
		t.Errorf("found layout:\n%s\n%+v", sys, menu)
	}
	plain := receptor("r-test", "🧪", "Name a behaviour no test checks.")
	sysPlain, _, _ := Assemble(plain, d, s.Bundle, MenuEmoji)
	if sysPlain == sys {
		t.Error("the answer mode must change the shared answer block")
	}
	rec := newRunner(t, `{"note":"untested","found":"the comma branch"}`).Run(context.Background(), s)
	if len(rec.Deposits) != 1 || rec.Deposits[0].Emoji != "🧪" {
		t.Fatalf("found: %+v", rec)
	}
	rec = newRunner(t, `{"note":"covered","found":""}`).Run(context.Background(), s)
	if len(rec.Deposits) != 0 || rec.Outcome != core.OutcomeDone {
		t.Fatalf("empty: %+v", rec)
	}
	bad := plain
	bad.Role = "marker"
	bad.Answer = core.AnswerFound
	if err := bad.Validate(d); err == nil {
		t.Error("answer found on a non-receptor was accepted")
	}
}

// A probe keeps the receptors' prefix up to the answer block, states both
// options in the order asked, and never leans toward one.
func TestAssembleProbe(t *testing.T) {
	s := session()
	d := newRunner(t).Dict
	bug := receptor("r-bug", "🐛", "Is there an input for which f gives a wrong result?")
	base, _, _ := Assemble(bug, d, s.Bundle, MenuEmoji)
	prefix := base[:strings.Index(base, "\nHOW TO ANSWER")]
	noFirst, user, m := AssembleProbe(bug, d, s.Bundle, MenuEmoji, ProbeOrder{})
	yesFirst, _, my := AssembleProbe(bug, d, s.Bundle, MenuEmoji, ProbeOrder{YesFirst: true})
	direct, _, md := AssembleProbe(bug, d, s.Bundle, MenuEmoji, ProbeOrder{Direct: true})
	if !strings.HasPrefix(noFirst, prefix) || !strings.HasPrefix(yesFirst, prefix) {
		t.Fatal("probe prefix differs from the receptor prefix")
	}
	if !strings.Contains(noFirst, `"no" or "yes"`) || !strings.Contains(yesFirst, `"yes" or "no"`) {
		t.Errorf("option order not stated:\n%s\n%s", noFirst, yesFirst)
	}
	for _, sys := range []string{noFirst, yesFirst, direct} {
		if strings.Contains(sys, "Otherwise answer no") || strings.Contains(sys, `"answer":"no"`) {
			t.Errorf("probe prompt leans to no:\n%s", sys)
		}
	}
	if strings.Contains(direct, "note") || !md.Direct || m.YesFirst || !my.YesFirst || !strings.Contains(user, "wrong result") {
		t.Errorf("menus %+v %+v %+v", m, my, md)
	}
}

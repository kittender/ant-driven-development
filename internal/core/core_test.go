package core

import (
	"os"
	"strings"
	"testing"
)

// Table-driven tests are the Go idiom. JS analogy: test.each([...]) in Vitest.
func TestParseTarget(t *testing.T) {
	ok := []struct {
		in   string
		kind TargetKind
	}{
		{"src/a.ts", TargetFile},
		{"src/a.ts#onSubmit", TargetSymbol},
		{"@leni03", TargetAgeant},
		{"honey:credit-search/search-by-amount", TargetFrame},
		{"honey:credit-search/search-by-amount/empty-amount", TargetCell},
		{"garden:credit-search/data-access", TargetPlot},
	}
	for _, c := range ok {
		got, err := ParseTarget(c.in)
		if err != nil {
			t.Errorf("ParseTarget(%q): %v", c.in, err)
			continue
		}
		if got.Kind != c.kind {
			t.Errorf("ParseTarget(%q).Kind = %v, want %v", c.in, got.Kind, c.kind)
		}
		if got.String() != c.in {
			t.Errorf("round trip %q -> %q", c.in, got.String())
		}
	}

	bad := []string{"", "/etc/passwd", "../secret", "src/../../x", "src/./a.ts", "a b.ts",
		`src\a.ts`, "src/a.ts#", "src/a.ts#x#y", "@", "honey:only", "honey:a/b/c/d", "src//a.ts"}
	for _, in := range bad {
		if _, err := ParseTarget(in); err == nil {
			t.Errorf("ParseTarget(%q) accepted, want error", in)
		}
	}
}

func TestWithin(t *testing.T) {
	scope, _ := ParseTarget("src/a.ts#onSubmit")
	for in, want := range map[string]bool{
		"src/a.ts":          true,
		"src/a.ts#onSubmit": true,
		"src/a.ts#other":    true,
		"src/b.ts":          false,
		"@leni03":           false,
	} {
		tg, _ := ParseTarget(in)
		if got := tg.Within(scope); got != want {
			t.Errorf("%s within %s = %v, want %v", in, scope, got, want)
		}
	}
}

func TestValidSignalEmoji(t *testing.T) {
	good := []string{"🌱", "🧪", "🐛", "⛔", "👀", "❓", "🧹", "📝", "🧩", "🙋", "🚩",
		"✅", "❌", "🔒", "⭐", "🍯", "🍄", "📌", "🚧", "🔁", "💀", "🚨"}
	for _, e := range good {
		if err := ValidSignalEmoji(e); err != nil {
			t.Errorf("%s: %v", e, err)
		}
	}
	bad := map[string]string{
		"⚠":   "needs U+FE0F to display as emoji",
		"⚠️":  "two code points",
		"👍🏽":  "skin tone sequence",
		"🏽":   "bare skin tone modifier",
		"👨‍💻": "ZWJ sequence",
		"🇫🇷":  "flag (two regional indicators)",
		"a":   "letter",
		"":    "empty",
		"🧪🧪":  "two emoji",
	}
	for e, why := range bad {
		if err := ValidSignalEmoji(e); err == nil {
			t.Errorf("%q accepted (%s)", e, why)
		}
	}
}

func TestNormalizeEmoji(t *testing.T) {
	got, changed := NormalizeEmoji("⭐️")
	if got != "⭐" || !changed {
		t.Errorf("got %q, %v", got, changed)
	}
	if _, changed := NormalizeEmoji("🧪"); changed {
		t.Error("🧪 should not change")
	}
}

func TestEmojiTableSorted(t *testing.T) {
	for i, rg := range emojiPresentation {
		if rg[0] > rg[1] {
			t.Fatalf("range %d inverted", i)
		}
		if i > 0 && emojiPresentation[i-1][1] >= rg[0] {
			t.Fatalf("range %d overlaps or is out of order", i)
		}
	}
}

func loadExampleDict(t *testing.T) *Dictionary {
	t.Helper() // failures point at the caller, not here
	f, err := os.Open("../../ecosystem-home.example/dictionary.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	d, err := LoadDictionary(f)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestExampleFiles(t *testing.T) {
	d := loadExampleDict(t)
	if e, ok := d.ByAlias("needs_test"); !ok || e.Emoji != "🧪" || e.Class != Request {
		t.Errorf("needs_test = %+v", e)
	}
	for _, name := range []string{"scout-l0.json", "scout-l0-b.json", "scout-l0-b-think.json", "scout-l0-c.json"} {
		f, err := os.Open("../../ecosystem-home.example/genomes/" + name)
		if err != nil {
			t.Fatal(err)
		}
		g, err := LoadGenome(f)
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
		if err := g.Validate(d); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDictionaryRejects(t *testing.T) {
	cases := map[string][]Entry{
		"duplicate emoji": {{Alias: "a", Emoji: "🧪", Class: Request}, {Alias: "b", Emoji: "🧪", Class: Request}},
		"duplicate alias": {{Alias: "a", Emoji: "🧪", Class: Request}, {Alias: "a", Emoji: "🐛", Class: Request}},
		"bad class":       {{Alias: "a", Emoji: "🧪", Class: "maybe"}},
		"bad emoji":       {{Alias: "a", Emoji: "⚠", Class: Request}},
		"empty alias":     {{Emoji: "🧪", Class: Request}},
	}
	for name, entries := range cases {
		if _, err := NewDictionary(entries); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestGenomeValidate(t *testing.T) {
	d := loadExampleDict(t)
	base := Genome{
		ID: "g", Caste: Scout, Prompt: "p",
		MayDeposit: []string{"🧪"}, Tools: []Tool{ToolDeposit, ToolFinish},
		Limits: Limits{Steps: 1, Seconds: 1, OutputTokens: 1, Malformed: 1},
	}
	if err := base.Validate(d); err != nil {
		t.Fatalf("base genome: %v", err)
	}
	mutations := map[string]func(*Genome){
		"scout with write":   func(g *Genome) { g.Tools = append(g.Tools, ToolWrite) },
		"deposits a verdict": func(g *Genome) { g.MayDeposit = []string{"✅"} },
		"unknown emoji":      func(g *Genome) { g.MayDeposit = []string{"🦄"} },
		"no finish":          func(g *Genome) { g.Tools = []Tool{ToolDeposit} },
		"zero steps":         func(g *Genome) { g.Limits.Steps = 0 },
		"negative deposits":  func(g *Genome) { g.Limits.Deposits = -1 },
		"unknown caste":      func(g *Genome) { g.Caste = "drone" },
		"queen deposits":     func(g *Genome) { g.Caste = Queen },
	}
	for name, mutate := range mutations {
		g := base
		g.Tools = append([]Tool(nil), base.Tools...) // copy the slice, or mutations leak between cases
		g.MayDeposit = append([]string(nil), base.MayDeposit...)
		mutate(&g)
		if err := g.Validate(d); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestSpell(t *testing.T) {
	d := loadExampleDict(t)
	cases := map[string]string{
		"credit.service.ts  ✅ 🧪🧪 📌":              "credit.service.ts  passed needs_test needs_test contract",
		"⭐️ example":                             "reference example",
		"If you cannot judge, deposit ❓.":        "If you cannot judge, deposit unsure.",
		"🦄 not in the dictionary, code `i <= n`": "🦄 not in the dictionary, code `i <= n`",
	}
	for in, want := range cases {
		if got := d.Spell(in); got != want {
			t.Errorf("Spell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAnnotate(t *testing.T) {
	d := loadExampleDict(t)
	if got := d.Annotate("x.ts  ✅ 🧪🧪 ⭐️ 🦄"); got != "x.ts  ✅ passed 🧪 needs_test 🧪 needs_test ⭐ reference 🦄" {
		t.Errorf("Annotate = %q", got)
	}
	if e, ok := d.Lookup("🌱"); !ok || e.Alias != "missing" {
		t.Errorf("🌱 alias = %q, want missing", e.Alias)
	}
}

// The README glossary must list exactly the dictionary: same emoji, same
// name, same meaning. Edit dictionary.json, then the table, or this fails.
func TestReadmeGlossaryMatchesDictionary(t *testing.T) {
	d := loadExampleDict(t)
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ name, meaning string }
	rows := map[string]row{}
	for _, line := range strings.Split(string(raw), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) < 5 {
			continue
		}
		emoji := strings.TrimSpace(cells[1])
		if _, ok := d.Lookup(emoji); !ok {
			continue
		}
		if _, dup := rows[emoji]; dup {
			t.Errorf("%s listed twice in the README glossary", emoji)
		}
		rows[emoji] = row{strings.Trim(strings.TrimSpace(cells[2]), "`"), strings.TrimSpace(cells[3])}
	}
	for _, e := range d.Entries() {
		r, ok := rows[e.Emoji]
		switch {
		case !ok:
			t.Errorf("%s (%s) is missing from the README glossary", e.Emoji, e.Alias)
		case r.name != e.Alias || r.meaning != e.Meaning:
			t.Errorf("%s: README says %q / %q, dictionary says %q / %q", e.Emoji, r.name, r.meaning, e.Alias, e.Meaning)
		}
	}
	if len(rows) != len(d.Entries()) {
		t.Errorf("README glossary has %d rows, dictionary has %d entries", len(rows), len(d.Entries()))
	}
}

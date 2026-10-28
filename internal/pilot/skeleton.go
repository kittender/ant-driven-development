package pilot

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"kittender/add/internal/honey"
)

// A reflex (concept: lowest level first): the Ecosystem, not a model,
// writes the first line of every step definition a scenario needs, with the
// right quoting and parameters. Four trial runs showed a small model
// writing the same broken string for a step whose text holds an apostrophe,
// whatever the feedback; the tester now writes only the bodies.

// Skeleton is one step definition to write.
type Skeleton struct {
	Keyword    string // Given, When or Then (And/But resolved)
	Expression string // cucumber expression, e.g. it should be player {string}'s turn
	Params     []string
	Code       string // the first line and an empty body, ready to fill
	DefinedIn  string // a step file that already registers this expression
}

var (
	stepKw      = regexp.MustCompile(`^(Given|When|Then|And|But)\s+(.*)$`)
	quoted      = regexp.MustCompile(`"[^"]*"`)
	placeholder = regexp.MustCompile(`<([A-Za-z_][A-Za-z0-9_]*)>`)
	integer     = regexp.MustCompile(`(^|[^\w.])(-?\d+)\b`)
)

// Skeletons lists the step definitions a scenario needs, Background first,
// each once. defined maps an expression to the step file that registers it.
func Skeletons(f honey.Feature, sc honey.Scenario, defined map[string]string) []Skeleton {
	cols := exampleColumns(sc.Examples)
	var out []Skeleton
	seen := map[string]bool{}
	add := func(steps []string) {
		last := "Given"
		for i, s := range steps {
			m := stepKw.FindStringSubmatch(s)
			if m == nil {
				continue
			}
			kw := m[1]
			if kw == "And" || kw == "But" {
				kw = last
			}
			last = kw
			table := i+1 < len(steps) && strings.HasPrefix(steps[i+1], "|")
			sk := skeleton(kw, m[2], table, cols)
			if seen[sk.Expression] {
				continue
			}
			seen[sk.Expression] = true
			sk.DefinedIn = defined[sk.Expression]
			out = append(out, sk)
		}
	}
	add(f.Background)
	add(sc.Steps)
	return out
}

// exampleColumns maps each column of an outline's examples to "int" when
// every value is an integer, else "string".
func exampleColumns(rows []string) map[string]string {
	if len(rows) < 2 {
		return nil
	}
	split := func(r string) []string {
		cells := strings.Split(strings.Trim(r, "| "), "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		return cells
	}
	head := split(rows[0])
	kinds := map[string]string{}
	for i, h := range head {
		kinds[h] = "int"
		for _, r := range rows[1:] {
			c := split(r)
			if i >= len(c) {
				continue
			}
			if _, err := strconv.Atoi(c[i]); err != nil {
				kinds[h] = "string"
			}
		}
	}
	return kinds
}

func skeleton(kw, text string, table bool, cols map[string]string) Skeleton {
	var params []string
	n := map[string]int{}
	name := func(base string) string {
		n[base]++
		if n[base] == 1 {
			return base
		}
		return fmt.Sprintf("%s%d", base, n[base])
	}
	// Walk the text once, left to right, so parameters keep their order.
	var expr strings.Builder
	rest := text
	for rest != "" {
		qi := quoted.FindStringIndex(rest)
		pi := placeholder.FindStringSubmatchIndex(rest)
		ii := integer.FindStringSubmatchIndex(rest)
		next, kind := -1, ""
		for _, c := range []struct {
			at   []int
			kind string
		}{{qi, "q"}, {pi, "p"}, {ii, "i"}} {
			if c.at == nil {
				continue
			}
			start := c.at[0]
			if c.kind == "i" {
				start = c.at[4] // the number itself, not the character before it
			}
			if next == -1 || start < next {
				next, kind = start, c.kind
			}
		}
		if next == -1 {
			expr.WriteString(escapeExpr(rest))
			break
		}
		expr.WriteString(escapeExpr(rest[:next]))
		switch kind {
		case "q":
			// "X", or "<player>" in an outline: a quoted value either way.
			inner := rest[qi[0]+1 : qi[1]-1]
			base := "text"
			if m := placeholder.FindStringSubmatch(inner); m != nil && m[0] == inner {
				base = m[1]
			}
			expr.WriteString("{string}")
			params = append(params, name(base)+": string")
			rest = rest[qi[1]:]
		case "p":
			col := rest[pi[2]:pi[3]]
			if cols[col] == "int" {
				expr.WriteString("{int}")
				params = append(params, name(col)+": number")
			} else {
				expr.WriteString("{string}")
				params = append(params, name(col)+": string")
			}
			rest = rest[pi[1]:]
		case "i":
			expr.WriteString("{int}")
			params = append(params, name("n")+": number")
			rest = rest[ii[5]:]
		}
	}
	if table {
		params = append(params, "table: DataTable")
	}
	e := expr.String()
	args := append([]string{"world"}, params...)
	code := fmt.Sprintf("%s(%s, (%s) => {\n  // write this step\n});", kw, strconv.Quote(e), strings.Join(args, ", "))
	return Skeleton{Keyword: kw, Expression: e, Params: params, Code: code}
}

// escapeExpr escapes the characters cucumber expressions treat specially:
// ( ) for optional text, / for alternatives, { } for parameters.
func escapeExpr(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`, `/`, `\/`, `{`, `\{`, `}`, `\}`)
	return r.Replace(s)
}

var registered = regexp.MustCompile(`\b(?:Given|When|Then)\(\s*("(?:[^"\\]|\\.)*"|'(?:[^'\\]|\\.)*'|` + "`[^`]*`" + `)`)

// RegisteredExpressions reads the expressions a step file registers.
func RegisteredExpressions(src string) []string {
	var out []string
	for _, m := range registered.FindAllStringSubmatch(src, -1) {
		lit := m[1]
		if strings.HasPrefix(lit, `"`) {
			if s, err := strconv.Unquote(lit); err == nil {
				out = append(out, s)
			}
			continue
		}
		inner := lit[1 : len(lit)-1]
		out = append(out, strings.NewReplacer(`\'`, `'`, `\\`, `\`).Replace(inner))
	}
	return out
}

// RenderSkeletons is the bundle section for the tester.
func RenderSkeletons(sks []Skeleton) string {
	var b strings.Builder
	b.WriteString("Written by the Ecosystem from the step texts: the first line of each step (expression, quoting, parameters) is right. Keep it exactly; write the body. A step marked as already defined must not be written again.\n\n```ts\n")
	for _, s := range sks {
		if s.DefinedIn != "" {
			fmt.Fprintf(&b, "// already defined in %s: do not write it again\n// %s(%s, …)\n\n", s.DefinedIn, s.Keyword, strconv.Quote(s.Expression))
			continue
		}
		b.WriteString(s.Code + "\n\n")
	}
	b.WriteString("```")
	return b.String()
}

package pilot

import (
	"fmt"
	"regexp"
	"strings"
)

// Reflex 4 (outline, run 3): the Ecosystem writes src/game/game.ts, as it
// writes the step files for testers. A worker sends one body per contract
// method and an optional helpers section; the harness assembles the file:
//
//	import type { GameApi } from './contract';
//	// Helpers …
//	const newGame: GameApi['newGame'] = () => { <body> };
//	…
//	export const game: GameApi = { newGame, … };
//
// JS analogy: a form with one textarea per function instead of a blank
// page; typing each const with GameApi['name'] makes TypeScript check every
// returned object literal against the contract (no 'string' widening).

// Method is one method of the contract's GameApi interface.
type Method struct {
	Name   string
	Params []string // parameter names, in order
	Sig    string   // the line as written in the contract, for the prompt
}

var (
	apiBlock   = regexp.MustCompile(`(?s)export\s+interface\s+GameApi\s*\{(.*?)\n\}`)
	methodLine = regexp.MustCompile(`(?m)^\s*(\w+)\s*\(([^)]*)\)\s*:\s*(.+?);\s*$`)
)

// ContractMethods lists GameApi's methods in contract order. None means the
// contract has no such interface: workers then write the whole file.
func ContractMethods(contract string) []Method {
	m := apiBlock.FindStringSubmatch(contract)
	if m == nil {
		return nil
	}
	var out []Method
	for _, l := range methodLine.FindAllStringSubmatch(m[1], -1) {
		var params []string
		for _, p := range strings.Split(l[2], ",") {
			name, _, _ := strings.Cut(strings.TrimSpace(p), ":")
			name = strings.TrimSuffix(strings.TrimSpace(name), "?")
			if name != "" {
				params = append(params, name)
			}
		}
		out = append(out, Method{Name: l[1], Params: params, Sig: strings.TrimSpace(l[0])})
	}
	return out
}

const (
	notImplemented = "throw new Error('not implemented');"
	helpersMark    = "// Helpers (written by workers)"
	methodsMark    = "// Contract methods (one body each, written by workers; the Ecosystem writes the rest)"
)

// GameFile is the part of game.ts the workers write.
type GameFile struct {
	Helpers string
	Bodies  map[string]string // method name → body; missing = not implemented
}

func methodHead(name string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^const ` + name + `: GameApi\['` + name + `'\] = \([^)\n]*\) => \{\n`)
}

var exportLine = regexp.MustCompile(`(?m)^export const game: GameApi = `)

// ParseGameFile reads back a file RenderGameFile wrote. Any other file (the
// human's stub) reads as no helpers and nothing implemented.
func ParseGameFile(src string, methods []Method) GameFile {
	gf := GameFile{Bodies: map[string]string{}}
	hi, mi := strings.Index(src, helpersMark), strings.Index(src, methodsMark)
	if hi < 0 || mi < hi {
		return gf
	}
	gf.Helpers = strings.Trim(src[hi+len(helpersMark):mi], "\n")
	type span struct {
		name        string
		start, head int
	}
	var spans []span
	for _, m := range methods {
		if loc := methodHead(m.Name).FindStringIndex(src); loc != nil {
			spans = append(spans, span{m.Name, loc[0], loc[1]})
		}
	}
	end := len(src)
	if loc := exportLine.FindStringIndex(src); loc != nil {
		end = loc[0]
	}
	for _, s := range spans {
		stop := end // the next method's first line, or the export
		for _, o := range spans {
			if o.start > s.start && o.start < stop {
				stop = o.start
			}
		}
		chunk := src[s.head:stop]
		if k := strings.LastIndex(chunk, "\n};"); k >= 0 {
			chunk = chunk[:k]
		}
		if b := strings.Trim(chunk, "\n"); strings.TrimSpace(b) != notImplemented {
			gf.Bodies[s.name] = b
		}
	}
	return gf
}

// RenderGameFile writes game.ts. A method keeps only the leading parameters
// its body uses (trailing unused ones would fail no-unused-vars); dropped
// lists "name: param" for each one left out, for the reflex log.
func RenderGameFile(gf GameFile, methods []Method) (src string, dropped []string) {
	var b strings.Builder
	b.WriteString("import type { GameApi } from './contract';\n\n")
	b.WriteString("// Written by the Ecosystem from the workers' bodies (reflex 4). Edit through a worker session.\n\n")
	b.WriteString(helpersMark + "\n")
	if h := strings.Trim(gf.Helpers, "\n"); h != "" {
		b.WriteString(h + "\n")
	}
	b.WriteString("\n" + methodsMark + "\n\n")
	var names []string
	for _, m := range methods {
		body, ok := gf.Bodies[m.Name]
		params := []string{}
		if !ok || strings.TrimSpace(body) == "" {
			body = "  " + notImplemented
		} else {
			params = m.Params
			for len(params) > 0 && !regexp.MustCompile(`\b`+regexp.QuoteMeta(params[len(params)-1])+`\b`).MatchString(body) {
				dropped = append(dropped, m.Name+": "+params[len(params)-1])
				params = params[:len(params)-1]
			}
		}
		fmt.Fprintf(&b, "const %s: GameApi['%s'] = (%s) => {\n%s\n};\n\n", m.Name, m.Name, strings.Join(params, ", "), strings.Trim(body, "\n"))
		names = append(names, m.Name)
	}
	b.WriteString("export const game: GameApi = { " + strings.Join(names, ", ") + " };\n")
	return b.String(), dropped
}

// RenderMethods is the numbered list a worker writes bodies for.
func RenderMethods(methods []Method, gf GameFile) string {
	var b strings.Builder
	for _, m := range methods {
		state := "not implemented yet"
		if _, ok := gf.Bodies[m.Name]; ok {
			state = "implemented; an empty body keeps it"
		}
		fmt.Fprintf(&b, "- %s (%s). Contract: %s Parameters in scope: %s.\n", m.Name, state, m.Sig, paramList(m.Params))
	}
	b.WriteString("\nThe Ecosystem writes each method's declaration; a body is only the statements inside it, ending with a return.\n")
	return b.String()
}

func paramList(ps []string) string {
	if len(ps) == 0 {
		return "none"
	}
	return strings.Join(ps, ", ")
}

var methodDecl = regexp.MustCompile(`GameApi\['\w+'\]\s*=`)

func declaresMethod(body string) bool { return methodDecl.MatchString(body) }

var contractImport = regexp.MustCompile(`(?m)^\s*import\s+type\s+\{[^}]*\}\s+from\s+'\./contract';?[ \t]*\n?`)

// CleanWorkerBody unwraps a body sent as a whole declaration of its method
// (reflex 4, logged) and refuses what is not the inside of one method.
func CleanWorkerBody(raw, name string) (string, bool, error) {
	body := raw
	if strings.HasPrefix(strings.TrimSpace(raw), "```") {
		body = stripFence(raw)
	}
	body = strings.Trim(body, "\n")
	if strings.TrimSpace(body) == "" {
		return "", false, nil
	}
	unwrapped := false
	head := regexp.MustCompile(`^\s*(?:export\s+)?(?:const\s+|let\s+|var\s+|function\s+|async\s+)?` + name + `\b`)
	if head.MatchString(body) {
		block := regexp.MustCompile(`^[^\n]*\{[ \t]*\n`)
		rest := strings.TrimRight(body, " \t\n;,")
		if loc := block.FindStringIndex(body); loc != nil && !strings.Contains(body[:loc[1]], "=> ({") && strings.HasSuffix(rest, "}") {
			// Block body: name(...) { … } or const name = (...) => { … };
			inner := strings.TrimRight(body[loc[1]:], " \t\n;,")
			body, unwrapped = strings.Trim(strings.TrimSuffix(inner, "}"), "\n"), true
		} else if i := strings.Index(body, "=>"); i >= 0 {
			// Expression arrow (run 3's gap): const name = (...) => ({ … }); → return ({ … });
			if expr := strings.TrimSpace(rest[i+2:]); expr != "" {
				body, unwrapped = "  return "+expr+";", true
			}
		}
	}
	if declaresMethod(body) {
		return "", unwrapped, fmt.Errorf("the body of %s declares a contract method: send only the code inside the method (the Ecosystem writes its declaration)", name)
	}
	for _, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "import ") || strings.HasPrefix(t, "export ") {
			return "", unwrapped, fmt.Errorf("a body is the code inside the method %s only: no import or export lines (the Ecosystem writes them)", name)
		}
	}
	return body, unwrapped, nil
}

// CleanHelpers drops contract type imports (the auto-import reflex adds
// what is used) and refuses other imports, exports, or a redeclared method.
func CleanHelpers(raw string, methods []Method) (string, error) {
	h := contractImport.ReplaceAllString(stripFence(raw), "")
	for _, l := range strings.Split(h, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "import ") || strings.HasPrefix(t, "export ") {
			return "", fmt.Errorf("helpers: no import or export lines (the game is pure and the Ecosystem writes the exports): %q", t)
		}
	}
	for _, m := range methods {
		if regexp.MustCompile(`(?m)^\s*(?:const|let|var|function)\s+` + m.Name + `\b`).MatchString(h) {
			return "", fmt.Errorf("helpers: %s is a contract method; send its code as its body, not as a helper", m.Name)
		}
	}
	return strings.Trim(h, "\n"), nil
}

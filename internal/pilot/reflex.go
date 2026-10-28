package pilot

import (
	"regexp"
	"sort"
	"strings"
)

// Reflexes (concept: lowest level first) are deterministic edits the
// Ecosystem makes to a written file, with no model, the way an editor or a
// formatter would. Each one is logged with the attempt, so the report shows
// how much of a green scenario a reflex carried.

var (
	exportedType = regexp.MustCompile(`(?m)^export\s+(?:type|interface)\s+([A-Z]\w*)`)
	importLine   = regexp.MustCompile(`(?m)^import\s.*;[ \t]*$`)
	contractImp  = regexp.MustCompile(`(?m)^import type \{([^}]*)\} from '([^']*contract)';[ \t]*$`)
	localDecl    = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:type|interface|class|const|let|function)\s+([A-Z]\w*)`)
)

// ContractTypes lists the types and interfaces a contract file exports.
func ContractTypes(contract string) []string {
	var out []string
	for _, m := range exportedType.FindAllStringSubmatch(contract, -1) {
		out = append(out, m[1])
	}
	return out
}

// AutoImportTypes adds an `import type` for every contract type the source
// uses but neither imports nor declares, as an editor's auto-import does.
// from is the import path of the contract seen from this file. It returns
// the new source and the names it added.
func AutoImportTypes(src, from string, types []string) (string, []string) {
	imported := map[string]bool{}
	for _, m := range regexp.MustCompile(`(?m)^import\s[^;]*;`).FindAllString(src, -1) {
		for _, w := range regexp.MustCompile(`\b[A-Z]\w*\b`).FindAllString(m, -1) {
			imported[w] = true
		}
	}
	declared := map[string]bool{}
	for _, m := range localDecl.FindAllStringSubmatch(src, -1) {
		declared[m[1]] = true
	}
	body := importLine.ReplaceAllString(src, "")
	var missing []string
	for _, t := range types {
		if imported[t] || declared[t] {
			continue
		}
		if regexp.MustCompile(`\b` + t + `\b`).MatchString(body) {
			missing = append(missing, t)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}
	sort.Strings(missing)
	if m := contractImp.FindStringSubmatchIndex(src); m != nil {
		names := strings.Split(src[m[2]:m[3]], ",")
		for i := range names {
			names[i] = strings.TrimSpace(names[i])
		}
		names = append(names, missing...)
		sort.Strings(names)
		line := "import type { " + strings.Join(names, ", ") + " } from '" + src[m[4]:m[5]] + "';"
		return src[:m[0]] + line + src[m[1]:], missing
	}
	line := "import type { " + strings.Join(missing, ", ") + " } from '" + from + "';\n"
	if locs := importLine.FindAllStringIndex(src, -1); len(locs) > 0 {
		end := locs[len(locs)-1][1]
		return src[:end] + "\n" + strings.TrimSuffix(line, "\n") + src[end:], missing
	}
	return line + src, missing
}

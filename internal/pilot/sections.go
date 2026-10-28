package pilot

import "strings"

// ExtractSections returns the Markdown sections whose heading contains one
// of the keywords, each up to the next heading of the same or a higher
// level. Lines inside ``` code fences are never read as headings (READMEs
// put "# a comment" in shell and Gherkin examples).
func ExtractSections(md string, keywords []string) string {
	lines := strings.Split(md, "\n")
	var out []string
	inFence, take, level := false, false, 0
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "```") {
			inFence = !inFence
		}
		if !inFence && strings.HasPrefix(t, "#") && !strings.HasPrefix(t, "```") {
			lv := len(t) - len(strings.TrimLeft(t, "#"))
			if take && lv <= level {
				take = false
			}
			if !take {
				for _, k := range keywords {
					if strings.Contains(t, k) {
						take, level = true, lv
						if len(out) > 0 {
							out = append(out, "")
						}
						break
					}
				}
			}
		}
		if take {
			out = append(out, l)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

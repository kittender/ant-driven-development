package pilot

import (
	"fmt"
	"strings"
)

// LineDiff returns a short unified-style diff of two texts: "-" for lines
// removed, "+" for lines added, two lines of context around each change.
// It is what a retry is shown of its failed attempt (architecture 0.2,
// context part 9: the patch of a failed change and the check that failed).
//
// The longest common subsequence is O(n*m); files here are a few hundred
// lines, so that is fine.
func LineDiff(old, new string, maxLines int) string {
	a, b := strings.Split(old, "\n"), strings.Split(new, "\n")
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else {
				lcs[i][j] = max(lcs[i+1][j], lcs[i][j+1])
			}
		}
	}
	type op struct {
		k    byte
		text string
	}
	var ops []op
	i, j := 0, 0
	for i < n || j < m {
		switch {
		case i < n && j < m && a[i] == b[j]:
			ops = append(ops, op{' ', a[i]})
			i, j = i+1, j+1
		case j < m && (i == n || lcs[i][j+1] >= lcs[i+1][j]):
			ops = append(ops, op{'+', b[j]})
			j++
		default:
			ops = append(ops, op{'-', a[i]})
			i++
		}
	}
	keep := make([]bool, len(ops))
	for k, o := range ops {
		if o.k != ' ' {
			for c := max(0, k-2); c <= min(len(ops)-1, k+2); c++ {
				keep[c] = true
			}
		}
	}
	var out []string
	skipped := false
	for k, o := range ops {
		if !keep[k] {
			skipped = true
			continue
		}
		if skipped && len(out) > 0 {
			out = append(out, "  …")
		}
		skipped = false
		out = append(out, string(o.k)+" "+o.text)
	}
	if len(out) > maxLines {
		rest := len(out) - maxLines
		out = append(out[:maxLines], fmt.Sprintf("  … %d more diff lines", rest))
	}
	return strings.Join(out, "\n")
}

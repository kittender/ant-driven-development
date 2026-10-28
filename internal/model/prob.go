package model

import (
	"fmt"
	"math"
	"regexp"
	"strings"
)

// AnswerProbability reads P(yes) for a binary reply from token log
// probabilities, instead of trusting the token the model happened to write.
//
// It finds where the value of "answer" starts in the generated JSON, takes
// the token at that position, and sums the probability of every alternative
// that would have started the value with "y" (yes) or "n" (no). The result
// is yes / (yes + no): a schema that only allows the two words removes
// every other option, and the ratio between them survives that.
//
// JS analogy: instead of reading the string the model typed, we look at
// how strongly it leaned toward each key before pressing one.
func AnswerProbability(lps []TokenLogprob) (float64, error) {
	var text strings.Builder
	starts := make([]int, len(lps))
	for i, t := range lps {
		starts[i] = text.Len()
		text.WriteString(t.Token)
	}
	full := text.String()
	loc := answerValue.FindStringIndex(full)
	if loc == nil {
		return 0, fmt.Errorf("no \"answer\" value in the reply")
	}
	pos := loc[1] // first character of the value
	i := -1
	for k := range lps {
		if starts[k]+len(lps[k].Token) > pos {
			i = k
			break
		}
	}
	if i < 0 {
		return 0, fmt.Errorf("reply ends before the answer value")
	}
	prefix := full[starts[i]:pos] // e.g. `"` when the token is `"yes`
	alts := lps[i].Top
	seen := map[string]bool{}
	var yes, no float64
	add := func(tok string, lp float64) {
		if seen[tok] || !strings.HasPrefix(tok, prefix) {
			return
		}
		seen[tok] = true
		rest := strings.ToLower(strings.TrimLeft(tok[len(prefix):], " "))
		switch {
		case strings.HasPrefix(rest, "y"):
			yes += math.Exp(lp)
		case strings.HasPrefix(rest, "n"):
			no += math.Exp(lp)
		}
	}
	for _, a := range alts {
		add(a.Token, a.Logprob)
	}
	add(lps[i].Token, lps[i].Logprob) // some runtimes leave the chosen token out of the list
	if yes+no == 0 {
		return 0, fmt.Errorf("no yes/no alternative at the answer token %q", lps[i].Token)
	}
	return yes / (yes + no), nil
}

var answerValue = regexp.MustCompile(`"answer"\s*:\s*"`)

package fuzzy

import "strings"

// Terms is an immutable compiled query. A result population shares its folded
// patterns, and each candidate is decoded only once even for multiword queries.
type Terms struct {
	matcher  Matcher
	patterns [][]rune
}

// CompileTerms prepares a whitespace-separated AND query for repeated scoring.
func (m Matcher) CompileTerms(query string) Terms {
	terms := Terms{matcher: m}
	for _, word := range strings.Fields(query) {
		terms.patterns = append(terms.patterns, []rune(m.fold(word)))
	}
	return terms
}

// Score preserves ScoreTerms' AND semantics and scores, including smart case,
// word boundaries, density limits, and the original-text camel-case bonus.
func (q Terms) Score(text string) int {
	if len(q.patterns) == 0 {
		return 1
	}
	if text == "" {
		return 0
	}
	original := []rune(text)
	folded := []rune(q.matcher.fold(text))
	total := 0
	for _, pattern := range q.patterns {
		score := scoreSubsequence(folded, original, pattern)
		if score == 0 {
			return 0
		}
		total += score
	}
	return total
}

package fuzzy

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

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
	// Most titles and short descriptions fit on the stack. Decode and fold in
	// one pass, preserving original runes for the camel-case score. Long text
	// gets exact-size buffers; the compiled query stays immutable and shareable.
	var originalBuffer, foldedBuffer [128]rune
	original, folded := originalBuffer[:0], foldedBuffer[:0]
	if len(text) > len(originalBuffer) {
		if count := utf8.RuneCountInString(text); count > len(originalBuffer) {
			original, folded = make([]rune, 0, count), make([]rune, 0, count)
		}
	}
	for _, r := range text {
		original = append(original, r)
		if !q.matcher.caseSensitive {
			r = unicode.ToLower(r)
		}
		folded = append(folded, r)
	}
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

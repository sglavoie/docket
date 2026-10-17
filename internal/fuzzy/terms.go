package fuzzy

import (
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// Terms is an immutable compiled query. A result population shares its folded
// patterns, and each candidate is decoded only once even for multiword queries.
type Terms struct {
	matcher  Matcher
	patterns [][]rune
	query    string
	cache    *ScoreCache
}

// Reuse bounded scratch for descriptions and comments. Each call borrows its
// own buffers, so a compiled query remains safe to share between goroutines.
// Larger documents use exact-size temporary slices rather than growing a pool
// entry indefinitely. Short titles still use stack scratch.
type termRuneBuffers struct {
	original [4096]rune
	folded   [4096]rune
}

var longTermBuffers = sync.Pool{New: func() any { return new(termRuneBuffers) }}

// CompileTerms prepares a whitespace-separated AND query for repeated scoring.
func (m Matcher) CompileTerms(query string) Terms {
	terms := Terms{matcher: m, query: query}
	for _, word := range strings.Fields(query) {
		terms.patterns = append(terms.patterns, []rune(m.fold(word)))
	}
	return terms
}

// Score preserves ScoreTerms' AND semantics and scores, including smart case,
// word boundaries, density limits, and the original-text camel-case bonus.
func (q Terms) Score(text string) int {
	if q.cache != nil && len(q.patterns) != 0 && len(text) > 128 {
		return q.cache.score(q, text)
	}
	return q.scoreUncached(text)
}

// WithCache shares bounded, content-addressed work across live query changes.
// It caches scores, never membership: callers still evaluate current predicates.
func (q Terms) WithCache(cache *ScoreCache) Terms {
	q.cache = cache
	return q
}

// ScoreParts has exactly the semantics of scoring a space-joined field. A
// bounded cache also reuses the joined text across successive query letters.
func (q Terms) ScoreParts(parts []string) int {
	if q.cache != nil {
		return q.Score(q.cache.join(parts))
	}
	return q.Score(strings.Join(parts, " "))
}

func (q Terms) scoreUncached(text string) int {
	if len(q.patterns) == 0 {
		return 1
	}
	if text == "" {
		return 0
	}
	if len(q.patterns) == 1 && len(q.patterns[0]) == 1 {
		return q.scoreSingleRune(text, q.patterns[0][0])
	}
	// Reject long text missing the first required rune before decoding or
	// allocating. This is only a necessary condition, never a scoring shortcut.
	if len(text) > 128 && !q.containsFirstRune(text) {
		return 0
	}
	// Decode and fold in one pass, preserving original case for camel bonuses.
	var originalBuffer, foldedBuffer [128]rune
	original, folded := originalBuffer[:0], foldedBuffer[:0]
	if len(text) > len(originalBuffer) {
		if count := utf8.RuneCountInString(text); count > len(originalBuffer) {
			if count <= len(termRuneBuffers{}.original) {
				buffers := longTermBuffers.Get().(*termRuneBuffers)
				defer longTermBuffers.Put(buffers)
				original, folded = buffers.original[:0], buffers.folded[:0]
			} else {
				original, folded = make([]rune, 0, count), make([]rune, 0, count)
			}
		}
	}
	for _, r := range text {
		original = append(original, r)
		if !q.matcher.caseSensitive {
			r = foldRune(r)
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

func (q Terms) containsFirstRune(text string) bool {
	first := q.patterns[0][0]
	if q.matcher.caseSensitive {
		return strings.ContainsRune(text, first)
	}
	for _, r := range text {
		if foldRune(r) == first {
			return true
		}
	}
	return false
}

// A one-letter query needs neither decoded buffers nor a dynamic-programming
// window. After the first character, a word/camel boundary is the best possible
// match, so it also ends the scan. Keep the document length tie-break unchanged.
func (q Terms) scoreSingleRune(text string, pattern rune) int {
	best := 0
	var previous rune
	for offset, original := range text {
		r := original
		if !q.matcher.caseSensitive {
			r = foldRune(r)
		}
		if r == pattern {
			if offset == 0 {
				return addLengthBonus(4, utf8.RuneCountInString(text), 1)
			}
			if isWordBoundary(previous) || isCamelBoundary(previous, original) {
				return addLengthBonus(3, utf8.RuneCountInString(text), 1)
			}
			best = 1
		}
		previous = original
	}
	if best == 0 {
		return 0
	}
	return addLengthBonus(best, utf8.RuneCountInString(text), 1)
}

func foldRune(r rune) rune {
	if r < utf8.RuneSelf {
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return r
	}
	return unicode.ToLower(r)
}

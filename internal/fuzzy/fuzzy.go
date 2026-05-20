// Package fuzzy provides fzf-style fuzzy matching with scoring.
package fuzzy

import (
	"slices"
	"strings"
	"unicode"
)

// Match returns true if the pattern fuzzy-matches the text.
// Matching is case-insensitive and uses subsequence matching,
// where all pattern characters must appear in the text in order
// but not necessarily contiguously.
func Match(text, pattern string) bool {
	return Score(text, pattern) > 0
}

// Score returns a match score for the pattern against the text.
// Returns 0 if there is no match.
// Higher scores indicate better matches based on:
//   - Consecutive character matches (+3 bonus)
//   - Word boundary matches (+2 bonus)
//   - Match at start of text (+1 bonus)
//   - Base score of 1 per matched character
//
// The score is also adjusted to prefer shorter texts.
func Score(text, pattern string) int {
	if pattern == "" {
		return 1 // Empty pattern matches everything with minimal score
	}
	if text == "" {
		return 0
	}

	textLower := strings.ToLower(text)
	patternLower := strings.ToLower(pattern)

	textRunes := []rune(textLower)
	patternRunes := []rune(patternLower)

	return scoreSubsequence(textRunes, patternRunes)
}

// ScoreTerms splits the query into whitespace-separated terms and returns
// a combined score if ALL terms match the text. Returns 0 if any term fails to match.
// This preserves AND logic for multi-word queries like "need way".
func ScoreTerms(text, query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1
	}

	terms := strings.Fields(query)
	if len(terms) == 0 {
		return 1
	}

	totalScore := 0
	for _, term := range terms {
		score := Score(text, term)
		if score == 0 {
			return 0 // All terms must match
		}
		totalScore += score
	}

	return totalScore
}

// MatchTerms returns true if all whitespace-separated terms in the query
// fuzzy-match the text.
func MatchTerms(text, query string) bool {
	return ScoreTerms(text, query) > 0
}

// ScoreWithPositions returns both a match score and the indices in text where matches occurred.
// Returns 0 score and nil positions if there is no match.
// Positions are 0-indexed rune positions in the original text (before lowercasing);
// callers that need to slice the original string by these positions must index
// via []rune(text), not by byte offset.
func ScoreWithPositions(text, pattern string) (int, []int) {
	if pattern == "" {
		return 1, nil
	}
	if text == "" {
		return 0, nil
	}

	textLower := strings.ToLower(text)
	patternLower := strings.ToLower(pattern)

	textRunes := []rune(textLower)
	patternRunes := []rune(patternLower)

	return scoreSubsequenceWithPositions(textRunes, patternRunes)
}

// ScoreTermsWithPositions splits the query into whitespace-separated terms and returns
// a combined score and merged match positions if ALL terms match the text.
// Returns 0 and nil if any term fails to match.
func ScoreTermsWithPositions(text, query string) (int, []int) {
	query = strings.TrimSpace(query)
	if query == "" {
		return 1, nil
	}

	terms := strings.Fields(query)
	if len(terms) == 0 {
		return 1, nil
	}

	totalScore := 0
	positionSet := make(map[int]struct{})

	for _, term := range terms {
		score, positions := ScoreWithPositions(text, term)
		if score == 0 {
			return 0, nil // All terms must match
		}
		totalScore += score
		for _, pos := range positions {
			positionSet[pos] = struct{}{}
		}
	}

	// Convert set to sorted slice
	var allPositions []int
	for pos := range positionSet {
		allPositions = append(allPositions, pos)
	}
	slices.Sort(allPositions)

	return totalScore, allPositions
}

// scoreSubsequenceWithPositions finds the best subsequence match and returns a score and positions.
// Returns 0 and nil if no match is found.
// Tries starting from each occurrence of pattern[0] to avoid greedy mismatches.
func scoreSubsequenceWithPositions(text, pattern []rune) (int, []int) {
	if len(pattern) == 0 {
		return 1, nil
	}
	if len(text) < len(pattern) {
		return 0, nil
	}

	bestScore := 0
	var bestPositions []int

	for startIdx := 0; startIdx <= len(text)-len(pattern); startIdx++ {
		if text[startIdx] != pattern[0] {
			continue
		}

		score, positions := tryGreedyFromWithPositions(text, pattern, startIdx)
		if score > bestScore {
			bestScore = score
			bestPositions = positions
		}
	}

	return bestScore, bestPositions
}

// tryGreedyFromWithPositions attempts a greedy subsequence match starting from startIdx.
func tryGreedyFromWithPositions(text, pattern []rune, startIdx int) (int, []int) {
	patternIdx := 0
	score := 0
	prevMatchIdx := -2
	firstMatchIdx := -1
	var positions []int

	for textIdx := startIdx; textIdx < len(text) && patternIdx < len(pattern); textIdx++ {
		if text[textIdx] == pattern[patternIdx] {
			positions = append(positions, textIdx)
			matchScore := 1

			if firstMatchIdx == -1 {
				firstMatchIdx = textIdx
			}

			if textIdx == prevMatchIdx+1 {
				matchScore += 3
			}

			if textIdx == 0 {
				matchScore += 2
			} else if isWordBoundary(text[textIdx-1]) {
				matchScore += 2
			}

			if patternIdx == 0 && textIdx == 0 {
				matchScore += 1
			}

			score += matchScore
			prevMatchIdx = textIdx
			patternIdx++
		}
	}

	if patternIdx < len(pattern) {
		return 0, nil
	}

	// Reject matches that are too scattered (span > 3x pattern length)
	matchSpan := prevMatchIdx - firstMatchIdx + 1
	if matchSpan > len(pattern)*3 {
		return 0, nil
	}

	lengthDiff := len(text) - len(pattern)
	if lengthDiff == 0 {
		score += 10
	} else if lengthDiff <= 3 {
		score += 5
	}

	return score, positions
}

// scoreSubsequence finds the best subsequence match and returns a score.
// Returns 0 if no match is found.
// Tries starting from each occurrence of pattern[0] to avoid greedy mismatches.
func scoreSubsequence(text, pattern []rune) int {
	if len(pattern) == 0 {
		return 1
	}
	if len(text) < len(pattern) {
		return 0
	}

	bestScore := 0

	for startIdx := 0; startIdx <= len(text)-len(pattern); startIdx++ {
		if text[startIdx] != pattern[0] {
			continue
		}

		score := tryGreedyFrom(text, pattern, startIdx)
		if score > bestScore {
			bestScore = score
		}
	}

	return bestScore
}

// tryGreedyFrom attempts a greedy subsequence match starting from startIdx.
func tryGreedyFrom(text, pattern []rune, startIdx int) int {
	patternIdx := 0
	score := 0
	prevMatchIdx := -2
	firstMatchIdx := -1

	for textIdx := startIdx; textIdx < len(text) && patternIdx < len(pattern); textIdx++ {
		if text[textIdx] == pattern[patternIdx] {
			matchScore := 1

			if firstMatchIdx == -1 {
				firstMatchIdx = textIdx
			}

			if textIdx == prevMatchIdx+1 {
				matchScore += 3
			}

			if textIdx == 0 {
				matchScore += 2
			} else if isWordBoundary(text[textIdx-1]) {
				matchScore += 2
			}

			if patternIdx == 0 && textIdx == 0 {
				matchScore += 1
			}

			score += matchScore
			prevMatchIdx = textIdx
			patternIdx++
		}
	}

	if patternIdx < len(pattern) {
		return 0
	}

	// Reject matches that are too scattered (span > 3x pattern length)
	matchSpan := prevMatchIdx - firstMatchIdx + 1
	if matchSpan > len(pattern)*3 {
		return 0
	}

	lengthDiff := len(text) - len(pattern)
	if lengthDiff == 0 {
		score += 10
	} else if lengthDiff <= 3 {
		score += 5
	}

	return score
}

// isWordBoundary returns true if the rune is a word boundary character.
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || r == '-' || r == '_' || r == '/' || r == '.'
}

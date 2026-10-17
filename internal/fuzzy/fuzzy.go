// Package fuzzy provides fzf-style fuzzy matching with scoring.
package fuzzy

import (
	"slices"
	"strings"
	"unicode"
)

// Score returns a case-insensitive match score for the pattern against the text.
// Returns 0 if there is no match.
// Higher scores indicate better matches based on:
//   - Consecutive character matches (+3 bonus)
//   - Word boundary matches (+2 bonus)
//   - Match at start of text (+2 bonus, plus +1 when the pattern's first
//     character lands there)
//   - Base score of 1 per matched character
//
// Shorter texts are then preferred as a tie-break worth at most 3.
func Score(text, pattern string) int {
	return Matcher{}.Score(text, pattern)
}

// Score returns a match score for the pattern against the text under the
// matcher's case sensitivity. Returns 0 if there is no match.
func (m Matcher) Score(text, pattern string) int {
	if pattern == "" {
		return 1 // Empty pattern matches everything with minimal score
	}
	if text == "" {
		return 0
	}

	textOriginalRunes := []rune(text)
	textRunes := []rune(m.fold(text))
	patternRunes := []rune(m.fold(pattern))

	return scoreSubsequence(textRunes, textOriginalRunes, patternRunes)
}

// ScoreTerms splits the query into whitespace-separated terms and returns
// a combined case-insensitive score if ALL terms match the text. Returns 0 if
// any term fails to match.
// This preserves AND logic for multi-word queries like "need way".
func ScoreTerms(text, query string) int {
	return Matcher{}.ScoreTerms(text, query)
}

// ScoreTerms splits the query into whitespace-separated terms and returns a
// combined score if ALL terms match the text under the matcher's case
// sensitivity. Returns 0 if any term fails to match.
func (m Matcher) ScoreTerms(text, query string) int {
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
		score := m.Score(text, term)
		if score == 0 {
			return 0 // All terms must match
		}
		totalScore += score
	}

	return totalScore
}

// ScoreWithPositions returns both a match score and the indices in text where
// matches occurred, under the matcher's case sensitivity.
// Returns 0 score and nil positions if there is no match.
// Positions are 0-indexed rune positions in the folded text. A case-sensitive
// matcher folds nothing, so the indices are positions in []rune(text) directly.
// Otherwise Go's strings.ToLower performs a 1:1 rune mapping (it does not apply
// locale-specific SpecialCase tables that expand a rune into multiple runes), so
// the lowercased rune sequence has the same length as []rune(text) for all
// default inputs. Callers can therefore safely use these indices on
// []rune(text). If a caller pre-applies a locale-specific case mapping (e.g.
// Turkish) before calling this function, that 1:1 invariant no longer holds and
// positions should be interpreted as indices into the (already-lowercased) rune
// sequence passed in.
func (m Matcher) ScoreWithPositions(text, pattern string) (int, []int) {
	if pattern == "" {
		return 1, nil
	}
	if text == "" {
		return 0, nil
	}

	textOriginalRunes := []rune(text)
	textRunes := []rune(m.fold(text))
	patternRunes := []rune(m.fold(pattern))

	return scoreSubsequenceWithPositions(textRunes, textOriginalRunes, patternRunes)
}

// ScoreTermsWithPositions splits the query into whitespace-separated terms and
// returns a combined score and merged match positions if ALL terms match the
// text under the matcher's case sensitivity.
// Returns 0 and nil if any term fails to match.
func (m Matcher) ScoreTermsWithPositions(text, query string) (int, []int) {
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
		score, positions := m.ScoreWithPositions(text, term)
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
func scoreSubsequenceWithPositions(text, originalText, pattern []rune) (int, []int) {
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

		score, positions := scoreFromStart(text, originalText, pattern, startIdx, true)
		if score > bestScore {
			bestScore = score
			bestPositions = positions
		}
	}

	return bestScore, bestPositions
}

// scoreSubsequence finds the best subsequence match and returns a score.
// Returns 0 if no match is found.
// Tries starting from each occurrence of pattern[0] to avoid greedy mismatches.
func scoreSubsequence(text, originalText, pattern []rune) int {
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

		score, _ := scoreFromStart(text, originalText, pattern, startIdx, false)
		if score > bestScore {
			bestScore = score
		}
	}

	return bestScore
}

// scoreFromStart finds the best scoring subsequence that starts at startIdx.
// It only considers matches inside the existing density window
// (span <= 3x pattern length).
func scoreFromStart(text, originalText, pattern []rune, startIdx int, wantPositions bool) (int, []int) {
	maxEnd := min(len(text)-1, startIdx+len(pattern)*3-1)
	if maxEnd-startIdx+1 < len(pattern) {
		return 0, nil
	}
	if score, positions, ok := scoreConsecutiveFrom(text, originalText, pattern, startIdx, wantPositions); ok {
		return score, positions
	}

	windowLen := maxEnd - startIdx + 1
	// Most interactive queries fit on the stack. Long patterns still use their
	// full density window; this changes allocation, never matching semantics.
	var scratch [192]int
	var dpPrev, dpCurr []int
	if windowLen <= len(scratch)/2 {
		dpPrev, dpCurr = scratch[:windowLen], scratch[len(scratch)/2:len(scratch)/2+windowLen]
	} else {
		dpPrev, dpCurr = make([]int, windowLen), make([]int, windowLen)
	}
	dpPrev[0] = matchScore(originalText, startIdx, 0, -2)

	var predecessors [][]int
	if wantPositions {
		predecessors = make([][]int, len(pattern))
	}

	for patternIdx := 1; patternIdx < len(pattern); patternIdx++ {
		clear(dpCurr)
		if wantPositions {
			predecessors[patternIdx] = make([]int, windowLen)
			for i := range predecessors[patternIdx] {
				predecessors[patternIdx][i] = -1
			}
		}

		bestPrevScore := 0
		bestPrevIdx := -1
		latestIdx := maxEnd - (len(pattern) - 1 - patternIdx)

		for textIdx := startIdx + patternIdx; textIdx <= latestIdx; textIdx++ {
			relIdx := textIdx - startIdx
			prevIdx := textIdx - 1
			prevRelIdx := relIdx - 1
			if dpPrev[prevRelIdx] > bestPrevScore {
				bestPrevScore = dpPrev[prevRelIdx]
				bestPrevIdx = prevIdx
			}
			if text[textIdx] != pattern[patternIdx] || bestPrevScore == 0 {
				continue
			}

			baseScore := matchScore(originalText, textIdx, patternIdx, -2)
			score := bestPrevScore + baseScore
			predecessor := bestPrevIdx

			if dpPrev[prevRelIdx] > 0 {
				consecutiveScore := dpPrev[prevRelIdx] + baseScore + 3
				if consecutiveScore > score {
					score = consecutiveScore
					predecessor = prevIdx
				}
			}

			dpCurr[relIdx] = score
			if wantPositions {
				predecessors[patternIdx][relIdx] = predecessor
			}
		}

		dpPrev, dpCurr = dpCurr, dpPrev
	}

	bestScore := 0
	bestEndIdx := -1
	for textIdx := startIdx + len(pattern) - 1; textIdx <= maxEnd; textIdx++ {
		if dpPrev[textIdx-startIdx] > bestScore {
			bestScore = dpPrev[textIdx-startIdx]
			bestEndIdx = textIdx
		}
	}
	if bestScore == 0 {
		return 0, nil
	}

	bestScore = addLengthBonus(bestScore, len(text), len(pattern))
	if !wantPositions {
		return bestScore, nil
	}

	positions := make([]int, len(pattern))
	positions[len(pattern)-1] = bestEndIdx
	for patternIdx := len(pattern) - 1; patternIdx > 0; patternIdx-- {
		positions[patternIdx-1] = predecessors[patternIdx][positions[patternIdx]-startIdx]
	}

	return bestScore, positions
}

func scoreConsecutiveFrom(text, originalText, pattern []rune, startIdx int, wantPositions bool) (int, []int, bool) {
	if startIdx+len(pattern) > len(text) {
		return 0, nil, false
	}

	score := 0
	prevMatchIdx := -2
	var positions []int
	if wantPositions {
		positions = make([]int, len(pattern))
	}

	for patternIdx := range pattern {
		textIdx := startIdx + patternIdx
		if text[textIdx] != pattern[patternIdx] {
			return 0, nil, false
		}
		score += matchScore(originalText, textIdx, patternIdx, prevMatchIdx)
		prevMatchIdx = textIdx
		if wantPositions {
			positions[patternIdx] = textIdx
		}
	}

	return addLengthBonus(score, len(text), len(pattern)), positions, true
}

// The maximum swing here (3) stays strictly below the advantage a start-of-text
// match already earns, so a shorter text can never outrank an earlier match.
func addLengthBonus(score, textLen, patternLen int) int {
	return score + max(0, 3-(textLen-patternLen))
}

func matchScore(text []rune, textIdx, patternIdx, prevMatchIdx int) int {
	score := 1

	if textIdx == prevMatchIdx+1 {
		score += 3
	}

	if textIdx == 0 {
		score += 2
	} else if isWordBoundary(text[textIdx-1]) || isCamelBoundary(text[textIdx-1], text[textIdx]) {
		score += 2
	}

	if patternIdx == 0 && textIdx == 0 {
		score += 1
	}

	return score
}

func isCamelBoundary(prev, curr rune) bool {
	return unicode.IsLower(prev) && unicode.IsUpper(curr)
}

// isWordBoundary returns true if the rune is a word boundary character.
func isWordBoundary(r rune) bool {
	return unicode.IsSpace(r) || unicode.IsPunct(r) || unicode.IsSymbol(r)
}

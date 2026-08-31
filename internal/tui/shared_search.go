package tui

import (
	"strings"
)

// searchSubstringPositions returns the rune positions of the first substring
// match, folding case unless caseSensitive. It answers a render question only —
// which columns to paint — and never decides membership: what a query matches is
// stated once, by the tiered matcher in scoreSearchProjectionTask.
func searchSubstringPositions(text, query string, caseSensitive bool) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	textRunes := []rune(text)
	queryRunes := []rune(query)
	if len(queryRunes) > len(textRunes) {
		return nil
	}
	equal := strings.EqualFold
	if caseSensitive {
		equal = func(a, b string) bool { return a == b }
	}
	for start := 0; start+len(queryRunes) <= len(textRunes); start++ {
		if equal(string(textRunes[start:start+len(queryRunes)]), query) {
			positions := make([]int, len(queryRunes))
			for offset := range queryRunes {
				positions[offset] = start + offset
			}
			return positions
		}
	}
	return nil
}

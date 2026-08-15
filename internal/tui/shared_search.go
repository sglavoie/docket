package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// TaskMatchesSearch reports whether query is a substring of the task's visible
// title or description, folding case unless caseSensitive. Search facets belong
// to the Outliner's dedicated filter grammar and are deliberately not
// interpreted here.
func TaskMatchesSearch(task *models.Task, query string, caseSensitive bool) bool {
	if task == nil {
		return false
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	if caseSensitive {
		return strings.Contains(task.Title, query) || strings.Contains(task.Description, query)
	}
	return strings.Contains(strings.ToLower(task.Title), strings.ToLower(query)) ||
		strings.Contains(strings.ToLower(task.Description), strings.ToLower(query))
}

// searchSubstringPositions returns the rune positions of the first substring
// match, folding case unless caseSensitive. The renderer uses these positions to
// highlight titles without reintroducing fuzzy matching.
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

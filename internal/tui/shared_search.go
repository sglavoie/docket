package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/models"
)

// TaskMatchesSearch reports whether query is a case-insensitive substring of
// the task's visible title or description. Search facets belong to the
// Outliner's dedicated filter grammar and are deliberately not interpreted
// here.
func TaskMatchesSearch(task *models.Task, query string) bool {
	if task == nil {
		return false
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(task.Title), strings.ToLower(query)) ||
		strings.Contains(strings.ToLower(task.Description), strings.ToLower(query))
}

// searchSubstringPositions returns the rune positions of the first
// case-insensitive substring match. The renderer uses these positions to
// highlight titles without reintroducing fuzzy matching.
func searchSubstringPositions(text, query string) []int {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil
	}
	textRunes := []rune(text)
	queryRunes := []rune(query)
	if len(queryRunes) > len(textRunes) {
		return nil
	}
	for start := 0; start+len(queryRunes) <= len(textRunes); start++ {
		if strings.EqualFold(string(textRunes[start:start+len(queryRunes)]), query) {
			positions := make([]int, len(queryRunes))
			for offset := range queryRunes {
				positions[offset] = start + offset
			}
			return positions
		}
	}
	return nil
}

package models

import "strings"

// SearchResult holds a task and its parent project for search results.
type SearchResult struct {
	Task    *Task
	Project *Project
}

// MatchesSearchQuery checks if a task matches the search query (case-insensitive).
// It searches both the title and description for the query string.
// The query should be pre-lowercased for optimal performance.
func (t *Task) MatchesSearchQuery(queryLower string) bool {
	if strings.Contains(strings.ToLower(t.Title), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Description), queryLower) {
		return true
	}
	return false
}

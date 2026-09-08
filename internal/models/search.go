package models

import "strings"

// MatchesSearchQuery checks if a task matches the search query (case-insensitive).
// It searches the title, the description, and the resume note for the query
// string: all three are prose the task itself carries, which is the whole of
// what this matcher covers.
// The query should be pre-lowercased for optimal performance.
func (t *Task) MatchesSearchQuery(queryLower string) bool {
	if strings.Contains(strings.ToLower(t.Title), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(t.Description), queryLower) {
		return true
	}
	if strings.Contains(strings.ToLower(t.ResumeNote), queryLower) {
		return true
	}
	return false
}

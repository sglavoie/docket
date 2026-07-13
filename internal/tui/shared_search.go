package tui

import (
	"strings"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// TaskMatchesSearch checks if a task matches the search query by fuzzy-matching
// the title, tags, description, and comments.
func TaskMatchesSearch(task *models.Task, query string) bool {
	q := strings.TrimSpace(query)
	if q == "" {
		return true
	}
	if fuzzy.MatchTerms(task.Title, q) {
		return true
	}
	for _, tag := range task.Tags {
		if fuzzy.MatchTerms(tag, q) {
			return true
		}
	}
	if fuzzy.MatchTerms(task.Description, q) {
		return true
	}
	for _, comment := range task.Comments {
		if fuzzy.MatchTerms(comment.Text, q) {
			return true
		}
	}
	return false
}

// TaskMatchesStructuredSearch parses structured filters from the query, applies
// them, and then fuzzy-matches the remaining text portion. The config resolves
// Focus Queue membership for active-flag filters.
func TaskMatchesStructuredSearch(task *models.Task, query string, cfg *config.Config) bool {
	parsed := ParseSearchQuery(query)
	if !TaskMatchesFilters(task, parsed, cfg) {
		return false
	}
	return TaskMatchesSearch(task, parsed.TextQuery)
}

// ExtractSearchTextQuery returns only the text portion of a search query,
// with all structured filter tokens removed.
func ExtractSearchTextQuery(query string) string {
	return ParseSearchQuery(query).TextQuery
}

package tui

import (
	"strings"

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

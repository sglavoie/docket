package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// filteredSearchResult holds a search result with match positions for highlighting.
type filteredSearchResult struct {
	result         models.SearchResult
	titlePositions []int // Match positions within the title for highlighting
}

// filterTasks filters tasks based on the current search query.
func (m *GlobalSearchModel) filterTasks() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Parse structured filters from query
	m.parsedFilters = ParseSearchQuery(query)

	// Empty query shows no results (unless filters are active)
	if query == "" {
		m.filtered = nil
		m.cursor = 0
		return
	}

	// Score each task and collect matches with title positions
	type scoredResult struct {
		result         models.SearchResult
		score          int
		titlePositions []int
	}
	var scored []scoredResult

	for _, result := range m.allTasks {
		if result.IsNote() {
			// Score notes by title and body
			textQuery := m.parsedFilters.TextQuery
			if textQuery != "" {
				score, titlePositions := m.scoreNote(result.Note, textQuery)
				if score > 0 {
					scored = append(scored, scoredResult{
						result:         result,
						score:          score,
						titlePositions: titlePositions,
					})
				}
			}
			continue
		}

		// First apply structured filters
		if !TaskMatchesFilters(result.Task, m.parsedFilters) {
			continue
		}

		// If there's a text query, apply fuzzy scoring
		textQuery := m.parsedFilters.TextQuery
		if textQuery != "" {
			score, titlePositions := m.scoreTask(result.Task, textQuery)
			if score > 0 {
				scored = append(scored, scoredResult{
					result:         result,
					score:          score,
					titlePositions: titlePositions,
				})
			}
		} else {
			// No text query but filters matched - include with base score
			scored = append(scored, scoredResult{
				result:         result,
				score:          1,
				titlePositions: nil,
			})
		}
	}

	// Sort by score descending (best matches first)
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Extract sorted results with positions
	m.filtered = make([]filteredSearchResult, len(scored))
	for i, s := range scored {
		m.filtered[i] = filteredSearchResult{
			result:         s.result,
			titlePositions: s.titlePositions,
		}
	}

	// Reset cursor if out of bounds
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// scoreNote returns a fuzzy match score for a note against the query.
func (m *GlobalSearchModel) scoreNote(note *models.Note, query string) (int, []int) {
	titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(note.Title, query)
	if titleScore > 0 {
		titleScore *= 2
	}
	bodyScore := fuzzy.ScoreTerms(note.Body, query)
	if titleScore > bodyScore {
		return titleScore, titlePositions
	}
	return bodyScore, nil
}

// scoreTask returns a fuzzy match score for a task against the query,
// along with match positions within the title for highlighting.
// Returns 0 and nil if no match, higher scores indicate better matches.
func (m *GlobalSearchModel) scoreTask(task *models.Task, query string) (int, []int) {
	// Score against title (weighted higher)
	titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	if titleScore > 0 {
		titleScore *= 2 // Title matches are more important
	}

	// Score against description
	descScore := fuzzy.ScoreTerms(task.Description, query)

	// Return the best score with title positions (for highlighting)
	if titleScore > descScore {
		return titleScore, titlePositions
	}
	return descScore, nil // No title highlighting if description matched better
}

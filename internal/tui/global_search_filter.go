package tui

import (
	"sort"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// noteMatchesFilters checks applicable has:/no:/is: filters against a note.
// Task-only filters (checklist, attachment, dependency, etc.) auto-pass no:
// and auto-fail has:.
func noteMatchesFilters(note *models.Note, filters *ParsedSearchQuery) bool {
	for _, f := range filters.HasPresence {
		switch f {
		case "tag":
			if len(note.Tags) == 0 {
				return false
			}
		case "description":
			if len(note.Body) == 0 {
				return false
			}
		default:
			// Task-only has: filter: note doesn't have it → fail
			return false
		}
	}
	for _, f := range filters.NoPresence {
		switch f {
		case "tag":
			if len(note.Tags) > 0 {
				return false
			}
		case "description":
			if len(note.Body) > 0 {
				return false
			}
		default:
			// Task-only no: filter: note doesn't have it → pass (vacuously true)
		}
	}
	for _, f := range filters.IsState {
		switch f {
		case "pinned":
			if !note.Pinned {
				return false
			}
		default:
			// Task-only is: filter: note doesn't have this state → fail
			return false
		}
	}
	return true
}

// filteredSearchResult holds a search result with match positions for highlighting.
type filteredSearchResult struct {
	result            models.SearchResult
	titlePositions    []int  // Match positions within the title for highlighting
	matchSource       string // "title", "description", or "comment"
	matchedCommentIdx int    // Index of the matched comment (when matchSource == "comment")
}

// filterTasks filters tasks based on the current search query.
func (m *GlobalSearchModel) filterTasks() {
	query := strings.TrimSpace(m.searchInput.Value())

	// Parse structured filters from query
	m.parsedFilters = ParseSearchQuery(query)

	// Empty query shows no results unless hideCompleted is on, filters are active,
	// or we're on the Notes tab (which always shows available notes).
	if query == "" && !m.hideCompleted && m.tabs.Active != globalSearchTabNotes {
		m.filtered = nil
		m.cursor = 0
		return
	}

	// Score each task and collect matches with title positions
	type scoredResult struct {
		result            models.SearchResult
		score             int
		titlePositions    []int
		matchSource       string
		matchedCommentIdx int
	}
	var scored []scoredResult

	for _, result := range m.allTasks {
		// Apply tab filter
		if m.tabs.Active == globalSearchTabTasks && result.IsNote() {
			continue
		}
		if m.tabs.Active == globalSearchTabNotes && !result.IsNote() {
			continue
		}

		// Hide completed/canceled tasks when toggle is on and no explicit status filter
		if m.hideCompleted && !result.IsNote() && len(m.parsedFilters.Statuses) == 0 {
			if result.Task.Status.IsComplete() {
				continue
			}
		}

		// Apply project slug filter to both tasks and notes
		if m.parsedFilters.ProjectSlug != "" {
			slug := strings.ToLower(result.Project.Slug)
			if !strings.HasPrefix(slug, m.parsedFilters.ProjectSlug) {
				continue
			}
		}

		if result.IsNote() {
			// Apply applicable has:/no:/is: filters to notes
			if !noteMatchesFilters(result.Note, m.parsedFilters) {
				continue
			}

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
			} else {
				// No text query - include all notes with base score
				scored = append(scored, scoredResult{
					result: result,
					score:  1,
				})
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
			score, titlePositions, matchSource, matchedCommentIdx := m.scoreTask(result.Task, textQuery)
			if score > 0 {
				scored = append(scored, scoredResult{
					result:            result,
					score:             score,
					titlePositions:    titlePositions,
					matchSource:       matchSource,
					matchedCommentIdx: matchedCommentIdx,
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
			result:            s.result,
			titlePositions:    s.titlePositions,
			matchSource:       s.matchSource,
			matchedCommentIdx: s.matchedCommentIdx,
		}
	}

	m.cursor = ClampCursor(m.cursor, len(m.filtered))
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
// Also returns the match source ("title", "description", or "comment") and
// the index of the best-matching comment when applicable.
// Returns 0, nil, "", -1 if no match; higher scores indicate better matches.
func (m *GlobalSearchModel) scoreTask(task *models.Task, query string) (int, []int, string, int) {
	// Score against title (weighted higher)
	titleScore, titlePositions := fuzzy.ScoreTermsWithPositions(task.Title, query)
	if titleScore > 0 {
		titleScore *= 2 // Title matches are more important
	}

	// Score against description
	descScore := fuzzy.ScoreTerms(task.Description, query)

	// Score against comments (same weight as description)
	bestCommentScore := 0
	bestCommentIdx := -1
	for i, comment := range task.Comments {
		score := fuzzy.ScoreTerms(comment.Text, query)
		if score > bestCommentScore {
			bestCommentScore = score
			bestCommentIdx = i
		}
	}

	// Return the best score with source info
	bestScore := titleScore
	bestSource := "title"
	bestPositions := titlePositions

	if descScore > bestScore {
		bestScore = descScore
		bestSource = "description"
		bestPositions = nil
	}
	if bestCommentScore > bestScore {
		bestScore = bestCommentScore
		bestSource = "comment"
		bestPositions = nil
	}

	if bestScore == 0 {
		return 0, nil, "", -1
	}
	if bestSource != "comment" {
		bestCommentIdx = -1
	}
	return bestScore, bestPositions, bestSource, bestCommentIdx
}

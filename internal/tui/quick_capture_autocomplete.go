package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// getTagSuggestions returns matching tags for the partial query.
func (q *quickCaptureInput) getTagSuggestions(partial string) []ACSuggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range q.tagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top matches (no hard limit, scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, ACSuggestion{
			Display: "#" + s.tag,
			Value:   s.tag,
		})
	}
	return suggestions
}

// getTypeSuggestions returns matching task type options for the partial query.
func (q *quickCaptureInput) getTypeSuggestions(partial string) []ACSuggestion {
	types := []struct {
		name string
		icon string
	}{
		{"epic", "◆"},
		{"story", "◇"},
		{"task", "□"},
		{"bug", "⚠"},
		{"idea", "💡"},
		{"note", "📝"},
	}

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("^%s %s", t.name, t.icon),
				Value:   t.name,
			})
		}
	}
	return suggestions
}

// getPrioritySuggestions returns matching priority options for the partial query.
func (q *quickCaptureInput) getPrioritySuggestions(partial string) []ACSuggestion {
	priorities := []struct {
		value string
		desc  string
	}{
		{"1", "Urgent"},
		{"2", "High"},
		{"3", "Normal"},
		{"4", "Low"},
		{"5", "Lowest"},
	}

	var suggestions []ACSuggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				Value:   p.value,
			})
		}
	}
	return suggestions
}

// getDateSuggestions returns common due date options for the partial query.
func (q *quickCaptureInput) getDateSuggestions(partial string) []ACSuggestion {
	options := []struct {
		value string
		desc  string
	}{
		{"today", "today"},
		{"tomorrow", "tomorrow"},
		{"+1d", "in 1 day"},
		{"+3d", "in 3 days"},
		{"+1w", "in 1 week"},
		{"+2w", "in 2 weeks"},
		{"+1m", "in 1 month"},
		{"monday", "next Monday"},
		{"friday", "next Friday"},
		{"next monday", "next Monday (7+ days)"},
		{"next friday", "next Friday (7+ days)"},
		{"in 3 days", "3 days from now"},
		{"in 1 week", "1 week from now"},
		{"in 2 weeks", "2 weeks from now"},
	}

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("@%s (%s)", o.value, o.desc),
				Value:   o.value,
			})
		}
	}
	return suggestions
}

// getRecurrenceSuggestions returns recurrence pattern options for the partial query.
func (q *quickCaptureInput) getRecurrenceSuggestions(partial string) []ACSuggestion {
	options := []struct {
		value string
		desc  string
	}{
		{"daily", "every day"},
		{"weekly", "every week"},
		{"biweekly", "every 2 weeks"},
		{"monthly", "every month"},
		{"2d", "every 2 days"},
		{"3d", "every 3 days"},
		{"2w", "every 2 weeks"},
		{"3w", "every 3 weeks"},
	}

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("~%s (%s)", o.value, o.desc),
				Value:   o.value,
			})
		}
	}
	return suggestions
}

// getSectionSuggestions returns matching sections for the partial query.
func (q *quickCaptureInput) getSectionSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, section := range q.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "/" + section,
				Value:   section,
			})
		}
	}
	return suggestions
}

// getStatusSuggestions returns matching status options for the partial query.
func (q *quickCaptureInput) getStatusSuggestions(partial string) []ACSuggestion {
	statuses := []struct {
		name string
		icon string
		desc string
	}{
		{"start", "◐", "in progress + activate"},
		{"review", "◑", "send to review"},
		{"standby", "⏸", "put on hold"},
		{"maybe", "❓", "tentative / someday"},
		{"blocked", "◈", "blocked"},
		{"done", "●", "completed"},
		{"canceled", "✕", "canceled"},
	}

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("$%s %s (%s)", s.name, s.icon, s.desc),
				Value:   s.name,
			})
		}
	}
	return suggestions
}

// getParentSuggestions returns matching parent tasks for the partial query.
func (q *quickCaptureInput) getParentSuggestions(partial string) []ACSuggestion {
	type scoredParent struct {
		task  *models.Task
		score int
	}

	var scored []scoredParent
	for _, task := range q.parentCache {
		// Build searchable text: "#ShortID Type Title"
		searchText := fmt.Sprintf("#%d %s %s", task.ShortID, task.Type.HeadingPrefix(), task.Title)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredParent{task: task, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		// Truncate title if too long (width-aware: ~15 chars overhead for icon+id+spaces)
		titleLimit := max(q.width-15, 10)
		title := truncateToVisualWidth(task.Title, titleLimit)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%s #%d %s", task.Type.Icon(), task.ShortID, title),
			Value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}

// getProjectSuggestions returns matching projects for the partial query.
func (q *quickCaptureInput) getProjectSuggestions(partial string) []ACSuggestion {
	type scoredProject struct {
		project *models.Project
		score   int
	}

	var scored []scoredProject
	for _, project := range q.projectCache {
		// Build searchable text: "Name [context]"
		searchText := fmt.Sprintf("%s %s", project.Name, project.Context)
		score := fuzzy.Score(searchText, partial)
		if score > 0 {
			scored = append(scored, scoredProject{project: project, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return all matches (scrollable)
	suggestions := make([]ACSuggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		// Truncate name if too long (width-aware: ~20 chars overhead for context+brackets+spaces)
		nameLimit := max(q.width-20, 10)
		name := truncateToVisualWidth(project.Name, nameLimit)
		suggestions = append(suggestions, ACSuggestion{
			Display: fmt.Sprintf("%%%s [%s]", name, project.Context),
			Value:   project.Slug,
		})
	}
	return suggestions
}

// acceptSuggestion inserts the selected suggestion into the input.
func (q *quickCaptureInput) acceptSuggestion() {
	if !q.ac.IsActive() {
		return
	}

	triggerChar := TriggerCharForMode(q.ac.Mode, q.triggers)
	newInput := q.ac.BuildAcceptedInput(q.input.Value(), triggerChar)

	q.input.SetValue(newInput)
	// Move cursor to end
	q.input.CursorEnd()

	// Reset autocomplete state
	q.ac.Clear()
}

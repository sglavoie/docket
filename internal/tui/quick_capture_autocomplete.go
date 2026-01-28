package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
)

// detectAutocompleteTrigger scans the input for autocomplete triggers.
// Returns the trigger type, trigger position, and partial text after trigger.
func (q *quickCaptureInput) detectAutocompleteTrigger() (int, int, string) {
	input := q.input.Value()
	if len(input) == 0 {
		return acModeNone, 0, ""
	}

	// Scan backward from end to find most recent trigger
	// Triggers only activate if at start of input or preceded by whitespace
	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			// Hit whitespace before finding trigger
			break
		}
		// Check if trigger is at start or preceded by whitespace
		atStartOrAfterSpace := i == 0 || input[i-1] == ' ' || input[i-1] == '\n'
		if !atStartOrAfterSpace {
			continue
		}
		partial := input[i+1:]
		switch ch {
		case '#':
			return acModeTag, i, partial
		case '>':
			return acModeParent, i, partial
		case '^':
			return acModeType, i, partial
		case '!':
			return acModePriority, i, partial
		case '@':
			return acModeDate, i, partial
		case '~':
			return acModeRecur, i, partial
		case '/':
			return acModeSection, i, partial
		case '$':
			return acModeStatus, i, partial
		case '%':
			return acModeProject, i, partial
		}
	}
	return acModeNone, 0, ""
}

// getTagSuggestions returns matching tags for the partial query.
func (q *quickCaptureInput) getTagSuggestions(partial string) []suggestion {
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
	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		suggestions = append(suggestions, suggestion{
			display: "#" + s.tag,
			value:   s.tag,
		})
	}
	return suggestions
}

// getTypeSuggestions returns matching task type options for the partial query.
func (q *quickCaptureInput) getTypeSuggestions(partial string) []suggestion {
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

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("^%s %s", t.name, t.icon),
				value:   t.name,
			})
		}
	}
	return suggestions
}

// getPrioritySuggestions returns matching priority options for the partial query.
func (q *quickCaptureInput) getPrioritySuggestions(partial string) []suggestion {
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

	var suggestions []suggestion
	for _, p := range priorities {
		if partial == "" || strings.HasPrefix(p.value, partial) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("!%s (%s)", p.value, p.desc),
				value:   p.value,
			})
		}
	}
	return suggestions
}

// getDateSuggestions returns common due date options for the partial query.
func (q *quickCaptureInput) getDateSuggestions(partial string) []suggestion {
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

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("@%s (%s)", o.value, o.desc),
				value:   o.value,
			})
		}
	}
	return suggestions
}

// getRecurrenceSuggestions returns recurrence pattern options for the partial query.
func (q *quickCaptureInput) getRecurrenceSuggestions(partial string) []suggestion {
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

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.value, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("~%s (%s)", o.value, o.desc),
				value:   o.value,
			})
		}
	}
	return suggestions
}

// getSectionSuggestions returns matching sections for the partial query.
func (q *quickCaptureInput) getSectionSuggestions(partial string) []suggestion {
	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, section := range q.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, suggestion{
				display: "/" + section,
				value:   section,
			})
		}
	}
	return suggestions
}

// getStatusSuggestions returns matching status options for the partial query.
func (q *quickCaptureInput) getStatusSuggestions(partial string) []suggestion {
	statuses := []struct {
		name string
		icon string
		desc string
	}{
		{"start", "◐", "in progress + activate"},
		{"blocked", "◈", "blocked"},
		{"done", "●", "completed"},
		{"canceled", "✕", "canceled"},
	}

	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, suggestion{
				display: fmt.Sprintf("$%s %s (%s)", s.name, s.icon, s.desc),
				value:   s.name,
			})
		}
	}
	return suggestions
}

// getParentSuggestions returns matching parent tasks for the partial query.
func (q *quickCaptureInput) getParentSuggestions(partial string) []suggestion {
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
	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		task := s.task
		// Truncate title if too long
		title := task.Title
		if len(title) > 30 {
			title = title[:27] + "..."
		}
		suggestions = append(suggestions, suggestion{
			display: fmt.Sprintf("%s #%d %s", task.Type.Icon(), task.ShortID, title),
			value:   strconv.Itoa(task.ShortID),
		})
	}
	return suggestions
}

// getProjectSuggestions returns matching projects for the partial query.
func (q *quickCaptureInput) getProjectSuggestions(partial string) []suggestion {
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
	suggestions := make([]suggestion, 0, len(scored))
	for _, s := range scored {
		project := s.project
		// Truncate name if too long
		name := project.Name
		if len(name) > 25 {
			name = name[:22] + "..."
		}
		suggestions = append(suggestions, suggestion{
			display: fmt.Sprintf("%%%s [%s]", name, project.Context),
			value:   project.Slug,
		})
	}
	return suggestions
}

// acceptSuggestion inserts the selected suggestion into the input.
func (q *quickCaptureInput) acceptSuggestion() {
	if q.acMode == acModeNone || len(q.suggestions) == 0 {
		return
	}

	selected := q.suggestions[q.acCursor]
	input := q.input.Value()

	// Replace from trigger position to end with the completed value
	var newInput string
	switch q.acMode {
	case acModeTag:
		newInput = input[:q.acTriggerPos] + "#" + selected.value + " "
	case acModeParent:
		newInput = input[:q.acTriggerPos] + ">" + selected.value + " "
	case acModeType:
		newInput = input[:q.acTriggerPos] + "^" + selected.value + " "
	case acModePriority:
		newInput = input[:q.acTriggerPos] + "!" + selected.value + " "
	case acModeDate:
		newInput = input[:q.acTriggerPos] + "@" + selected.value + " "
	case acModeRecur:
		newInput = input[:q.acTriggerPos] + "~" + selected.value + " "
	case acModeSection:
		newInput = input[:q.acTriggerPos] + "/" + selected.value + " "
	case acModeStatus:
		newInput = input[:q.acTriggerPos] + "$" + selected.value + " "
	case acModeProject:
		newInput = input[:q.acTriggerPos] + "%" + selected.value + " "
	}

	q.input.SetValue(newInput)
	// Move cursor to end
	q.input.CursorEnd()

	// Reset autocomplete state
	q.acMode = acModeNone
	q.suggestions = nil
	q.acCursor = 0
	q.acViewportTop = 0
}

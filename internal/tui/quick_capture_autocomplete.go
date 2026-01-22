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
func (m *QuickCaptureModel) detectAutocompleteTrigger() (int, int, string) {
	input := m.input.Value()
	if len(input) == 0 {
		return acModeNone, 0, ""
	}

	// Scan backward from end to find most recent trigger
	for i := len(input) - 1; i >= 0; i-- {
		ch := input[i]
		if ch == ' ' || ch == '\n' {
			// Hit whitespace before finding trigger
			break
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
		}
	}
	return acModeNone, 0, ""
}

// getTagSuggestions returns matching tags for the partial query.
func (m *QuickCaptureModel) getTagSuggestions(partial string) []suggestion {
	type scoredTag struct {
		tag   string
		score int
	}

	var scored []scoredTag
	for _, tag := range m.tagCache {
		score := fuzzy.Score(tag, partial)
		if score > 0 {
			scored = append(scored, scoredTag{tag: tag, score: score})
		}
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// Return top 5
	suggestions := make([]suggestion, 0, 5)
	for i := 0; i < len(scored) && i < 5; i++ {
		suggestions = append(suggestions, suggestion{
			display: "#" + scored[i].tag,
			value:   scored[i].tag,
		})
	}
	return suggestions
}

// getTypeSuggestions returns matching task type options for the partial query.
func (m *QuickCaptureModel) getTypeSuggestions(partial string) []suggestion {
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
func (m *QuickCaptureModel) getPrioritySuggestions(partial string) []suggestion {
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
func (m *QuickCaptureModel) getDateSuggestions(partial string) []suggestion {
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
func (m *QuickCaptureModel) getRecurrenceSuggestions(partial string) []suggestion {
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
func (m *QuickCaptureModel) getSectionSuggestions(partial string) []suggestion {
	var suggestions []suggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, suggestion{
				display: "/" + section,
				value:   section,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getStatusSuggestions returns matching status options for the partial query.
func (m *QuickCaptureModel) getStatusSuggestions(partial string) []suggestion {
	statuses := []struct {
		name string
		icon string
		desc string
	}{
		{"start", "◐", "in progress + activate"},
		{"blocked", "◈", "blocked"},
		{"done", "●", "completed"},
		{"cancelled", "✕", "cancelled"},
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
func (m *QuickCaptureModel) getParentSuggestions(partial string) []suggestion {
	type scoredParent struct {
		task  *models.Task
		score int
	}

	var scored []scoredParent
	for _, task := range m.parentCache {
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

	// Return top 5
	suggestions := make([]suggestion, 0, 5)
	for i := 0; i < len(scored) && i < 5; i++ {
		task := scored[i].task
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

// acceptSuggestion inserts the selected suggestion into the input.
func (m *QuickCaptureModel) acceptSuggestion() {
	if m.acMode == acModeNone || len(m.suggestions) == 0 {
		return
	}

	selected := m.suggestions[m.acCursor]
	input := m.input.Value()

	// Replace from trigger position to end with the completed value
	var newInput string
	switch m.acMode {
	case acModeTag:
		newInput = input[:m.acTriggerPos] + "#" + selected.value + " "
	case acModeParent:
		newInput = input[:m.acTriggerPos] + ">" + selected.value + " "
	case acModeType:
		newInput = input[:m.acTriggerPos] + "^" + selected.value + " "
	case acModePriority:
		newInput = input[:m.acTriggerPos] + "!" + selected.value + " "
	case acModeDate:
		newInput = input[:m.acTriggerPos] + "@" + selected.value + " "
	case acModeRecur:
		newInput = input[:m.acTriggerPos] + "~" + selected.value + " "
	case acModeSection:
		newInput = input[:m.acTriggerPos] + "/" + selected.value + " "
	case acModeStatus:
		newInput = input[:m.acTriggerPos] + "$" + selected.value + " "
	}

	m.input.SetValue(newInput)
	// Move cursor to end
	m.input.CursorEnd()

	// Reset autocomplete state
	m.acMode = acModeNone
	m.suggestions = nil
	m.acCursor = 0
}

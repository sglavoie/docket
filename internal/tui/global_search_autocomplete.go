package tui

import (
	"fmt"
	"strings"
)

// Autocomplete mode constants for global search
const (
	acSearchModeNone    = iota // No autocomplete active
	acSearchModeTag            // Completing #tag
	acSearchModeSection        // Completing /section
	acSearchModeStatus         // Completing $status
	acSearchModeType           // Completing ^type
	acSearchModeDueDate        // Completing @duedate
)

// searchSuggestion represents an autocomplete suggestion for global search.
type searchSuggestion struct {
	display string // Display text with icon/formatting
	value   string // Raw value to insert
}

// detectAutocomplete checks if the cursor is after a filter trigger character
// and populates suggestions accordingly.
func (m *GlobalSearchModel) detectAutocomplete() {
	input := m.searchInput.Value()
	if len(input) == 0 {
		m.acMode = acSearchModeNone
		m.suggestions = nil
		return
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
			m.acMode = acSearchModeTag
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getTagSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '/':
			m.acMode = acSearchModeSection
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getSectionSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '$':
			m.acMode = acSearchModeStatus
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getStatusSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '^':
			m.acMode = acSearchModeType
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getTypeSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		case '@':
			m.acMode = acSearchModeDueDate
			m.acTriggerPos = i
			m.acCursor = 0
			m.suggestions = m.getDueDateSuggestions(partial)
			if len(m.suggestions) == 0 {
				m.acMode = acSearchModeNone
			}
			return
		}
	}

	m.acMode = acSearchModeNone
	m.suggestions = nil
}

// getTagSuggestions returns matching tags for the partial query.
func (m *GlobalSearchModel) getTagSuggestions(partial string) []searchSuggestion {
	var suggestions []searchSuggestion
	partialLower := strings.ToLower(partial)

	for _, tag := range m.tagCache {
		// Show all if partial is empty, otherwise filter by prefix/contains
		if partial == "" || strings.Contains(strings.ToLower(tag), partialLower) {
			suggestions = append(suggestions, searchSuggestion{
				display: "#" + tag,
				value:   tag,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getSectionSuggestions returns matching sections for the partial query.
func (m *GlobalSearchModel) getSectionSuggestions(partial string) []searchSuggestion {
	var suggestions []searchSuggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, searchSuggestion{
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
func (m *GlobalSearchModel) getStatusSuggestions(partial string) []searchSuggestion {
	statuses := []struct {
		name string
		icon string
	}{
		{"todo", "○"},
		{"in_progress", "◐"},
		{"done", "●"},
		{"blocked", "◈"},
		{"canceled", "✕"},
	}

	var suggestions []searchSuggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, searchSuggestion{
				display: fmt.Sprintf("$%s %s", s.name, s.icon),
				value:   s.name,
			})
		}
	}
	return suggestions
}

// getTypeSuggestions returns matching type options for the partial query.
func (m *GlobalSearchModel) getTypeSuggestions(partial string) []searchSuggestion {
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

	var suggestions []searchSuggestion
	partialLower := strings.ToLower(partial)

	for _, t := range types {
		if partial == "" || strings.Contains(t.name, partialLower) {
			suggestions = append(suggestions, searchSuggestion{
				display: fmt.Sprintf("^%s %s", t.name, t.icon),
				value:   t.name,
			})
		}
	}
	return suggestions
}

// getDueDateSuggestions returns due date filter options for the partial query.
func (m *GlobalSearchModel) getDueDateSuggestions(partial string) []searchSuggestion {
	options := []struct {
		name string
		desc string
	}{
		{"today", "due today"},
		{"tomorrow", "due tomorrow"},
		{"week", "due this week"},
		{"overdue", "past due date"},
		{"none", "no due date"},
	}

	var suggestions []searchSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.name, partialLower) {
			suggestions = append(suggestions, searchSuggestion{
				display: fmt.Sprintf("@%s (%s)", o.name, o.desc),
				value:   o.name,
			})
		}
	}
	return suggestions
}

// acceptAutocompleteSuggestion inserts the selected suggestion into the input.
func (m *GlobalSearchModel) acceptAutocompleteSuggestion() {
	if m.acMode == acSearchModeNone || len(m.suggestions) == 0 {
		return
	}

	selected := m.suggestions[m.acCursor]
	input := m.searchInput.Value()

	// Build the prefix character based on mode
	var prefix string
	switch m.acMode {
	case acSearchModeTag:
		prefix = "#"
	case acSearchModeSection:
		prefix = "/"
	case acSearchModeStatus:
		prefix = "$"
	case acSearchModeType:
		prefix = "^"
	case acSearchModeDueDate:
		prefix = "@"
	}

	// Replace from trigger position to end with completed value
	newInput := input[:m.acTriggerPos] + prefix + selected.value + " "
	m.searchInput.SetValue(newInput)
	m.searchInput.SetCursor(len(newInput))

	// Reset autocomplete state
	m.acMode = acSearchModeNone
	m.suggestions = nil
	m.acCursor = 0
}

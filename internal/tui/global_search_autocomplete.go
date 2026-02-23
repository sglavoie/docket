package tui

import (
	"fmt"
	"strings"
)

// globalSearchTriggers defines the trigger characters for global search autocomplete.
var globalSearchTriggers = []ACTrigger{
	{'#', acModeTag},
	{'/', acModeSection},
	{'$', acModeStatus},
	{'^', acModeType},
	{'@', acModeDate},
}

// detectAutocomplete checks if the cursor is after a filter trigger character
// and populates suggestions accordingly.
func (m *GlobalSearchModel) detectAutocomplete() {
	mode, pos, partial := DetectTrigger(m.searchInput.Value(), globalSearchTriggers)
	if mode == acModeNone {
		m.ac.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeTag:
		items = m.getTagSuggestions(partial)
	case acModeSection:
		items = m.getSectionSuggestions(partial)
	case acModeStatus:
		items = m.getStatusSuggestions(partial)
	case acModeType:
		items = m.getTypeSuggestions(partial)
	case acModeDate:
		items = m.getDueDateSuggestions(partial)
	}
	m.ac.UpdateSuggestions(mode, pos, items)
}

// getTagSuggestions returns matching tags for the partial query.
func (m *GlobalSearchModel) getTagSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, tag := range m.tagCache {
		if partial == "" || strings.Contains(strings.ToLower(tag), partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "#" + tag,
				Value:   tag,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getSectionSuggestions returns matching sections for the partial query.
func (m *GlobalSearchModel) getSectionSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, section := range m.sectionCache {
		if partial == "" || strings.Contains(strings.ToLower(section), partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "/" + section,
				Value:   section,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// getStatusSuggestions returns matching status options for the partial query.
func (m *GlobalSearchModel) getStatusSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, s := range statuses {
		if partial == "" || strings.Contains(s.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("$%s %s", s.name, s.icon),
				Value:   s.name,
			})
		}
	}
	return suggestions
}

// getTypeSuggestions returns matching type options for the partial query.
func (m *GlobalSearchModel) getTypeSuggestions(partial string) []ACSuggestion {
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

// getDueDateSuggestions returns due date filter options for the partial query.
func (m *GlobalSearchModel) getDueDateSuggestions(partial string) []ACSuggestion {
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

	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)

	for _, o := range options {
		if partial == "" || strings.Contains(o.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("@%s (%s)", o.name, o.desc),
				Value:   o.name,
			})
		}
	}
	return suggestions
}

// acceptAutocompleteSuggestion inserts the selected suggestion into the input.
func (m *GlobalSearchModel) acceptAutocompleteSuggestion() {
	if !m.ac.IsActive() {
		return
	}

	triggerChar := TriggerCharForMode(m.ac.Mode, globalSearchTriggers)
	newInput := m.ac.BuildAcceptedInput(m.searchInput.Value(), triggerChar)

	m.searchInput.SetValue(newInput)
	m.searchInput.SetCursor(len(newInput))

	m.ac.Clear()
}

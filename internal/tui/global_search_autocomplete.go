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
		// Fall back to keyword trigger detection
		mode, pos, partial = DetectKeywordTrigger(m.searchInput.Value())
	}
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
	case acModeHas:
		items = m.getHasSuggestions(partial)
	case acModeIs:
		items = m.getIsSuggestions(partial)
	case acModeNo:
		items = m.getNoSuggestions(partial)
	case acModeSearchProject:
		items = m.getProjectSlugSuggestions(partial)
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
		{"review", "◑"},
		{"standby", "⏸"},
		{"maybe", "❓"},
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

// keywordForMode returns the keyword string for keyword-based autocomplete modes.
// Returns "" for symbol-trigger modes.
func keywordForMode(mode int) string {
	switch mode {
	case acModeHas:
		return "has"
	case acModeIs:
		return "is"
	case acModeNo:
		return "no"
	case acModeSearchProject:
		return "p"
	}
	return ""
}

// acceptAutocompleteSuggestion inserts the selected suggestion into the input.
func (m *GlobalSearchModel) acceptAutocompleteSuggestion() {
	if !m.ac.IsActive() {
		return
	}

	var newInput string
	if kw := keywordForMode(m.ac.Mode); kw != "" {
		newInput = m.ac.BuildKeywordAcceptedInput(m.searchInput.Value(), kw)
	} else {
		triggerChar := TriggerCharForMode(m.ac.Mode, globalSearchTriggers)
		newInput = m.ac.BuildAcceptedInput(m.searchInput.Value(), triggerChar)
	}

	m.searchInput.SetValue(newInput)
	m.searchInput.SetCursor(len(newInput))

	m.ac.Clear()
}

// getHasSuggestions returns autocomplete suggestions for has: filters.
func (m *GlobalSearchModel) getHasSuggestions(partial string) []ACSuggestion {
	properties := []struct{ name, desc string }{
		{"description", "non-empty description"},
		{"comment", "at least one comment"},
		{"checklist", "has checklist items"},
		{"attachment", "has attachments"},
		{"due", "has a due date"},
		{"dependency", "depends on other tasks"},
		{"relation", "has related tasks"},
		{"recurrence", "has recurrence pattern"},
		{"url", "has external URL"},
		{"stage", "has workflow stage"},
		{"parent", "has a parent task"},
		{"children", "has child tasks"},
	}
	return filterKeywordSuggestions("has", properties, partial)
}

// getIsSuggestions returns autocomplete suggestions for is: filters.
func (m *GlobalSearchModel) getIsSuggestions(partial string) []ACSuggestion {
	states := []struct{ name, desc string }{
		{"note", "notes only"},
		{"pinned", "pinned tasks"},
		{"active", "active tasks"},
		{"dimmed", "dimmed tasks"},
		{"blocked", "status = blocked"},
		{"overdue", "past due + incomplete"},
		{"snoozed", "currently snoozed"},
		{"recurring", "has recurrence"},
		{"leaf", "no children"},
		{"root", "no parent"},
	}
	return filterKeywordSuggestions("is", states, partial)
}

// getNoSuggestions returns autocomplete suggestions for no: filters.
func (m *GlobalSearchModel) getNoSuggestions(partial string) []ACSuggestion {
	properties := []struct{ name, desc string }{
		{"description", "empty description"},
		{"comment", "no comments"},
		{"checklist", "no checklist"},
		{"attachment", "no attachments"},
		{"due", "no due date"},
		{"dependency", "no dependencies"},
		{"children", "no child tasks"},
		{"tag", "no tags"},
	}
	return filterKeywordSuggestions("no", properties, partial)
}

// getProjectSlugSuggestions returns autocomplete suggestions for p: filters.
func (m *GlobalSearchModel) getProjectSlugSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	for _, p := range m.projects {
		slug := p.Slug
		if partial == "" || strings.HasPrefix(strings.ToLower(slug), partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("p:%s", slug),
				Value:   slug,
			})
		}
		if len(suggestions) >= 8 {
			break
		}
	}
	return suggestions
}

// filterKeywordSuggestions filters a list of name/desc pairs by partial match and
// returns ACSuggestion items prefixed with the keyword.
func filterKeywordSuggestions(keyword string, items []struct{ name, desc string }, partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	for _, item := range items {
		if partial == "" || strings.HasPrefix(item.name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("%s:%s (%s)", keyword, item.name, item.desc),
				Value:   item.name,
			})
		}
	}
	return suggestions
}

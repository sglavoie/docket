package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
)

// searchTriggers defines the trigger characters for project search autocomplete.
var searchTriggers = []ACTrigger{
	{'#', acModeTag},
	{'/', acModeSection},
	{'$', acModeStatus},
	{'^', acModeType},
	{'@', acModeDate},
}

// DetectSearchAutocomplete checks the search input for trigger characters or
// keyword patterns and updates the autocomplete state with matching suggestions.
// tags and sections are the available values for the current project context.
func DetectSearchAutocomplete(ac *AutocompleteState, input string, tags, sections []string) {
	mode, pos, partial := DetectTrigger(input, searchTriggers)
	if mode == acModeNone {
		mode, pos, partial = DetectKeywordTrigger(input)
	}
	if mode == acModeNone {
		ac.Clear()
		return
	}

	var items []ACSuggestion
	switch mode {
	case acModeTag:
		items = searchTagSuggestions(tags, partial)
	case acModeSection:
		items = searchSectionSuggestions(sections, partial)
	case acModeStatus:
		items = searchStatusSuggestions(partial)
	case acModeType:
		items = searchTypeSuggestions(partial)
	case acModeDate:
		items = searchDueDateSuggestions(partial)
	case acModeHas:
		items = searchHasSuggestions(partial)
	case acModeIs:
		items = searchIsSuggestions(partial)
	case acModeNo:
		items = searchNoSuggestions(partial)
	}
	ac.UpdateSuggestions(mode, pos, items)
}

func searchTagSuggestions(tags []string, partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	for _, tag := range tags {
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

func searchSectionSuggestions(sections []string, partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	for _, section := range sections {
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

func searchStatusSuggestions(partial string) []ACSuggestion {
	statuses := []struct {
		name string
		icon string
	}{
		{"todo", icons.Get("kanban.todo")},
		{"in_progress", icons.Get("status.in_progress")},
		{"review", icons.Get("status.review")},
		{"standby", icons.Get("status.standby")},
		{"maybe", icons.Get("status.maybe")},
		{"done", icons.Get("status.done")},
		{"blocked", icons.Get("blocked_dot")},
		{"canceled", icons.Get("status.canceled")},
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

func searchTypeSuggestions(partial string) []ACSuggestion {
	types := []struct {
		name string
		icon string
	}{
		{"epic", icons.Get("type.epic")},
		{"story", icons.Get("type.story")},
		{"task", icons.Get("type.task")},
		{"bug", icons.Get("type.bug")},
		{"idea", icons.Get("idea")},
		{"note", icons.Get("note_type")},
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

func searchDueDateSuggestions(partial string) []ACSuggestion {
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

func searchHasSuggestions(partial string) []ACSuggestion {
	properties := []struct{ name, desc string }{
		{"description", "non-empty description"},
		{"comment", "at least one comment"},
		{"checklist", "has checklist items"},
		{"attachment", "has attachments"},
		{"issue", "has a GitHub issue link"},
		{"due", "has a due date"},
		{"dependency", "depends on other tasks"},
		{"relation", "has related tasks"},
		{"recurrence", "has recurrence pattern"},
		{"url", "has external URL"},
		{"parent", "has a parent task"},
		{"children", "has child tasks"},
	}
	return filterKeywordSuggestions("has", properties, partial)
}

func searchIsSuggestions(partial string) []ACSuggestion {
	states := []struct{ name, desc string }{
		{"pinned", "pinned tasks"},
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

func searchNoSuggestions(partial string) []ACSuggestion {
	properties := []struct{ name, desc string }{
		{"description", "empty description"},
		{"comment", "no comments"},
		{"checklist", "no checklist"},
		{"attachment", "no attachments"},
		{"issue", "no GitHub issue links"},
		{"due", "no due date"},
		{"dependency", "no dependencies"},
		{"children", "no child tasks"},
		{"tag", "no tags"},
	}
	return filterKeywordSuggestions("no", properties, partial)
}

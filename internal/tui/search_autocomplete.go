package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/textgrammar"
)

// searchTriggers defines the trigger characters for project search autocomplete.
var searchTriggers = []ACTrigger{
	{'#', acModeTag},
	{'$', acModeStatus},
	{'^', acModeType},
	{'@', acModeDate},
}

// DetectSearchAutocomplete checks the search input for trigger characters or
// keyword patterns and updates the autocomplete state with matching suggestions.
// tags are the available values for the current project context.
func DetectSearchAutocomplete(ac *AutocompleteState, input string, tags []string) {
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

func searchStatusSuggestions(partial string) []ACSuggestion {
	return canonicalStateSuggestions(partial, false)
}

func searchTypeSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldTaskType)
	for _, name := range def.Values {
		icon := icons.Get("note_type")
		if taskType, ok := models.ParseTaskType(name); ok {
			icon = captureTypeIcon(taskType)
		}
		if partial == "" || strings.Contains(name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("^%s %s", name, icon),
				Value:   name,
			})
		}
	}
	return suggestions
}

func searchDueDateSuggestions(partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partialLower := strings.ToLower(partial)
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldDue)
	for _, name := range def.Values {
		if partial == "" || strings.Contains(name, partialLower) {
			suggestions = append(suggestions, ACSuggestion{
				Display: "@" + name,
				Value:   name,
			})
		}
	}
	return suggestions
}

func searchHasSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldHas)
	return filterKeywordSuggestions("has", keywordValues(def.Values), partial)
}

func searchIsSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldIs)
	return filterKeywordSuggestions("is", keywordValues(def.Values), partial)
}

func searchNoSuggestions(partial string) []ACSuggestion {
	def, _ := textgrammar.DefinitionFor(textgrammar.SurfaceSearch, textgrammar.FieldNo)
	return filterKeywordSuggestions("no", keywordValues(def.Values), partial)
}

func keywordValues(values []string) []struct{ name, desc string } {
	result := make([]struct{ name, desc string }, 0, len(values))
	for _, value := range values {
		result = append(result, struct{ name, desc string }{name: value, desc: value})
	}
	return result
}

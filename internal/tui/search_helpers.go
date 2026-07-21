package tui

import (
	"fmt"
	"strings"
)

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
	default:
		return ""
	}
}

func filterKeywordSuggestions(keyword string, items []struct{ name, desc string }, partial string) []ACSuggestion {
	var suggestions []ACSuggestion
	partial = strings.ToLower(partial)
	for _, item := range items {
		if partial == "" || strings.HasPrefix(item.name, partial) {
			suggestions = append(suggestions, ACSuggestion{
				Display: fmt.Sprintf("%s:%s (%s)", keyword, item.name, item.desc),
				Value:   item.name,
			})
		}
	}
	return suggestions
}

func wrapValues(values []string, maxWidth int) string {
	var lines []string
	line := ""
	for _, value := range values {
		if line == "" {
			line = value
		} else if len(line)+1+len(value) > maxWidth {
			lines = append(lines, line)
			line = value
		} else {
			line += " " + value
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n  ")
}

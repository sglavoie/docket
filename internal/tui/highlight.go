package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// HighlightStyle is used to highlight matched characters in search results.
var HighlightStyle = lipgloss.NewStyle().Foreground(ColorHighlight).Bold(true)

// HighlightText renders text with certain character positions highlighted.
// Positions are rune indices (0-based) that should be highlighted.
// This is used to show fuzzy match positions in search results.
func HighlightText(text string, positions []int, baseStyle lipgloss.Style) string {
	if len(positions) == 0 {
		return baseStyle.Render(text)
	}

	// Create a set for quick lookup
	posSet := make(map[int]struct{})
	for _, p := range positions {
		posSet[p] = struct{}{}
	}

	var result strings.Builder
	runes := []rune(text)

	for i, r := range runes {
		char := string(r)
		if _, highlight := posSet[i]; highlight {
			result.WriteString(HighlightStyle.Render(char))
		} else {
			result.WriteString(baseStyle.Render(char))
		}
	}

	return result.String()
}

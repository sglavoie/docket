package textfit

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// HighlightText renders text with certain character positions highlighted.
// Positions are rune indices (0-based) that should be highlighted.
// This is used to show fuzzy match positions in search results.
func HighlightText(text string, positions []int, baseStyle lipgloss.Style) string {
	return HighlightTextWithStyle(text, positions, baseStyle, theme.HighlightStyle)
}

// HighlightTextWithStyle lets a selected row retain its selection palette while
// distinguishing matches with an underline instead of a second background.
func HighlightTextWithStyle(text string, positions []int, baseStyle, matchStyle lipgloss.Style) string {
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
			result.WriteString(matchStyle.Render(char))
		} else {
			result.WriteString(baseStyle.Render(char))
		}
	}

	return result.String()
}

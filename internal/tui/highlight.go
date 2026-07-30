package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// HighlightSubstring renders text with all case-insensitive occurrences of query highlighted.
func HighlightSubstring(text, query string, baseStyle lipgloss.Style) string {
	if query == "" {
		return baseStyle.Render(text)
	}

	runes := []rune(text)
	queryRunes := []rune(strings.ToLower(query))
	textLower := []rune(strings.ToLower(text))
	queryLen := len(queryRunes)

	var positions []int
	for i := 0; i <= len(textLower)-queryLen; i++ {
		match := true
		for j := range queryLen {
			if textLower[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			for j := range queryLen {
				positions = append(positions, i+j)
			}
		}
	}

	return HighlightText(string(runes), positions, baseStyle)
}

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

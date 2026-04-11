package tui

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
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

// HighlightStyledText highlights all case-insensitive occurrences of query
// within already-styled (ANSI) text. It strips ANSI to find match positions
// in the plain text, then walks the original styled string byte-by-byte,
// wrapping matched visible characters with HighlightStyle.
func HighlightStyledText(styled, query string) string {
	if query == "" || styled == "" {
		return styled
	}

	plain := ansi.Strip(styled)
	plainLower := strings.ToLower(plain)
	queryLower := strings.ToLower(query)

	// Find all match positions in the plain text (rune indices).
	matchSet := make(map[int]struct{})
	plainRunes := []rune(plainLower)
	queryRunes := []rune(queryLower)
	qLen := len(queryRunes)
	for i := 0; i <= len(plainRunes)-qLen; i++ {
		match := true
		for j := range qLen {
			if plainRunes[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			for j := range qLen {
				matchSet[i+j] = struct{}{}
			}
		}
	}

	if len(matchSet) == 0 {
		return styled
	}

	// Walk the styled string, tracking which plain-text rune index
	// each visible character corresponds to.
	var result strings.Builder
	result.Grow(len(styled) + len(matchSet)*20)

	bytes := []byte(styled)
	plainIdx := 0
	i := 0
	for i < len(bytes) {
		// Check for ANSI escape sequence (CSI: ESC [ ... final byte).
		if bytes[i] == 0x1b && i+1 < len(bytes) && bytes[i+1] == '[' {
			// Copy the entire escape sequence as-is.
			start := i
			i += 2
			for i < len(bytes) && (bytes[i] < 0x40 || bytes[i] > 0x7e || bytes[i] == '[') {
				i++
			}
			if i < len(bytes) {
				i++ // include the final byte
			}
			result.Write(bytes[start:i])
			continue
		}

		// Check for OSC sequence (ESC ] ... ST).
		if bytes[i] == 0x1b && i+1 < len(bytes) && bytes[i+1] == ']' {
			start := i
			i += 2
			for i < len(bytes) {
				if bytes[i] == 0x1b && i+1 < len(bytes) && bytes[i+1] == '\\' {
					i += 2
					break
				}
				if bytes[i] == 0x07 {
					i++
					break
				}
				i++
			}
			result.Write(bytes[start:i])
			continue
		}

		// Visible character — decode one rune.
		r, size := utf8.DecodeRune(bytes[i:])
		if _, highlight := matchSet[plainIdx]; highlight {
			result.WriteString(HighlightStyle.Render(string(r)))
		} else {
			result.Write(bytes[i : i+size])
		}
		plainIdx++
		i += size
	}

	return result.String()
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

// highlightTruncated highlights matched character positions in a
// (possibly truncated) title.  Positions beyond the title length are
// silently dropped.  When no positions remain, the plain title is
// returned without any style wrapper.
func highlightTruncated(title string, positions []int) string {
	if len(positions) == 0 {
		return title
	}
	titleRunes := []rune(title)
	var valid []int
	for _, p := range positions {
		if p < len(titleRunes) {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return title
	}
	return HighlightText(title, valid, NormalStyle)
}

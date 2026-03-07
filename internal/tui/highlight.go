package tui

import (
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// HighlightStyle is used to highlight matched characters in search results.
var HighlightStyle = lipgloss.NewStyle().Background(ColorHighlight).Foreground(lipgloss.Color("0")).Bold(true)

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
		for j := 0; j < queryLen; j++ {
			if textLower[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			for j := 0; j < queryLen; j++ {
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
		for j := 0; j < qLen; j++ {
			if plainRunes[i+j] != queryRunes[j] {
				match = false
				break
			}
		}
		if match {
			for j := 0; j < qLen; j++ {
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

package tui

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// wrapStyled wraps a string that may contain ANSI escape codes, accounting for
// visual width (not byte length). Use this when input has been styled with
// lipgloss; use wrapText for plain text.
func wrapStyled(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}

// wrapText wraps text to the specified width.
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	for len(text) > 0 {
		if len(text) <= width {
			lines = append(lines, text)
			break
		}

		breakAt := width
		for i := width; i > 0; i-- {
			if text[i] == ' ' {
				breakAt = i
				break
			}
		}

		if breakAt == width && len(text) > width {
			for i := width - 1; i > width/2; i-- {
				if text[i] == ' ' {
					breakAt = i
					break
				}
			}
		}

		lines = append(lines, text[:breakAt])
		text = strings.TrimLeft(text[breakAt:], " ")
	}

	return lines
}

package textfit

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// WrapStyled wraps a string that may contain ANSI escape codes, accounting for
// visual width (not byte length). Use this when input has been styled with
// lipgloss; use WrapText for plain text.
func WrapStyled(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	return strings.Split(ansi.Wrap(s, width, ""), "\n")
}

func hardWrapStyled(s string, width int) []string {
	if width <= 0 || s == "" {
		return []string{s}
	}
	return strings.Split(ansi.Hardwrap(s, width, true), "\n")
}

// WrapText wraps text to the specified width.
func WrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return nil
	}
	return strings.Split(ansi.Wrap(text, width, ""), "\n")
}

package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

func truncateToVisualWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width < 2 {
		return ansi.Truncate(text, width, "")
	}
	return ansi.Truncate(text, width-1, "") + "…"
}

func fitStyledLineToWidth(line string, width int) string {
	if width <= 0 || lipgloss.Width(line) <= width {
		return line
	}
	return ansi.Truncate(line, width, "")
}

func leftPadToWidth(text string, width int) string {
	visualWidth := lipgloss.Width(text)
	if visualWidth >= width {
		return text
	}
	return strings.Repeat(" ", width-visualWidth) + text
}

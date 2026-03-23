package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

func leftPadToWidth(text string, width int) string {
	visualWidth := lipgloss.Width(text)
	if visualWidth >= width {
		return text
	}
	return strings.Repeat(" ", width-visualWidth) + text
}

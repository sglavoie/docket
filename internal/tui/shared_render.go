package tui

import (
	"github.com/charmbracelet/x/ansi"
	"github.com/sglavoie/tt/internal/tui/theme"
)

// ZebraLine applies a subtle background tint to odd-indexed rows when zebra
// striping is enabled.
func ZebraLine(line string, width int, rowIndex int) string {
	if !theme.ZebraEnabled || rowIndex%2 == 0 {
		return line
	}
	return theme.ZebraRowStyle.Width(width).Render(line)
}

// SelectedLine strips all ANSI styling and re-renders with the unified
// selection style (colored background, high-contrast foreground, bold).
// Call after full line assembly so the entire row gets consistent styling.
func SelectedLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return theme.SelectedStyle.Width(width).Render(stripped)
}

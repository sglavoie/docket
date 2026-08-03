package tui

import "github.com/charmbracelet/x/ansi"

// ZebraLine applies a subtle background tint to odd-indexed rows when zebra
// striping is enabled.
func ZebraLine(line string, width int, rowIndex int) string {
	if !zebraEnabled || rowIndex%2 == 0 {
		return line
	}
	return ZebraRowStyle.Width(width).Render(line)
}

// SelectedLine strips all ANSI styling and re-renders with the unified
// selection style (colored background, high-contrast foreground, bold).
// Call after full line assembly so the entire row gets consistent styling.
func SelectedLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return SelectedStyle.Width(width).Render(stripped)
}

// SelectedFlashLine is like SelectedLine but uses the flash background.
func SelectedFlashLine(line string, width int) string {
	stripped := ansi.Strip(line)
	return SelectedStyle.Background(ColorFlashBg).Width(width).Render(stripped)
}

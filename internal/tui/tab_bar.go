package tui

import "github.com/charmbracelet/lipgloss"

// RenderTabBar renders a horizontal tab bar with the given tab labels, highlighting
// the active tab. The availableWidth controls the filler line width. The padding
// parameter sets the horizontal padding for each tab (typically 1 or 2).
func RenderTabBar(tabs []string, activeIdx int, availableWidth int, padding int) string {
	plainStyle := lipgloss.NewStyle().Padding(0, padding)
	totalWidth := 0
	for _, name := range tabs {
		totalWidth += lipgloss.Width(plainStyle.Render(name))
	}
	fitsSingleLine := totalWidth <= availableWidth

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, padding)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, padding)

	if fitsSingleLine {
		activeStyle = activeStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(ColorPrimary)
		inactiveStyle = inactiveStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder)
	}

	var rendered []string
	for i, name := range tabs {
		if i == activeIdx {
			rendered = append(rendered, activeStyle.Render(name))
		} else {
			rendered = append(rendered, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, rendered...)

	if fitsSingleLine {
		tabWidth := lipgloss.Width(row)
		remaining := availableWidth - tabWidth
		if remaining > 0 {
			filler := lipgloss.NewStyle().
				BorderBottom(true).
				BorderStyle(lipgloss.Border{Bottom: "─"}).
				BorderForeground(ColorBorder).
				Width(remaining).
				Render("")
			row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
		}
	}

	return row
}

package tui

import "github.com/charmbracelet/lipgloss"

// RenderTabBar renders a horizontal tab bar with the given tab labels, highlighting
// the active tab. The availableWidth controls the filler line width. The padding
// parameter sets the horizontal padding for each tab (typically 1 or 2).
func RenderTabBar(tabs []string, activeIdx int, availableWidth int, padding int) string {
	return RenderColoredTabBar(tabs, activeIdx, availableWidth, padding, nil)
}

// RenderColoredTabBar renders a tab bar where inactive tabs can have custom foreground
// colors via the tabColors map (tab name -> color name from SectionColorPalette).
// Active tabs always use the primary color. Tabs not in the map use the default muted color.
func RenderColoredTabBar(tabs []string, activeIdx int, availableWidth int, padding int, tabColors map[string]string) string {
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
			// Use custom color for active tab if available
			if tabColors != nil {
				if colorName, ok := tabColors[name]; ok && colorName != "" && colorName != "gray" {
					colorVal := GetSectionColorValue(colorName)
					if colorVal != "" {
						coloredActive := lipgloss.NewStyle().
							Bold(true).
							Foreground(colorVal).
							Padding(0, padding)
						if fitsSingleLine {
							coloredActive = coloredActive.
								BorderBottom(true).
								BorderStyle(lipgloss.ThickBorder()).
								BorderForeground(colorVal)
						}
						rendered = append(rendered, coloredActive.Render(name))
						continue
					}
				}
			}
			rendered = append(rendered, activeStyle.Render(name))
		} else {
			// Check for custom color on inactive tabs
			if tabColors != nil {
				if colorName, ok := tabColors[name]; ok && colorName != "" && colorName != "gray" {
					colorVal := GetSectionColorValue(colorName)
					if colorVal != "" {
						coloredStyle := lipgloss.NewStyle().
							Foreground(colorVal).
							Padding(0, padding)
						if fitsSingleLine {
							coloredStyle = coloredStyle.
								BorderBottom(true).
								BorderStyle(lipgloss.Border{Bottom: "─"}).
								BorderForeground(ColorBorder)
						}
						rendered = append(rendered, coloredStyle.Render(name))
						continue
					}
				}
			}
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

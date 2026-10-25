package chrome

import (
	"image/color"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/theme"
)

// ActiveTabStyle returns the standard active tab style: bold, primary-colored,
// with optional bottom border. Pass an optional accent color to override the
// primary color for the foreground and border.
func ActiveTabStyle(padding int, withBorder bool, accent ...color.Color) lipgloss.Style {
	fg := theme.ColorPrimary
	if len(accent) > 0 && accent[0] != nil {
		fg = accent[0]
	}
	s := lipgloss.NewStyle().
		Bold(true).
		Foreground(fg).
		Padding(0, padding)
	if withBorder {
		s = s.BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(fg)
	}
	if theme.ThemeBgColor != "" {
		s = s.Background(lipgloss.Color(theme.ThemeBgColor))
	}
	return s
}

// InactiveTabStyle returns the standard inactive tab style: muted foreground,
// with optional bottom border. Used by tab bar renderers to avoid style duplication.
func InactiveTabStyle(padding int, withBorder bool) lipgloss.Style {
	s := lipgloss.NewStyle().
		Foreground(theme.ColorMuted).
		Padding(0, padding)
	if withBorder {
		s = s.BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(theme.ColorBorder)
	}
	if theme.ThemeBgColor != "" {
		s = s.Background(lipgloss.Color(theme.ThemeBgColor))
	}
	return s
}

// RenderTabBar renders a horizontal tab bar with the given tab labels, highlighting
// the active tab. The availableWidth controls the filler line width. The padding
// parameter sets the horizontal padding for each tab (typically 1 or 2).
// Pass an optional accent color to tint the active tab.
func RenderTabBar(tabs []string, activeIdx int, availableWidth int, padding int, accent ...color.Color) string {
	return RenderColoredTabBar(tabs, activeIdx, availableWidth, padding, nil, accent...)
}

// RenderColoredTabBar renders a tab bar where inactive tabs can have custom foreground
// colors via the tabColors map (tab name -> color name from SectionColorPalette).
// Active tabs always use the primary color. Tabs not in the map use the default muted color.
// Pass an optional accent color to tint the active tab.
func RenderColoredTabBar(tabs []string, activeIdx int, availableWidth int, padding int, tabColors map[string]string, accent ...color.Color) string {
	if len(tabs) == 0 {
		return ""
	}
	if activeIdx < 0 || activeIdx >= len(tabs) {
		activeIdx = 0
	}
	plainStyle := lipgloss.NewStyle().Padding(0, padding)
	totalWidth := 0
	for _, name := range tabs {
		totalWidth += lipgloss.Width(plainStyle.Render(name))
	}
	fitsSingleLine := totalWidth <= availableWidth

	active := ActiveTabStyle(padding, fitsSingleLine, accent...)
	inactive := InactiveTabStyle(padding, fitsSingleLine)

	var rendered []string
	for i, name := range tabs {
		if i == activeIdx {
			// Use custom color for active tab if available
			if tabColors != nil {
				if colorName, ok := tabColors[name]; ok && colorName != "" && colorName != "gray" {
					colorVal := theme.GetSectionColorValue(colorName)
					if colorVal != nil {
						colored := ActiveTabStyle(padding, fitsSingleLine, accent...).
							Foreground(colorVal).
							BorderForeground(colorVal)
						rendered = append(rendered, colored.Render(name))
						continue
					}
				}
			}
			rendered = append(rendered, active.Render(name))
		} else {
			// Check for custom color on inactive tabs
			if tabColors != nil {
				if colorName, ok := tabColors[name]; ok && colorName != "" && colorName != "gray" {
					colorVal := theme.GetSectionColorValue(colorName)
					if colorVal != nil {
						colored := InactiveTabStyle(padding, fitsSingleLine).
							Foreground(colorVal)
						rendered = append(rendered, colored.Render(name))
						continue
					}
				}
			}
			rendered = append(rendered, inactive.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, rendered...)

	if fitsSingleLine {
		remaining := availableWidth - lipgloss.Width(row)
		if filler := renderTabFiller(remaining); filler != "" {
			row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
		}
	}

	return row
}

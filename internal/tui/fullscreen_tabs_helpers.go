package tui

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderWrappingTabBar renders a tab bar that wraps to multiple rows when tabs
// overflow the available width. Each row is filled with a border filler on the right.
// This differs from RenderTabBar which renders a single-line tab bar.
// Pass an optional accent color to tint the active tab.
func renderWrappingTabBar(tabs []string, activeIdx, availableWidth int, accent ...color.Color) string {
	return renderWrappingTabBarColored(tabs, activeIdx, nil, availableWidth, accent...)
}

// renderWrappingTabBarColored is like renderWrappingTabBar but accepts per-tab
// foreground colors for inactive tabs (nil entry = default muted color).
func renderWrappingTabBarColored(tabs []string, activeIdx int, tabColors []color.Color, availableWidth int, accent ...color.Color) string {
	if len(tabs) == 0 {
		return ""
	}
	if activeIdx < 0 || activeIdx >= len(tabs) {
		activeIdx = 0
	}
	plainStyle := lipgloss.NewStyle().Padding(0, 1)
	active := activeTabStyle(1, true, accent...)
	inactive := inactiveTabStyle(1, true)

	// Build rows of tabs that fit within available width
	var rows []string
	var currentRow []string
	currentWidth := 0

	for i, name := range tabs {
		tabWidth := lipgloss.Width(plainStyle.Render(name))
		if currentWidth+tabWidth > availableWidth && len(currentRow) > 0 {
			rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
			currentRow = nil
			currentWidth = 0
		}
		if i == activeIdx {
			currentRow = append(currentRow, active.Render(name))
		} else if i < len(tabColors) && tabColors[i] != nil {
			colored := inactiveTabStyle(1, true).Foreground(tabColors[i])
			currentRow = append(currentRow, colored.Render(name))
		} else {
			currentRow = append(currentRow, inactive.Render(name))
		}
		currentWidth += tabWidth
	}
	if len(currentRow) > 0 {
		rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
	}

	return strings.Join(rows, "\n")
}

// renderSubTabBarWithFocus renders a sub-tab bar row with a ▐ focus indicator.
// tabs is the list of tab names, activeIdx is the active tab, focused indicates
// whether this row has focus (shows ▐) or not (shows space), and width is the
// available width for the tab bar.
func renderSubTabBarWithFocus(tabs []string, activeIdx int, focused bool, width int, accent ...color.Color) string {
	bar := RenderTabBar(tabs, activeIdx, width-2, 1, accent...)

	if focused {
		bar = FocusIndicator + bar
	} else {
		bar = " " + bar
	}
	return bar
}

// renderTabFiller returns a bottom-border filler element that extends a tab row
// to the given width. Returns "" if width <= 0.
func renderTabFiller(width int) string {
	if width <= 0 {
		return ""
	}
	s := lipgloss.NewStyle().
		BorderBottom(true).
		BorderStyle(lipgloss.Border{Bottom: "─"}).
		BorderForeground(ColorBorder).
		Width(width)
	if themeBgColor != "" {
		s = s.Background(lipgloss.Color(themeBgColor))
	}
	return s.Render("")
}

// finishWrappingTabRow joins a row of rendered tabs and fills remaining width with a border.
func finishWrappingTabRow(tabs []string, availableWidth int) string {
	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)
	remaining := availableWidth - lipgloss.Width(row)
	if filler := renderTabFiller(remaining); filler != "" {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
	}
	return row
}

// renderWrappingHelpLine renders a help line that wraps to multiple lines when width is exceeded.
// Items should be separated by " · " in the input. The function splits them across lines as needed.
func renderWrappingHelpLine(helpText string, width int) string {
	if width <= 0 {
		width = 80
	}

	separator := " · "
	items := strings.Split(helpText, separator)
	if len(items) == 0 {
		return ""
	}

	var lines []string
	var currentLine strings.Builder
	currentWidth := 0

	for i, item := range items {
		itemWidth := lipgloss.Width(item)
		sepWidth := 0
		if currentLine.Len() > 0 {
			sepWidth = lipgloss.Width(separator)
		}

		// Check if adding this item would exceed width
		if currentLine.Len() > 0 && currentWidth+sepWidth+itemWidth > width {
			// Start a new line
			lines = append(lines, currentLine.String())
			currentLine.Reset()
			currentWidth = 0
		}

		// Add separator if not first item on this line
		if currentLine.Len() > 0 {
			currentLine.WriteString(separator)
			currentWidth += sepWidth
		}

		currentLine.WriteString(item)
		currentWidth += itemWidth

		// Handle last item
		if i == len(items)-1 && currentLine.Len() > 0 {
			lines = append(lines, currentLine.String())
		}
	}

	return strings.Join(lines, "\n")
}

// renderTabBarWithOptionalSubTabs renders a tab bar that switches between a
// wrapping tab bar and a sub-tab-bar-with-focus layout depending on whether
// sub-tabs are present.
func renderTabBarWithOptionalSubTabs(tabNames []string, activeTab int, subTabNames []string, subTabRowFocused bool, width int, accent ...color.Color) string {
	if len(subTabNames) > 1 {
		return renderSubTabBarWithFocus(tabNames, activeTab, !subTabRowFocused, width, accent...)
	}
	return renderWrappingTabBar(tabNames, activeTab, width, accent...)
}

// renderDynamicSubTabBar renders the dynamic sub-tab row when there are
// multiple sub-tabs (prepends "All"), or returns "" if none.
func renderDynamicSubTabBar(subTabNames []string, subTab int, subTabRowFocused bool, width int, accent ...color.Color) string {
	if len(subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, subTabNames...)

	// Use wrapping tab bar with width reduced for focus indicator column
	bar := renderWrappingTabBar(tabs, subTab, width-2, accent...)

	lines := strings.Split(bar, "\n")
	if subTabRowFocused {
		lines[0] = FocusIndicator + lines[0]
	} else {
		lines[0] = " " + lines[0]
	}
	return strings.Join(lines, "\n")
}

package chrome

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/theme"
)

// TabSpacing chooses how a wrapping tab bar spaces its tabs.
type TabSpacing int

const (
	// TabSpacingDefault pads each tab by one column on both sides.
	TabSpacingDefault TabSpacing = iota
	// TabSpacingCompact pads no tab and separates neighbors by one border
	// column, so the active tab's underline covers its label alone. It is for a
	// bar whose labels would otherwise wrap at the width it is read at.
	TabSpacingCompact
)

// RenderWrappingTabBar renders a tab bar that wraps to multiple rows when tabs
// overflow the available width. Each row is filled with a border filler on the right.
// This differs from RenderTabBar which renders a single-line tab bar.
// Pass an optional accent color to tint the active tab.
func RenderWrappingTabBar(tabs []string, activeIdx, availableWidth int, spacing TabSpacing, accent ...color.Color) string {
	return renderWrappingTabBarColored(tabs, activeIdx, nil, nil, availableWidth, spacing, accent...)
}

// renderWrappingTabBarColored is like RenderWrappingTabBar but accepts per-tab
// foreground colors for inactive tabs (nil entry = default muted color) and an
// optional emptyTabs slice. When emptyTabs[i] is true and i is not the active tab,
// the tab is rendered faint to indicate it has no items.
func renderWrappingTabBarColored(tabs []string, activeIdx int, tabColors []color.Color, emptyTabs []bool, availableWidth int, spacing TabSpacing, accent ...color.Color) string {
	if len(tabs) == 0 {
		return ""
	}
	if activeIdx < 0 || activeIdx >= len(tabs) {
		activeIdx = 0
	}
	padding, gap := 1, 0
	if spacing == TabSpacingCompact {
		padding, gap = 0, 1
	}
	plainStyle := lipgloss.NewStyle().Padding(0, padding)
	active := ActiveTabStyle(padding, true, accent...)
	inactive := InactiveTabStyle(padding, true)

	// Build rows of tabs that fit within available width
	var rows []string
	var currentRow []string
	currentWidth := 0

	for i, name := range tabs {
		tabWidth := lipgloss.Width(plainStyle.Render(name))
		if len(currentRow) > 0 && currentWidth+gap+tabWidth > availableWidth {
			rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
			currentRow = nil
			currentWidth = 0
		}
		if len(currentRow) > 0 && gap > 0 {
			currentRow = append(currentRow, renderTabFiller(gap))
			currentWidth += gap
		}
		if i == activeIdx {
			currentRow = append(currentRow, active.Render(name))
		} else {
			s := inactive
			if i < len(tabColors) && tabColors[i] != nil {
				s = InactiveTabStyle(padding, true).Foreground(tabColors[i])
			}
			if i < len(emptyTabs) && emptyTabs[i] {
				s = s.Faint(true)
			}
			currentRow = append(currentRow, s.Render(name))
		}
		currentWidth += tabWidth
	}
	if len(currentRow) > 0 {
		rows = append(rows, finishWrappingTabRow(currentRow, availableWidth))
	}

	return strings.Join(rows, "\n")
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
		BorderForeground(theme.ColorBorder).
		Width(width)
	if theme.ThemeBgColor != "" {
		s = s.Background(lipgloss.Color(theme.ThemeBgColor))
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

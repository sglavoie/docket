package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// renderPinnedTasksBar renders the compact, non-interactive pinned summary.
// Navigation and search live in Finder; the retired pinned popup is gone.
func (m *AppModel) renderPinnedTasksBar(width int) string {
	pinned := m.collectPinnedTasks()
	if len(pinned) == 0 {
		return ""
	}

	separatorStr := MutedStyle.Render(" · ")
	separatorWidth := lipgloss.Width(separatorStr)
	maxDisplay := m.runtime.Settings().GetMaxPinnedBarItems()
	if maxDisplay == 0 {
		return ""
	}
	overflow := 0
	if len(pinned) > maxDisplay {
		overflow = len(pinned) - maxDisplay
		pinned = pinned[:maxDisplay]
	}

	numEntries := len(pinned)
	totalSeparatorWidth := separatorWidth * (numEntries - 1)
	if numEntries == 1 {
		totalSeparatorWidth = 0
	}
	perEntryWidth := (width - totalSeparatorWidth) / numEntries

	entries := make([]string, 0, len(pinned))
	for _, item := range pinned {
		typeBadge := GetTypeBadge(string(item.Task.Type), item.Task.Type.Icon())
		titleWidth := max(perEntryWidth-2, 3)
		title := TaskTitleStyleFor(item.Task, TaskTitleStyleOptions{}).
			Render(truncateToVisualWidth(item.Title, titleWidth))
		if renderedWidth := lipgloss.Width(title); renderedWidth < titleWidth {
			title += strings.Repeat(" ", titleWidth-renderedWidth)
		}
		entries = append(entries, typeBadge+" "+title)
	}

	result := strings.Join(entries, separatorStr)
	if overflow > 0 {
		result += MutedStyle.Render(fmt.Sprintf(" +%d", overflow))
	}
	return result
}

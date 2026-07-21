package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

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

	var entries []string
	for _, p := range pinned {
		typeBadge := GetTypeBadge(string(p.Task.Type), p.Task.Type.Icon())

		prefixWidth := 1 + 1
		titleWidth := max(perEntryWidth-prefixWidth, 3)

		title := TaskTitleStyleFor(p.Task, TaskTitleStyleOptions{}).
			Render(truncateToVisualWidth(p.Title, titleWidth))
		if w := lipgloss.Width(title); w < titleWidth {
			title += strings.Repeat(" ", titleWidth-w)
		}

		entries = append(entries, typeBadge+" "+title)
	}

	result := strings.Join(entries, separatorStr)

	if overflow > 0 {
		more := MutedStyle.Render(fmt.Sprintf(" +%d", overflow))
		result += more
	}

	return result
}

// renderPinnedPopup renders the pinned tasks overlay popup.
func (m *AppModel) renderPinnedPopup() string {
	if !m.showPinnedPopup || len(m.pinnedPopupItems) == 0 {
		return ""
	}

	borderStyle := InfoStatusStyle
	titleStyle := TitleStyle
	title := titleStyle.Render("Pinned Tasks")
	filterStyle := InfoStatusStyle

	popupWidth := max(m.width-4, 30)

	badgeStyle := InfoStatusStyle
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle

	filtered := m.pinnedPopupFiltered
	innerWidth := popupWidth - 4

	maxProjLen := 0
	for _, realIdx := range filtered {
		entry := m.pinnedPopupItems[realIdx]
		if width := lipgloss.Width(entry.ProjectName); width > maxProjLen {
			maxProjLen = width
		}
	}

	projColWidth := maxProjLen
	fixedWidth := 1 + 1 + 1 + 1 + 1 + 1 + 3 + 1 + 2 + projColWidth
	titleWidth := max(innerWidth-fixedWidth, 10)

	var contentLines []string

	if m.pinnedPopupFiltering {
		filterLine := filterStyle.Render("> "+m.pinnedPopupFilter) + MutedStyle.Render("_")
		contentLines = append(contentLines, filterLine)
	} else if m.pinnedPopupFilter != "" {
		filterLine := filterStyle.Render("> " + m.pinnedPopupFilter)
		contentLines = append(contentLines, filterLine)
	}

	if len(filtered) == 0 {
		noMatch := MutedStyle.Render("No matching tasks")
		contentLines = append(contentLines, noMatch)
	} else {
		for i, realIdx := range filtered {
			entry := m.pinnedPopupItems[realIdx]

			badge := badgeStyle.Render("›")
			if m.pinnedPopupFiltering {
				badge = dimBadgeStyle.Render("·")
			}

			statusIcon := GetStateStyle(entry.Task.State()).Render(entry.Task.State().Icon())
			typeBadge := GetTypeBadge(string(entry.Task.Type), entry.Task.Type.Icon())
			priorityBadge := renderPriorityBadge(entry.Task.Priority)
			projectName := entry.ProjectName

			titleStyle := TaskTitleStyleFor(entry.Task, TaskTitleStyleOptions{})
			taskTitle := titleStyle.Render(entry.Title)
			if i < len(m.pinnedPopupTitlePos) && len(m.pinnedPopupTitlePos[i]) > 0 {
				taskTitle = truncateAndPad(HighlightText(entry.Title, m.pinnedPopupTitlePos[i], titleStyle), titleWidth)
			} else {
				taskTitle = truncateAndPad(taskTitle, titleWidth)
			}

			paddedProject := leftPadToWidth(projectName, projColWidth)

			line := badge + " " + statusIcon + " " + typeBadge + " " + priorityBadge + " " + taskTitle + "  " + projectStyle.Render(paddedProject)

			if i == m.pinnedPopupCursor {
				lineWidth := lipgloss.Width(line)
				if lineWidth < innerWidth {
					line += strings.Repeat(" ", innerWidth-lineWidth)
				}
				line = SelectedLine(truncateIntentional(line, innerWidth, ""), innerWidth)
			}

			contentLines = append(contentLines, line)
		}
	}

	content := strings.Join(contentLines, "\n")

	var footer string
	if m.pinnedPopupFiltering {
		footer = MutedStyle.Render("↑↓ nav  enter open  shift+enter Peek  esc clear")
	} else if m.pinnedPopupFilter != "" {
		footer = MutedStyle.Render("j/k nav  enter open  shift+enter Peek  p unpin  J/K reorder  / filter  esc clear")
	} else {
		footer = MutedStyle.Render("j/k nav  / filter  enter open  shift+enter Peek  p unpin  J/K reorder  esc close")
	}

	return renderTitledBox(content, popupWidth, title, footer, borderStyle)
}

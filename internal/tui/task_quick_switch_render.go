package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// renderQuickSwitchPopup renders the quick-switch overlay popup.
func (m *AppModel) renderQuickSwitchPopup() string {
	if !m.showQuickSwitch || len(m.recentTaskVisits) == 0 {
		return ""
	}

	borderStyle := InfoStatusStyle
	titleText := "Recent Tasks"
	if m.quickSwitchShowCompleted {
		titleText += " (Completed)"
	}
	title := TitleStyle.Render(titleText)
	popupWidth := max(m.width-4, 30)

	badgeStyle := InfoStatusStyle
	dimBadgeStyle := MutedStyle
	projectStyle := MutedStyle
	pinStyle := WarningIconStyle
	tagStyle := MutedStyle
	dividerStyle := MutedStyle
	filterStyle := InfoStatusStyle

	filtered := m.quickSwitchFiltered
	innerWidth := popupWidth - 4
	var contentLines []string

	if m.quickSwitchFiltering {
		contentLines = append(contentLines, filterStyle.Render("> "+m.quickSwitchFilter)+MutedStyle.Render("_"))
	} else if m.quickSwitchFilter != "" {
		contentLines = append(contentLines, filterStyle.Render("> "+m.quickSwitchFilter))
	}

	if len(filtered) == 0 {
		contentLines = append(contentLines, MutedStyle.Render("No matching tasks"))
	} else {
		maxProjLen := 0
		for _, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]
			if width := lipgloss.Width(entry.Project.Name); width > maxProjLen {
				maxProjLen = width
			}
		}

		projColWidth := maxProjLen
		fixedWidth := 1 + 1 + 1 + 1 + 3 + 1 + 2 + projColWidth
		titleWidth := max(innerWidth-fixedWidth, 10)

		hasPinned := false
		firstUnpinnedFilteredIdx := -1
		for i, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]
			if entry.Pinned {
				hasPinned = true
			} else if firstUnpinnedFilteredIdx == -1 {
				firstUnpinnedFilteredIdx = i
			}
		}

		for i, realIdx := range filtered {
			entry := m.recentTaskVisits[realIdx]
			if hasPinned && i == firstUnpinnedFilteredIdx {
				contentLines = append(contentLines, dividerStyle.Render(strings.Repeat("─", innerWidth)))
			}

			badge := dimBadgeStyle.Render("·")
			if !m.quickSwitchFiltering && i < 9 {
				badge = badgeStyle.Render(fmt.Sprintf("%d", i+1))
			}

			statusIcon := GetStatusStyle(string(entry.Task.Status)).Render(entry.Task.Status.Icon())
			priorityBadge := renderPriorityBadge(entry.Task.Priority)
			projectName := entry.Project.Name

			pinIndicator := "  "
			if entry.Pinned {
				pinIndicator = pinStyle.Render("* ")
			}

			var metaParts []string
			if tags := entry.Task.Tags; len(tags) > 0 {
				displayTags := tags
				if len(displayTags) > 2 {
					displayTags = displayTags[:2]
				}
				for _, tag := range displayTags {
					metaParts = append(metaParts, tagStyle.Render("#"+tag))
				}
			}
			if dueStr := FormatRelativeDue(entry.Task.DueDate, true); dueStr != "" {
				metaParts = append(metaParts, "@"+dueStr)
			}

			metaSuffix := ""
			if len(metaParts) > 0 {
				metaSuffix = " " + strings.Join(metaParts, " ")
			}

			metaVisualWidth := lipgloss.Width(metaSuffix)
			pinVisualWidth := lipgloss.Width(pinIndicator)
			adjustedTitleWidth := max(titleWidth-metaVisualWidth-pinVisualWidth, 10)

			taskTitle := entry.Task.Title
			if i < len(m.quickSwitchTitlePositions) && len(m.quickSwitchTitlePositions[i]) > 0 {
				taskTitle = truncateAndPad(HighlightText(taskTitle, m.quickSwitchTitlePositions[i], NormalStyle), adjustedTitleWidth)
			} else {
				taskTitle = truncateAndPad(taskTitle, adjustedTitleWidth)
			}

			paddedProject := leftPadToWidth(projectName, projColWidth)
			line := badge + " " + statusIcon + " " + priorityBadge + " " + pinIndicator + taskTitle + metaSuffix + "  " + projectStyle.Render(paddedProject)

			if i == m.quickSwitchCursor {
				lineWidth := lipgloss.Width(line)
				if lineWidth < innerWidth {
					line += strings.Repeat(" ", innerWidth-lineWidth)
				}
				line = SelectedLine(ansi.Truncate(line, innerWidth, ""), innerWidth)
			}

			contentLines = append(contentLines, line)
		}
	}

	content := strings.Join(contentLines, "\n")
	rendered := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(0, 1).
		Width(popupWidth).
		Render(content)

	lines := strings.Split(rendered, "\n")
	if len(lines) >= 2 {
		actualWidth := lipgloss.Width(lines[0])
		lines[0] = renderTitledTopBorder(title, actualWidth, borderStyle)

		footer := MutedStyle.Render("j/k nav  / filter  1-9 jump  p pin  c toggle  ,status  !pri  x remove  esc close")
		if m.quickSwitchFiltering {
			footer = MutedStyle.Render("↑↓ nav  enter confirm  esc clear")
		} else if m.quickSwitchFilter != "" {
			footer = MutedStyle.Render("j/k nav  enter select  ,status  !pri  / filter  esc clear")
		}
		lines[len(lines)-1] = renderTitledBottomBorder(footer, actualWidth, borderStyle)
	}

	return strings.Join(lines, "\n")
}

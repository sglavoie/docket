package tui

import (
	"fmt"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

// renderAllSections renders all non-empty sections.
func (m *WeeklyReviewModel) renderAllSections() ([]string, int) {
	var lines []string
	currentIndex := 0
	cursorLineIndex := 0
	separator := renderHorizontalDivider(m.width)

	// Compute column layout from all visible task-section items.
	var colLayout *ColumnLayout
	if m.width >= MinColumnAlignWidth {
		var allTasks []*models.Task
		var allItems []CrossProjectTask
		for _, section := range m.sectionOrder {
			if m.collapsedSections[int(section)] {
				continue
			}
			switch section { //nolint:exhaustive
			case WeeklySectionStaleTasks:
				for _, dt := range m.staleTasks {
					allTasks = append(allTasks, dt.Task)
					allItems = append(allItems, CrossProjectTask{Task: dt.Task, Project: dt.Project})
				}
			case WeeklySectionPriorityMismatch:
				for _, dt := range m.priorityMismatch {
					allTasks = append(allTasks, dt.Task)
					allItems = append(allItems, CrossProjectTask{Task: dt.Task, Project: dt.Project})
				}
			case WeeklySectionUpcomingWeek:
				for _, dt := range m.upcomingWeek {
					allTasks = append(allTasks, dt.Task)
					allItems = append(allItems, CrossProjectTask{Task: dt.Task, Project: dt.Project})
				}
			case WeeklySectionRecurringOverview:
				for _, dt := range m.recurringOverview {
					allTasks = append(allTasks, dt.Task)
					allItems = append(allItems, CrossProjectTask{Task: dt.Task, Project: dt.Project})
				}
			}
		}
		colLayout = ComputeBadgeWidths(allTasks, BadgeWidthOpts{
			HideLowPriority: true,
			IncludeStatus:   true,
			IncludeType:     true,
			IncludeStage:    true,
			ProjectNameFor:  crossProjectNameLookup(allItems),
		})
	}

	rowIdx := 0
	for i, section := range m.sectionOrder {
		if i > 0 {
			lines = append(lines, separator)
		}

		count := m.getSectionItemCount(section)
		collapsed := m.collapsedSections[int(section)]

		// Section header
		headerSelected := currentIndex == m.cursor
		if headerSelected {
			cursorLineIndex = len(lines)
		}
		lines = append(lines, m.renderSectionHeader(section, count, collapsed, headerSelected, m.width))
		currentIndex++

		// Section items (if not collapsed)
		if !collapsed {
			if hint := getWeeklySectionHint(section); hint != "" {
				lines = append(lines, "    "+MutedStyle.Render(hint))
			}
			if section == WeeklySectionPriorityMismatch && m.priorityDriftWarning != "" {
				lines = append(lines, "    "+PreviewWarningStyle.Render(m.priorityDriftWarning))
			}
			for idx := range count {
				isSelected := currentIndex == m.cursor
				if isSelected {
					cursorLineIndex = len(lines)
				}
				lines = append(lines, m.renderSectionItem(section, idx, isSelected, colLayout, rowIdx))
				currentIndex++
				rowIdx++
			}
		}
	}

	// Insert inline preview after cursor task
	if m.previewMode && cursorLineIndex >= 0 && cursorLineIndex < len(lines) {
		if dt := m.getTaskAtCursor(); dt != nil {
			indent := "        " // align with cross-project task line content
			pLines := inlinePreviewLines(dt.Task, indent, m.width)
			if len(pLines) > 0 {
				insertAt := cursorLineIndex + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
				}
			}
		}
	}

	return lines, cursorLineIndex
}

// renderSectionItem renders a single item at the given index within a section.
func (m *WeeklyReviewModel) renderSectionItem(section WeeklyReviewSection, index int, isSelected bool, layout *ColumnLayout, rowIndex int) string {
	switch section {
	case WeeklySectionStaleProjects:
		return m.renderProjectLine(m.staleProjects[index], section, isSelected)
	case WeeklySectionInboxBacklog:
		return m.renderProjectLine(m.inboxBacklog[index], section, isSelected)
	case WeeklySectionBlockedWork:
		return m.renderProjectLine(m.blockedWork[index], section, isSelected)
	case WeeklySectionOvercommitted:
		return m.renderProjectLine(m.overcommitted[index], section, isSelected)
	case WeeklySectionStaleTasks:
		return m.renderTaskLine(m.staleTasks[index], section, isSelected, max(m.width-40, 10), layout, rowIndex)
	case WeeklySectionPriorityMismatch:
		return m.renderTaskLine(m.priorityMismatch[index], section, isSelected, max(m.width-40, 10), layout, rowIndex)
	case WeeklySectionUpcomingWeek:
		return m.renderTaskLine(m.upcomingWeek[index], section, isSelected, max(m.width-40, 10), layout, rowIndex)
	case WeeklySectionRecurringOverview:
		return m.renderTaskLine(m.recurringOverview[index], section, isSelected, max(m.width-40, 10), layout, rowIndex)
	}
	return ""
}

// renderSectionHeader renders a section header.
func (m *WeeklyReviewModel) renderSectionHeader(section WeeklyReviewSection, count int, collapsed bool, isSelected bool, width int) string {
	var style lipgloss.Style
	if m.reviewedSections[section] {
		style = MutedStyle
	} else {
		switch section {
		case WeeklySectionStaleProjects, WeeklySectionStaleTasks:
			style = MutedStyle
		case WeeklySectionBlockedWork:
			style = BlockedStyle
		case WeeklySectionInboxBacklog:
			style = DueTodayStyle
		case WeeklySectionOvercommitted:
			style = OverdueStyle
		case WeeklySectionPriorityMismatch:
			style = DueSoonStyle
		case WeeklySectionRecurringOverview:
			style = SectionHeaderStyle
		default:
			style = SectionHeaderStyle
		}
	}
	checkbox := "[ ] "
	if m.reviewedSections[section] {
		checkbox = "[x] "
	}
	sectionName := getWeeklySectionName(section)
	if section == WeeklySectionPriorityMismatch && m.priorityDriftPct > 0 {
		sectionName = fmt.Sprintf("%s — %d%% high-priority", sectionName, m.priorityDriftPct)
	}
	return renderCollapsibleHeader(checkbox+sectionName, count, collapsed, isSelected, m.cursorFlash, style, width)
}

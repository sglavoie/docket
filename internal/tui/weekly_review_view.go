package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/util"
)

func (m *WeeklyReviewModel) View() string {
	totalCount := m.getTotalItemCount()
	projectCount, taskCount := m.countByType()

	var b strings.Builder

	// Title
	subtitle := "all clear"
	if totalCount > 0 {
		subtitle = fmt.Sprintf("%d projects, %d tasks", projectCount, taskCount)
	}
	b.WriteString(renderFullscreenTitle("Weekly Review", subtitle))
	b.WriteString("\n")

	if totalCount == 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("All systems healthy — your projects are on track."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close."))
		return b.String()
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + blank(1) + footer(2) = 7
	contentHeight := m.height - 7

	if m.totalItems == 0 {
		b.WriteString(MutedStyle.Render("No items in this tab."))
		b.WriteString("\n\n")
		footer := HelpStyle.Render("[/]: tabs")
		b.WriteString(footer)
		return b.String()
	}

	lines, cursorLineIndex := m.renderAllSections()

	// Apply scrolling
	scrollOffset := 0
	if cursorLineIndex >= contentHeight {
		scrollOffset = cursorLineIndex - contentHeight + 1
	}

	visibleLines := lines
	if scrollOffset > 0 && scrollOffset < len(lines) {
		visibleLines = lines[scrollOffset:]
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	b.WriteString(strings.Join(visibleLines, "\n"))
	b.WriteString("\n\n")

	// Status message
	if m.statusMessage != "" {
		b.WriteString(SuccessStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	// Error message
	if m.err != nil {
		b.WriteString(ValidationErrorStyle.Render(m.err.Error()))
		b.WriteString("\n")
	}

	// Footer
	left := HelpStyle.Render("h/j/k/l nav | n/N section | ,s/d/b/x status")
	right := MutedStyle.Render(fmt.Sprintf("%d items", totalCount))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// countByType returns separate counts of project items and task items.
func (m *WeeklyReviewModel) countByType() (int, int) {
	projectCount := len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted)
	taskCount := len(m.staleTasks) + len(m.priorityMismatch) + len(m.upcomingWeek)
	return projectCount, taskCount
}

// renderTabBar renders the tab bar.
func (m *WeeklyReviewModel) renderTabBar() string {
	tabCounts := []int{
		len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted),
		len(m.staleTasks) + len(m.priorityMismatch),
		len(m.upcomingWeek),
	}
	labels := make([]string, len(weeklyReviewTabNames))
	for i, name := range weeklyReviewTabNames {
		labels[i] = fmt.Sprintf("%s (%d)", name, tabCounts[i])
	}
	return renderWrappingTabBar(labels, m.activeTab, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *WeeklyReviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",": StatusPrefixHints,
			"d": DueDatePrefixHints,
			"g": WeeklyReviewNavigationPrefixHints,
			"s": WeeklyReviewSectionPrefixHints,
			"c": CreateLinkedPrefixHints,
			"m": SecondaryViewMovePrefixHints,
		},
	}
}

// WeeklyReviewNavigationPrefixHints contains "g" prefix hints.
var WeeklyReviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project/task", Detail: "Navigate to project or task"},
}

// WeeklyReviewSectionPrefixHints contains "s" prefix hints.
var WeeklyReviewSectionPrefixHints = []PrefixHint{
	{Key: "p", Description: "stale projects", Detail: "Jump to stale projects"},
	{Key: "i", Description: "inbox backlog", Detail: "Jump to inbox backlog"},
	{Key: "b", Description: "blocked work", Detail: "Jump to blocked work"},
	{Key: "o", Description: "overcommitted", Detail: "Jump to overcommitted"},
	{Key: "t", Description: "stale tasks", Detail: "Jump to stale tasks"},
	{Key: "m", Description: "priority mismatch", Detail: "Jump to priority mismatch"},
	{Key: "w", Description: "upcoming week", Detail: "Jump to upcoming week"},
}

// renderAllSections renders all non-empty sections.
func (m *WeeklyReviewModel) renderAllSections() ([]string, int) {
	var lines []string
	currentIndex := 0
	cursorLineIndex := 0
	separator := renderHorizontalDivider(m.width)

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
		lines = append(lines, m.renderSectionHeader(section, count, collapsed, headerSelected))
		currentIndex++

		// Section items (if not collapsed)
		if !collapsed {
			switch section {
			case WeeklySectionStaleProjects:
				for _, wp := range m.staleProjects {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderProjectLine(wp, section, isSelected))
					currentIndex++
				}
			case WeeklySectionInboxBacklog:
				for _, wp := range m.inboxBacklog {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderProjectLine(wp, section, isSelected))
					currentIndex++
				}
			case WeeklySectionBlockedWork:
				for _, wp := range m.blockedWork {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderProjectLine(wp, section, isSelected))
					currentIndex++
				}
			case WeeklySectionOvercommitted:
				for _, wp := range m.overcommitted {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderProjectLine(wp, section, isSelected))
					currentIndex++
				}
			case WeeklySectionStaleTasks:
				for _, dt := range m.staleTasks {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderTaskLine(dt, section, isSelected, m.width-40))
					currentIndex++
				}
			case WeeklySectionPriorityMismatch:
				for _, dt := range m.priorityMismatch {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderTaskLine(dt, section, isSelected, m.width-40))
					currentIndex++
				}
			case WeeklySectionUpcomingWeek:
				for _, dt := range m.upcomingWeek {
					isSelected := currentIndex == m.cursor
					if isSelected {
						cursorLineIndex = len(lines)
					}
					lines = append(lines, m.renderTaskLine(dt, section, isSelected, m.width-40))
					currentIndex++
				}
			}
		}
	}

	return lines, cursorLineIndex
}

// renderSectionHeader renders a section header.
func (m *WeeklyReviewModel) renderSectionHeader(section WeeklyReviewSection, count int, collapsed bool, isSelected bool) string {
	indicator := "▼"
	if collapsed {
		indicator = "▶"
	}

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	name := getWeeklySectionName(section)
	header := fmt.Sprintf("%s%s %s (%d)", cursor, indicator, name, count)

	var styled string
	switch section {
	case WeeklySectionStaleProjects, WeeklySectionStaleTasks:
		styled = MutedStyle.Bold(true).Render(header)
	case WeeklySectionBlockedWork:
		styled = BlockedStyle.Bold(true).Render(header)
	case WeeklySectionInboxBacklog:
		styled = DueTodayStyle.Bold(true).Render(header)
	case WeeklySectionOvercommitted:
		styled = OverdueStyle.Bold(true).Render(header)
	case WeeklySectionPriorityMismatch:
		styled = DueSoonStyle.Bold(true).Render(header)
	default:
		styled = DailyReviewSectionHeaderStyle.Render(header)
	}

	if isSelected {
		return SelectedStyle.Render(styled)
	}
	return styled
}

// renderProjectLine renders a single project line with metric badges.
func (m *WeeklyReviewModel) renderProjectLine(wp WeeklyReviewProject, section WeeklyReviewSection, isSelected bool) string {
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	name := util.Truncate(wp.Project.Name, 25)

	var badges []string
	switch section {
	case WeeklySectionStaleProjects:
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%dd idle", wp.NoActivityDays)))
	case WeeklySectionInboxBacklog:
		badges = append(badges, DueTodayStyle.Render(fmt.Sprintf("%d inbox", wp.InboxCount)))
	case WeeklySectionBlockedWork:
		badges = append(badges, BlockedStyle.Render(fmt.Sprintf("%d blocked", wp.BlockedCount)))
	case WeeklySectionOvercommitted:
		badges = append(badges, OverdueStyle.Render(fmt.Sprintf("%d in-progress", wp.InProgressCount)))
	}

	// Add secondary context badges
	if section != WeeklySectionStaleProjects && wp.NoActivityDays >= 7 {
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%dd idle", wp.NoActivityDays)))
	}
	if section != WeeklySectionInboxBacklog && wp.InboxCount > 0 {
		badges = append(badges, MutedStyle.Render(fmt.Sprintf("%d inbox", wp.InboxCount)))
	}
	if wp.OverdueCount > 0 {
		badges = append(badges, OverdueStyle.Render(fmt.Sprintf("%d overdue", wp.OverdueCount)))
	}

	badgeStr := ""
	if len(badges) > 0 {
		badgeStr = "  " + strings.Join(badges, "  ")
	}

	line := fmt.Sprintf("%s%s%s", cursor, name, badgeStr)

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderTaskLine renders a single task line (same style as daily review).
func (m *WeeklyReviewModel) renderTaskLine(dt WeeklyReviewTask, section WeeklyReviewSection, isSelected bool, maxTitleLen int) string {
	task := dt.Task

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	projectName := util.Truncate(dt.Project.Name, 12)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	priorityStr := ""
	if task.Priority <= 2 {
		priorityStr = renderPriorityBadge(task.Priority) + " "
	}

	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	contextStr := ""
	switch section {
	case WeeklySectionStaleTasks:
		t := task.UpdatedAt
		if t.IsZero() {
			t = task.CreatedAt
		}
		if t.IsZero() {
			contextStr = MutedStyle.Render("no date")
		} else {
			contextStr = MutedStyle.Render(formatRelativeTime(t))
		}
	case WeeklySectionPriorityMismatch:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		} else {
			contextStr = MutedStyle.Render("no due date")
		}
	case WeeklySectionUpcomingWeek:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Mon Jan 2"))
		}
	}

	line := fmt.Sprintf("%s%s %s %s %s%s", cursor, projectStr, statusIcon, typeIcon, priorityStr, title)
	if contextStr != "" {
		lineWidth := len(line)
		padding := m.width - lineWidth - len(contextStr) - 5
		if padding > 0 {
			line = fmt.Sprintf("%-*s %s", m.width-len(contextStr)-5, line, contextStr)
		} else {
			line = line + " " + contextStr
		}
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

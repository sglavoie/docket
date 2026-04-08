package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *WeeklyReviewModel) View() tea.View {
	totalCount := m.getTotalItemCount()
	projectCount, taskCount := m.countByType()

	var b strings.Builder

	// Title
	var subtitle string
	if m.reviewCompleted {
		subtitle = fmt.Sprintf("completed %s at %s", m.completedAt.Format("Mon"), m.completedAt.Format("15:04"))
	} else if totalCount > 0 {
		subtitle = fmt.Sprintf("%d projects, %d tasks | %d/8 reviewed", projectCount, taskCount, m.reviewedCount())
	} else {
		subtitle = "all clear"
	}
	b.WriteString(renderFullscreenTitle("Weekly Review", subtitle, m.AccentColor()))
	b.WriteString("\n")

	if totalCount == 0 {
		b.WriteString("\n")
		b.WriteString(renderEmptyState("All systems healthy — your projects are on track.", "Press q or Esc to close."))
		return tea.NewView(b.String())
	}

	// Tab bar
	chrome, chromeLines := renderFullscreenChrome(m.renderTabBar(), "", true)
	b.WriteString(chrome)

	if m.showingSummary {
		b.WriteString(m.renderCompletionSummary())
		return tea.NewView(b.String())
	}

	contentHeight := fullscreenContentHeight(m.height, chromeLines)

	if m.totalItems == 0 {
		emptyMsg := "No tasks in this tab. Use [/] to switch tabs."
		if m.tabs.Active == 2 {
			emptyMsg = "No tasks due in the upcoming week. Use this tab to review and confirm tasks scheduled for the coming week."
		}
		b.WriteString(lipgloss.NewStyle().Width(m.width).Render(MutedStyle.Render(emptyMsg)))
		b.WriteString("\n\n")
		footer := HelpStyle.Render("[/]: tabs")
		b.WriteString(footer)
		return tea.NewView(b.String())
	}

	lines, cursorLineIndex := m.renderAllSections()

	// Account for preview lines in scroll calculation
	if m.previewMode {
		if dt := m.getTaskAtCursor(); dt != nil {
			previewCount := inlinePreviewLineCount(dt.Task)
			contentHeight -= previewCount
			if contentHeight < 1 {
				contentHeight = 1
			}
		}
	}

	b.WriteString(strings.Join(applyPersistentScroll(lines, cursorLineIndex, contentHeight, &m.scrollOffset), "\n"))

	// Footer — progressively shorten when terminal is narrow
	reviewed := m.reviewedCount()
	fullRight := fmt.Sprintf("%d/8 reviewed | %d items", reviewed, totalCount)
	shortRight := fmt.Sprintf("%d/8 reviewed", reviewed)
	reviewHint := "R: reviewed"
	if m.reviewCompleted {
		reviewHint = "R: unmark"
	}
	fullLeft := "h/j/k/l nav · n/N section · " + reviewHint + " · ,s/d/b/x status"
	shortLeft := "n/N section · " + reviewHint + " · ? help"

	rightText := fullRight
	leftText := fullLeft
	if lipgloss.Width(HelpStyle.Render(fullLeft))+lipgloss.Width(MutedStyle.Render(fullRight))+2 > m.width {
		leftText = shortLeft
	}
	if lipgloss.Width(HelpStyle.Render(leftText))+lipgloss.Width(MutedStyle.Render(fullRight))+2 > m.width {
		rightText = shortRight
	}
	left := HelpStyle.Render(leftText)
	right := MutedStyle.Render(rightText)
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.width, m.AccentColor()))

	return tea.NewView(b.String())
}

// countByType returns separate counts of project items and task items.
func (m *WeeklyReviewModel) countByType() (int, int) {
	projectCount := len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted)
	taskCount := len(m.staleTasks) + len(m.priorityMismatch) + len(m.upcomingWeek) + len(m.recurringOverview)
	return projectCount, taskCount
}

// renderTabBar renders the tab bar.
func (m *WeeklyReviewModel) renderTabBar() string {
	tabCounts := []int{
		len(m.staleProjects) + len(m.inboxBacklog) + len(m.blockedWork) + len(m.overcommitted),
		len(m.staleTasks) + len(m.priorityMismatch),
		len(m.upcomingWeek) + len(m.recurringOverview),
	}
	labels := make([]string, len(m.tabs.Names))
	for i, name := range m.tabs.Names {
		labels[i] = fmt.Sprintf("%s (%d)", name, tabCounts[i])
	}
	return renderWrappingTabBar(labels, m.tabs.Active, m.width, m.AccentColor())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *WeeklyReviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	var currentStage string
	var currentPriority int
	if wt := m.getTaskAtCursor(); wt != nil {
		currentStage = wt.Task.Stage
		currentPriority = wt.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["g"] = WeeklyReviewNavigationPrefixHints
	hints["z"] = append(SectionFoldPrefixHints, PrefixHint{Key: "p", Description: "preview", Detail: "Toggle inline preview"})
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// WeeklyReviewNavigationPrefixHints contains "g" prefix hints.
var WeeklyReviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project/task", Detail: "Navigate to project or task"},
	{Key: "P", Description: "stale projects", Detail: "Focus Stale Projects section"},
	{Key: "i", Description: "inbox backlog", Detail: "Focus Inbox Backlog section"},
	{Key: "b", Description: "blocked work", Detail: "Focus Blocked Work section"},
	{Key: "o", Description: "overcommitted", Detail: "Focus Overcommitted section"},
	{Key: "t", Description: "stale tasks", Detail: "Focus Stale Tasks section"},
	{Key: "m", Description: "priority mismatch", Detail: "Focus Priority Mismatch section"},
	{Key: "w", Description: "upcoming week", Detail: "Focus Upcoming Week section"},
	{Key: "r", Description: "recurring", Detail: "Focus Recurring Overview section"},
}

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
		for _, section := range m.sectionOrder {
			if m.collapsedSections[int(section)] {
				continue
			}
			switch section {
			case WeeklySectionStaleTasks:
				for _, dt := range m.staleTasks {
					allTasks = append(allTasks, dt.Task)
				}
			case WeeklySectionPriorityMismatch:
				for _, dt := range m.priorityMismatch {
					allTasks = append(allTasks, dt.Task)
				}
			case WeeklySectionUpcomingWeek:
				for _, dt := range m.upcomingWeek {
					allTasks = append(allTasks, dt.Task)
				}
			case WeeklySectionRecurringOverview:
				for _, dt := range m.recurringOverview {
					allTasks = append(allTasks, dt.Task)
				}
			}
		}
		colLayout = ComputeTaskLineColumnLayout(allTasks, true)
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
	checkbox := "[ ] "
	if m.reviewedSections[section] {
		checkbox = "[x] "
	}
	return renderCollapsibleHeader(checkbox+getWeeklySectionName(section), count, collapsed, isSelected, m.cursorFlash, style, width)
}

// renderProjectLine renders a single project line with metric badges.
func (m *WeeklyReviewModel) renderProjectLine(wp WeeklyReviewProject, section WeeklyReviewSection, isSelected bool) string {
	cursor := "  "
	if isSelected {
		cursor = "  "
	}

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
	default:
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

	maxNameWidth := 25
	if m.width > 0 {
		maxNameWidth = min(maxNameWidth, max(m.width-lipgloss.Width(cursor)-lipgloss.Width(badgeStr), 1))
	}
	name := truncateToVisualWidth(wp.Project.Name, maxNameWidth)

	line := fmt.Sprintf("%s%s%s", cursor, name, badgeStr)
	if m.width > 0 && lipgloss.Width(line) > m.width {
		excess := lipgloss.Width(line) - m.width
		name = truncateToVisualWidth(wp.Project.Name, max(maxNameWidth-excess, 1))
		line = fmt.Sprintf("%s%s%s", cursor, name, badgeStr)
	}

	if isSelected {
		selStyle := SelectedStyle
		if m.cursorFlash {
			selStyle = selStyle.Background(ColorFlashBg)
		}
		if m.width > 0 {
			line = selStyle.Width(m.width).Render(line)
		} else {
			line = selStyle.Render(line)
		}
	}

	return line
}

// renderTaskLine renders a single task line (same style as daily review).
func (m *WeeklyReviewModel) renderTaskLine(dt WeeklyReviewTask, section WeeklyReviewSection, isSelected bool, maxTitleLen int, layout *ColumnLayout, rowIndex int) string {
	task := dt.Task

	// Context string based on section
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
	case WeeklySectionRecurringOverview:
		if task.DueDate != nil {
			contextStr = DueDateStyle.Render(task.DueDate.Format("Jan 2"))
		}
		if dates := task.NextNDueDates(2); len(dates) > 0 {
			var upcoming []string
			for _, d := range dates {
				upcoming = append(upcoming, d.Format("Jan 2"))
			}
			nextStr := MutedStyle.Render("→ " + strings.Join(upcoming, ", "))
			if contextStr != "" {
				contextStr += " " + nextStr
			} else {
				contextStr = nextStr
			}
		}
	default:
	}

	return RenderCrossProjectTaskLine(task, dt.Project, nil, isSelected, m.width, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:          m.queueIDs[task.ID],
		HideLowPriority: true,
		DueStr:          contextStr,
		ShowRecurrence:  section == WeeklySectionRecurringOverview,
		Flash:           m.cursorFlash,
		Layout:          layout,
		RowIndex:        rowIndex,
	})
}

// getWeeklySectionHint returns a short action hint for a section.
func getWeeklySectionHint(section WeeklyReviewSection) string {
	switch section {
	case WeeklySectionStaleProjects:
		return "Archive or review tasks in these projects"
	case WeeklySectionInboxBacklog:
		return "Process inbox items to keep backlogs manageable"
	case WeeklySectionBlockedWork:
		return "Unblock tasks or reassess blocked work"
	case WeeklySectionOvercommitted:
		return "Reduce WIP — complete or defer in-progress tasks"
	case WeeklySectionStaleTasks:
		return "Update, snooze, or close these stale tasks"
	case WeeklySectionPriorityMismatch:
		return "Start high-priority tasks or lower their priority"
	case WeeklySectionUpcomingWeek:
		return "Confirm these tasks are ready for the coming week"
	case WeeklySectionRecurringOverview:
		return "Review recurring tasks — skip, defer, or adjust patterns"
	}
	return ""
}

// renderCompletionSummary renders the completion summary overlay.
func (m *WeeklyReviewModel) renderCompletionSummary() string {
	var b strings.Builder

	header := SuccessStyle.Bold(true).Render("Review Complete")
	b.WriteString(header)
	b.WriteString("\n\n")

	// Mutation summary
	totalMutations := 0
	for _, count := range m.mutationCounts {
		totalMutations += count
	}
	if totalMutations > 0 {
		fmt.Fprintf(&b, "Changes made during this review: %d\n", totalMutations)
		categoryLabels := []struct {
			key   string
			label string
		}{
			{"status", "Status changes"},
			{"priority", "Priority changes"},
			{"due_date", "Due date changes"},
			{"move", "Section moves"},
			{"delete", "Deletions"},
			{"active", "Active flag toggles"},
			{"tag", "Tag changes"},
			{"snooze", "Snoozes"},
		}
		for _, cl := range categoryLabels {
			if count, ok := m.mutationCounts[cl.key]; ok && count > 0 {
				fmt.Fprintf(&b, "  %s: %d\n", cl.label, count)
			}
		}
	} else {
		b.WriteString("No changes made during this review.\n")
	}
	b.WriteString("\n")

	// Remaining items
	hasRemaining := false
	allSections := []WeeklyReviewSection{
		WeeklySectionStaleProjects, WeeklySectionInboxBacklog,
		WeeklySectionBlockedWork, WeeklySectionOvercommitted,
		WeeklySectionStaleTasks, WeeklySectionPriorityMismatch,
		WeeklySectionUpcomingWeek, WeeklySectionRecurringOverview,
	}
	for _, s := range allSections {
		count := m.getSectionItemCount(s)
		if count > 0 {
			if !hasRemaining {
				b.WriteString("Remaining items:\n")
				hasRemaining = true
			}
			fmt.Fprintf(&b, "  %s: %d\n", getWeeklySectionName(s), count)
		}
	}
	if !hasRemaining {
		b.WriteString(SuccessStyle.Render("All sections cleared!"))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	b.WriteString(HelpStyle.Render("Press q to close, any other key to return to review"))

	return b.String()
}

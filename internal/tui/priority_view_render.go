package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPriorityTabBar renders the tab bar for the priority view.
func (m *PriorityViewModel) renderPriorityTabBar() string {
	hasSubTabs := len(m.subTabNames) > 1
	if hasSubTabs {
		return renderSubTabBarWithFocus(priorityTabNames, m.activeTab, !m.subTabRowFocused, m.width)
	}
	return renderWrappingTabBar(priorityTabNames, m.activeTab, m.width)
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *PriorityViewModel) renderSubTabBar() string {
	if len(m.subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(m.subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, m.subTabNames...)

	return renderSubTabBarWithFocus(tabs, m.subTab, m.subTabRowFocused, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *PriorityViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints: PrefixHints{
			",":   StatusPrefixHints,
			"g":   PriorityViewNavigationPrefixHints,
			"d":   DueDatePrefixHints,
			"c":   CreateLinkedPrefixHints,
			"F":   m.buildPriorityFilterPrefixHints(),
			"Ft":  FilterTypePrefixHints,
			"Fs":  FilterStatusPrefixHints,
			"Fd":  FilterDuePrefixHints,
			"Fa":  FilterAttrPrefixHints,
			"Fas": FilterAttrSeverityPrefixHints,
			"Far": FilterAttrReproduciblePrefixHints,
			"Fae": FilterAttrEffortPrefixHints,
			"Fac": FilterAttrCategoryPrefixHints,
			"Fav": FilterAttrValuePrefixHints,
			"Faw": FilterAttrWorktypePrefixHints,
			"Fao": FilterAttrScopePrefixHints,
			"Fap": FilterAttrPointsPrefixHints,
			"FS":  BuildFilterScorePrefixHints(m.config),
		},
	}
}

// renderSuggestionSection renders the "SUGGESTED NEXT" section at the top.
func (m *PriorityViewModel) renderSuggestionSection() string {
	if m.suggestion == nil {
		return ""
	}

	task := m.suggestion.Task.Task
	boxWidth := m.width - 4

	// Build task line
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Calculate available title width
	// Star + status + type + priority + spacing takes about 15-20 chars
	titleWidth := boxWidth - 22
	if titleWidth < 10 {
		titleWidth = 10
	}
	title := util.Truncate(task.Title, titleWidth)

	// Due date
	dueStr := ""
	if m.suggestion.Task.RelativeDue != "" {
		dueStr = DueDateStyle.Render(m.suggestion.Task.RelativeDue)
	}

	// Star indicator
	star := SuggestionStarStyle.Render("★")

	// Build the task line
	taskLine := fmt.Sprintf("%s %s %s %s %s", star, statusIcon, typeIcon, priorityBadge, title)
	if dueStr != "" {
		taskLine = fmt.Sprintf("%-*s %s", boxWidth-12, taskLine, dueStr)
	}

	// Reason line (muted, indented)
	reasonLine := SuggestionReasonStyle.Render("  " + m.suggestion.Reason)

	// Build the box
	content := taskLine + "\n" + reasonLine
	box := SuggestionBoxStyle.Width(boxWidth).Render(content)

	// Add header
	header := SuggestionHeaderStyle.Render("─ SUGGESTED NEXT ")
	headerPadding := boxWidth - 17
	if headerPadding > 0 {
		header += MutedStyle.Render(strings.Repeat("─", headerPadding))
	}

	return header + "\n" + box
}

// renderAllGroups renders all task groups and returns the lines.
func (m *PriorityViewModel) renderAllGroups(currentIndex *int) []string {
	if m.activeTab == priorityTabByScore {
		return m.renderScoreRanking(currentIndex)
	}

	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.width)

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	if len(overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("OVERDUE", len(overdue), true))
		lines = append(lines, m.renderGroup(overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("DUE TODAY", len(today), false))
		lines = append(lines, m.renderGroup(today, currentIndex, false)...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("THIS WEEK", len(thisWeek), false))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("LATER", len(later), false))
		lines = append(lines, m.renderGroup(later, currentIndex, false)...)
		hasContent = true
	}

	if len(backlog) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroup(backlog, currentIndex, false)...)
		hasContent = true
	}

	// Waiting On section (blocked tasks)
	if len(waitingOn) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		// In Blocked tab, tasks are navigable (rendered as regular tasks)
		if m.activeTab == priorityTabBlocked {
			lines = append(lines, m.renderGroupHeader("BLOCKED / WAITING", len(waitingOn), false))
			lines = append(lines, m.renderGroup(waitingOn, currentIndex, false)...)
		} else {
			lines = append(lines, m.renderWaitingOnHeader())
			if !m.waitingOnCollapsed {
				// Waiting on tasks are not navigable in All tab
				for _, pt := range waitingOn {
					lines = append(lines, m.renderWaitingOnTask(pt))
				}
			}
		}
	}

	return lines
}

// renderScoreRanking renders a flat ranked list of tasks by composite score.
func (m *PriorityViewModel) renderScoreRanking(currentIndex *int) []string {
	if m.config == nil {
		return []string{MutedStyle.Render("No scoring profile assigned. Use : scoring profiles to set one up.")}
	}

	profile := m.config.GetProjectScoringProfile(m.project.ID)
	if profile == nil {
		return []string{MutedStyle.Render("No scoring profile assigned. Use : scoring profiles to set one up.")}
	}

	if len(m.scoredTasks) == 0 {
		return []string{MutedStyle.Render("No tasks to rank.")}
	}

	var lines []string
	maxTitleLen := m.width - 50
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}

	for rank, pt := range m.scoredTasks {
		isSelected := *currentIndex == m.cursor
		task := pt.Task

		cursor := "  "
		if isSelected {
			cursor = "> "
		}

		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
		priorityBadge := renderPriorityBadge(task.Priority)
		title := util.Truncate(task.Title, maxTitleLen)
		scoreBadge := RenderScoreBadge(task, profile)

		line := fmt.Sprintf("%s%d. %s %s %s %s %s", cursor, rank+1, scoreBadge, statusIcon, typeIcon, priorityBadge, title)
		if isSelected {
			line = SelectedStyle.Render(line)
		}
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderGroupHeader renders a group header with count.
func (m *PriorityViewModel) renderGroupHeader(name string, count int, isOverdue bool) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	if isOverdue {
		return OverdueStyle.Bold(true).Render(header)
	}
	return PriorityGroupHeaderStyle.Render(header)
}

// renderWaitingOnHeader renders the collapsible waiting on section header.
func (m *PriorityViewModel) renderWaitingOnHeader() string {
	indicator := "▶"
	if !m.waitingOnCollapsed {
		indicator = "▼"
	}
	header := fmt.Sprintf("%s Waiting On (%d blocked)", indicator, len(m.waitingOn))
	return PriorityWaitingHeaderStyle.Render(header)
}

// renderGroup renders a group of tasks.
func (m *PriorityViewModel) renderGroup(tasks []*PriorityTask, currentIndex *int, isOverdue bool) []string {
	var lines []string
	maxTitleLen := m.width - 45 // Account for other elements

	for _, pt := range tasks {
		isSelected := *currentIndex == m.cursor
		line := m.renderTaskLine(pt, isOverdue, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single actionable task line.
func (m *PriorityViewModel) renderTaskLine(pt *PriorityTask, isOverdue bool, isSelected bool, maxTitleLen int) string {
	task := pt.Task

	// Star indicator for suggested task
	isSuggested := m.suggestion != nil && m.suggestion.Task.Task.ID == task.ID
	starIndicator := "  "
	if isSuggested {
		starIndicator = SuggestionStarStyle.Render("★ ")
	}

	// Cursor indicator
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Status icon with badge style
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Priority badge
	priorityBadge := m.renderPriorityBadge(task.Priority)

	// Title (reduce width if showing star indicator)
	adjustedMaxTitleLen := maxTitleLen
	if isSuggested {
		adjustedMaxTitleLen -= 2 // Account for star indicator
	}
	if adjustedMaxTitleLen < 10 {
		adjustedMaxTitleLen = 10
	}
	title := util.Truncate(task.Title, adjustedMaxTitleLen)

	// Custom section badge
	csStr := ""
	if task.CustomSection != "" {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csStr = " " + csStyle.Render("@ "+task.CustomSection)
	}

	// Tags
	tagsStr := ""
	if len(task.Tags) > 0 {
		tags := task.Tags
		if len(tags) > 2 {
			tags = tags[:2]
		}
		tagsStr = MutedStyle.Render(" #" + strings.Join(tags, " #"))
	}

	// Due date
	dueStr := ""
	if pt.RelativeDue != "" {
		if isOverdue {
			dueStr = OverdueStyle.Render(pt.RelativeDue)
		} else if pt.RelativeDue == "today" {
			dueStr = DueTodayStyle.Render(pt.RelativeDue)
		} else {
			dueStr = DueDateStyle.Render(pt.RelativeDue)
		}
	}

	// Composite score badge
	scoreBadge := ""
	if m.config != nil {
		if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
			scoreBadge = " " + RenderScoreBadge(task, profile)
		}
	}

	// Build line with star indicator before cursor
	line := fmt.Sprintf("%s%s%s %s %s %s%s%s%s", starIndicator, cursor, statusIcon, typeIcon, priorityBadge, title, scoreBadge, csStr, tagsStr)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderWaitingOnTask renders a blocked task in the waiting on section.
func (m *PriorityViewModel) renderWaitingOnTask(pt *PriorityTask) string {
	task := pt.Task
	maxTitleLen := m.width - 50

	// Blocked status icon
	statusIcon := BlockedStyle.Render("◈")

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Blocking reason
	blockingReason := m.getBlockingReason(task)

	return fmt.Sprintf("    %s %s %s %s", statusIcon, typeIcon, title, MutedStyle.Render(blockingReason))
}

// getBlockingReason returns a description of what's blocking a task.
func (m *PriorityViewModel) getBlockingReason(task *models.Task) string {
	if task.Status == models.StatusBlocked {
		return "(blocked)"
	}

	// Find incomplete dependencies
	for _, depID := range task.DependsOn {
		if dep := m.project.GetTaskByID(depID); dep != nil {
			if !dep.Status.IsComplete() {
				return fmt.Sprintf("← %s", util.Truncate(dep.Title, 20))
			}
		}
	}

	return "(blocked)"
}

// renderPriorityBadge delegates to the shared renderPriorityBadge helper.
func (m *PriorityViewModel) renderPriorityBadge(priority int) string {
	return renderPriorityBadge(priority)
}

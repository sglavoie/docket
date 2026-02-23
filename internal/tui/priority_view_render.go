package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPriorityTabBar renders the tab bar for the priority view.
func (m *PriorityViewModel) renderPriorityTabBar() string {
	return renderTabBarWithOptionalSubTabs(priorityTabNames, m.tabs.Active, m.subTabNames, m.subTabRowFocused, m.Width())
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *PriorityViewModel) renderSubTabBar() string {
	return renderDynamicSubTabBar(m.subTabNames, m.subTab, m.subTabRowFocused, m.Width())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *PriorityViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"g": PriorityViewNavigationPrefixHints,
		"d": DueDatePrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"c": CreateLinkedPrefixHints,
	}
	// Merge filter prefix hints (due date + status both included)
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true) {
		hints[k] = v
	}
	// Insert "r" (ready) hint into F hints before the last entry (clear-all)
	if fHints, ok := hints["F"]; ok {
		readyDetail := "off"
		if m.filter.Actionable {
			readyDetail = "on"
		}
		readyHint := PrefixHint{Key: "r", Description: "ready", Detail: readyDetail, Active: m.filter.Actionable}
		hints["F"] = append(fHints[:len(fHints)-1], readyHint, fHints[len(fHints)-1])
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// renderSuggestionSection renders the "SUGGESTED NEXT" section at the top.
func (m *PriorityViewModel) renderSuggestionSection() string {
	if m.suggestion == nil {
		return ""
	}

	task := m.suggestion.Task.Task
	boxWidth := m.Width() - 4

	// Calculate available title width
	// Star + cursor + status + type + priority + spacing takes about 15-20 chars
	titleWidth := boxWidth - 22

	// Due date
	dueStr := ""
	if m.suggestion.Task.RelativeDue != "" {
		dueStr = DueDateStyle.Render(m.suggestion.Task.RelativeDue)
	}

	// Star indicator
	star := SuggestionStarStyle.Render("★")

	// Build the task line
	taskLine := TaskLine{
		Task:      task,
		MaxTitle:  titleWidth,
		Prefix:    star,
		DueStr:    dueStr,
		ViewWidth: boxWidth,
	}.Render()

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
	if m.tabs.Active == priorityTabByScore {
		return m.renderScoreRanking(currentIndex)
	}

	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.Width())

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	if len(overdue) > 0 {
		lines = append(lines, renderGroupHeader("OVERDUE", len(overdue), true))
		lines = append(lines, m.renderGroup(overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("DUE TODAY", len(today), false))
		lines = append(lines, m.renderGroup(today, currentIndex, false)...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("THIS WEEK", len(thisWeek), false))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("LATER", len(later), false))
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
		if m.tabs.Active == priorityTabBlocked {
			lines = append(lines, renderGroupHeader("BLOCKED / WAITING", len(waitingOn), false))
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

	scored := m.visibleScoredTasks()
	if len(scored) == 0 {
		return []string{MutedStyle.Render("No tasks to rank.")}
	}

	var lines []string
	maxTitleLen := m.Width() - 50

	for rank, pt := range scored {
		isSelected := *currentIndex == m.cursor
		scoreBadge := RenderScoreBadge(pt.Task, profile)

		// Progress fraction for parent tasks
		progressStr := ""
		if frac := renderProgressFraction(pt.Task); frac != "" {
			progressStr = " " + frac
		}

		line := TaskLine{
			Task:       pt.Task,
			IsSelected: isSelected,
			MaxTitle:   maxTitleLen,
			PreBadges:  fmt.Sprintf("%d. %s ", rank+1, scoreBadge),
			PostTitle:  progressStr,
		}.Render()
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
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
	maxTitleLen := m.Width() - 45 // Account for other elements

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
	starPrefix := "  "
	if isSuggested {
		starPrefix = SuggestionStarStyle.Render("★ ")
	}

	// Adjust title width for star indicator
	adjustedMaxTitleLen := maxTitleLen
	if isSuggested {
		adjustedMaxTitleLen -= 2
	}

	// Custom section badge
	csStr := ""
	if task.CustomSection != "" {
		csColor := m.project.GetSectionColor(task.CustomSection)
		csStyle := GetSectionColorStyle(csColor)
		csStr = " " + csStyle.Render("@ "+task.CustomSection)
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

	// Progress fraction for parent tasks
	progressStr := ""
	if frac := renderProgressFraction(task); frac != "" {
		progressStr = " " + frac
	}

	return TaskLine{
		Task:       task,
		IsSelected: isSelected,
		MaxTitle:   adjustedMaxTitleLen,
		Prefix:     starPrefix,
		PostTitle:  scoreBadge + csStr + progressStr + renderInlineTags(task.Tags),
		DueStr:     dueStr,
		ViewWidth:  m.Width(),
	}.Render()
}

// renderWaitingOnTask renders a blocked task in the waiting on section.
func (m *PriorityViewModel) renderWaitingOnTask(pt *PriorityTask) string {
	task := pt.Task
	maxTitleLen := m.Width() - 50

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

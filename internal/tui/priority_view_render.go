package tui

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
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
	var currentStage string
	var currentPriority int
	if pt := m.getTaskAtCursor(); pt != nil {
		currentStage = pt.Task.Stage
		currentPriority = pt.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["g"] = PriorityViewNavigationPrefixHints
	hints["z"] = []PrefixHint{
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	// Merge filter prefix hints (due date + status both included)
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true))
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
	titleWidth := max(boxWidth-22, 10)

	// Due date
	dueStr := ""
	if m.suggestion.Task.RelativeDue != "" {
		dueStr = DueDateStyle.Render(m.suggestion.Task.RelativeDue)
	}

	// Star indicator
	star := SuggestionStarStyle.Render("★")

	// Build the task line
	taskLine := TaskLine{
		Task:       task,
		MaxTitle:   titleWidth,
		Prefix:     star,
		StageBadge: GetStageBadge(task.Stage),
		DueStr:     dueStr,
		ViewWidth:  boxWidth,
		Flash:      m.cursorFlash,
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
func (m *PriorityViewModel) renderAllGroups(currentIndex *int) ([]string, int) {
	cursorLineIdx := 0

	if m.tabs.Active == priorityTabByScore {
		lines := m.renderScoreRanking(currentIndex, &cursorLineIdx)
		return lines, cursorLineIdx
	}

	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.Width())

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	if len(overdue) > 0 {
		lines = append(lines, renderGroupHeader("OVERDUE", len(overdue), true, m.Width()))
		lines = append(lines, m.renderGroup(overdue, currentIndex, true, &cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("DUE TODAY", len(today), false, m.Width()))
		lines = append(lines, m.renderGroup(today, currentIndex, false, &cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("THIS WEEK", len(thisWeek), false, m.Width()))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, false, &cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("LATER", len(later), false, m.Width()))
		lines = append(lines, m.renderGroup(later, currentIndex, false, &cursorLineIdx, len(lines))...)
		hasContent = true
	}

	if len(backlog) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroup(backlog, currentIndex, false, &cursorLineIdx, len(lines))...)
		hasContent = true
	}

	// Waiting On section (blocked tasks)
	if len(waitingOn) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		// In Blocked tab, tasks are navigable (rendered as regular tasks)
		if m.tabs.Active == priorityTabBlocked {
			lines = append(lines, renderGroupHeader("BLOCKED / WAITING", len(waitingOn), false, m.Width()))
			lines = append(lines, m.renderGroup(waitingOn, currentIndex, false, &cursorLineIdx, len(lines))...)
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

	return lines, cursorLineIdx
}

// renderScoreRanking renders a flat ranked list of tasks by composite score.
func (m *PriorityViewModel) renderScoreRanking(currentIndex *int, cursorLineIdx *int) []string {
	if m.config == nil {
		return []string{MutedStyle.Render("No scoring profile assigned. Use : scoring profiles to set one up.")}
	}

	profile := m.config.GetProjectScoringProfile(m.project.ID)
	if profile == nil {
		return []string{MutedStyle.Render("No scoring profile assigned. Use : scoring profiles to set one up.")}
	}

	scored := m.visibleScoredTasks()
	if len(scored) == 0 {
		return []string{MutedStyle.Render("No tasks to rank. Set score dimensions with Shift+S on tasks.")}
	}

	var lines []string
	maxTitleLen := m.Width() - 50

	for rank, pt := range scored {
		isSelected := *currentIndex == m.cursor
		if isSelected {
			*cursorLineIdx = len(lines)
		}
		scoreBadge := RenderScoreBadge(pt.Task, profile)

		// Progress fraction for parent tasks
		postTitle := renderProgressFraction(pt.Task)
		if postTitle != "" {
			postTitle = " " + postTitle
		}

		line := TaskLine{
			Task:        pt.Task,
			IsSelected:  isSelected,
			MaxTitle:    maxTitleLen,
			PreBadges:   fmt.Sprintf("%d. %s ", rank+1, scoreBadge),
			StageBadge:  GetStageBadge(pt.Task.Stage),
			PostTitle:   postTitle,
			Flash:       m.cursorFlash,
			SearchQuery: m.searchQuery,
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
func (m *PriorityViewModel) renderGroup(tasks []*PriorityTask, currentIndex *int, isOverdue bool, cursorLineIdx *int, baseLine int) []string {
	var lines []string
	maxTitleLen := m.Width() - 45 // Account for other elements

	for _, pt := range tasks {
		isSelected := *currentIndex == m.cursor
		if isSelected {
			*cursorLineIdx = baseLine + len(lines)
		}
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
			scoreBadge = RenderScoreBadge(task, profile)
		}
	}

	// Build indicator groups
	structureGroup := JoinGroupParts(scoreBadge, renderProgressFraction(task))
	orgGroup := strings.TrimSpace(csStr)
	contentGroup := renderTruncatedTags(task.Tags, 2, 8)
	postTitle := JoinIndicatorGroups(structureGroup, orgGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:        task,
		IsSelected:  isSelected,
		MaxTitle:    adjustedMaxTitleLen,
		Prefix:      starPrefix,
		StageBadge:  GetStageBadge(task.Stage),
		PostTitle:   postTitle,
		DueStr:      dueStr,
		ViewWidth:   m.Width(),
		Flash:       m.cursorFlash,
		SearchQuery: m.searchQuery,
	}.Render()
}

// renderWaitingOnTask renders a blocked task in the waiting on section.
func (m *PriorityViewModel) renderWaitingOnTask(pt *PriorityTask) string {
	task := pt.Task

	// Blocked status icon
	statusIcon := BlockedStyle.Render(StatusIconBlocked)

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Blocking reason
	blockingReason := m.getBlockingReason(task)
	prefix := fmt.Sprintf("    %s %s ", statusIcon, typeIcon)
	suffix := " " + MutedStyle.Render(blockingReason)
	titleWidth := max(m.Width()-lipgloss.Width(prefix)-lipgloss.Width(suffix), 1)
	title := truncateToVisualWidth(task.Title, titleWidth)
	if m.searchQuery != "" {
		title = HighlightSubstring(title, m.searchQuery, NormalStyle)
	}

	line := prefix + title + suffix
	if task.Dimmed {
		// Replace leading space with ◌ indicator before dimming
		line = "◌" + line[1:]
		line = DimLine(line)
	}
	return line
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
				return fmt.Sprintf("← %s", truncateToVisualWidth(dep.Title, 20))
			}
		}
	}

	return "(blocked)"
}

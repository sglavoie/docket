package tui

import (
	"fmt"
	"maps"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/icons"
	"github.com/sglavoie/tt/internal/models"
)

// renderPriorityTabBar renders the tab bar for the priority view.
func (m *PriorityViewModel) renderPriorityTabBar() string {
	return renderTabBarWithOptionalSubTabs(priorityTabNames, m.tabs.Active, m.subTabNames, m.subTabRowFocused, m.Width(), m.AccentColor())
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *PriorityViewModel) renderSubTabBar() string {
	return renderDynamicSubTabBar(m.subTabNames, m.subTab, m.subTabRowFocused, m.Width(), m.AccentColor())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *PriorityViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	var currentPriority int
	if pt := m.getTaskAtCursor(); pt != nil {
		currentPriority = pt.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(currentPriority)
	hints["g"] = PriorityViewNavigationPrefixHints
	hints["f"] = []PrefixHint{
		{Key: "p", Description: "preview", Detail: "Toggle inline preview"},
	}
	// Merge filter prefix hints (due date + status both included)
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.runtime.Machine(), true, true, false))
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
	star := SuggestionStarStyle.Render(icons.Get("star"))

	postTitle := ""
	if m.runtime.Machine().IsDescriptionColumnVisible() {
		if descBadge := renderDescriptionIndicator(task); descBadge != "" {
			postTitle = " " + descBadge
		}
	}

	// Build the task line
	taskLine := TaskLine{
		Task:      task,
		Project:   m.project,
		Queued:    m.queueIDs[task.ID],
		Pinned:    task.Pinned,
		MaxTitle:  titleWidth,
		Prefix:    star,
		PostTitle: postTitle,
		DueStr:    dueStr,
		ViewWidth: boxWidth,
		Flash:     m.cursorFlash,
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

// computeColumnLayout builds a ColumnLayout for the current tab's visible tasks.
func (m *PriorityViewModel) computeColumnLayout() *ColumnLayout {
	if m.Width() < MinColumnAlignWidth {
		return nil
	}
	opts := BadgeWidthOpts{
		IncludeStatus: true,
		IncludeType:   true,
	}
	if m.tabs.Active == priorityTabByScore {
		scored := m.visibleScoredTasks()
		tasks := make([]*models.Task, 0, len(scored))
		for _, pt := range scored {
			tasks = append(tasks, pt.Task)
		}
		return ComputeBadgeWidths(tasks, opts)
	}
	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()
	var tasks []*models.Task
	for _, g := range [][]*PriorityTask{overdue, today, thisWeek, later, backlog, waitingOn} {
		for _, pt := range g {
			tasks = append(tasks, pt.Task)
		}
	}
	return ComputeBadgeWidths(tasks, opts)
}

// renderAllGroups renders all task groups and returns the lines.
func (m *PriorityViewModel) renderAllGroups(currentIndex *int) ([]string, int) {
	cursorLineIdx := 0
	colLayout := m.computeColumnLayout()
	rowIndex := 0

	if m.tabs.Active == priorityTabByScore {
		lines := m.renderScoreRanking(currentIndex, &cursorLineIdx, &rowIndex, colLayout)
		return lines, cursorLineIdx
	}

	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.Width())

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroupsWithSubTab()

	// Standard groups: each has an optional header and isOverdue flag.
	// An empty name means no header (used for the backlog group).
	type taskGroup struct {
		name      string
		tasks     []*PriorityTask
		isOverdue bool
	}
	groups := []taskGroup{
		{"OVERDUE", overdue, true},
		{"DUE TODAY", today, false},
		{"THIS WEEK", thisWeek, false},
		{"LATER", later, false},
		{"", backlog, false},
	}
	for _, g := range groups {
		if len(g.tasks) == 0 {
			continue
		}
		if hasContent {
			lines = append(lines, separator)
		}
		if g.name != "" {
			lines = append(lines, renderGroupHeader(g.name, len(g.tasks), g.isOverdue, m.Width()))
		}
		lines = append(lines, m.renderGroup(g.tasks, currentIndex, g.isOverdue, &cursorLineIdx, len(lines), &rowIndex, colLayout)...)
		hasContent = true
	}

	// Waiting On section (blocked tasks) — special rendering in non-Blocked tabs
	if len(waitingOn) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		if m.tabs.Active == priorityTabBlocked {
			lines = append(lines, renderGroupHeader("BLOCKED / WAITING", len(waitingOn), false, m.Width()))
			lines = append(lines, m.renderGroup(waitingOn, currentIndex, false, &cursorLineIdx, len(lines), &rowIndex, colLayout)...)
		} else {
			lines = append(lines, m.renderWaitingOnHeader())
			if !m.waitingOnCollapsed {
				for _, pt := range waitingOn {
					lines = append(lines, m.renderWaitingOnTask(pt))
				}
			}
		}
	}

	return lines, cursorLineIdx
}

// renderScoreRanking renders a flat ranked list of tasks by composite score.
func (m *PriorityViewModel) renderScoreRanking(currentIndex *int, cursorLineIdx *int, rowIndex *int, layout *ColumnLayout) []string {
	if m.runtime == nil {
		return []string{MutedStyle.Render("No scoring profile assigned. Use : scoring profiles to set one up.")}
	}

	profile := m.runtime.Automation().GetProjectScoringProfile(m.project.ID)
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
		if m.runtime.Machine().IsDescriptionColumnVisible() {
			if descBadge := renderDescriptionIndicator(pt.Task); descBadge != "" {
				postTitle = JoinGroupParts(descBadge, postTitle)
			}
		}
		if postTitle != "" {
			postTitle = " " + postTitle
		}

		line := TaskLine{
			Task:         pt.Task,
			Project:      m.project,
			IsSelected:   isSelected,
			Queued:       m.queueIDs[pt.Task.ID],
			DailyPlanned: m.dailyPlanIDs[pt.Task.ID],
			Pinned:       pt.Task.Pinned,
			MaxTitle:     maxTitleLen,
			PreBadges:    fmt.Sprintf("%d. %s ", rank+1, scoreBadge),
			PostTitle:    postTitle,
			Flash:        m.cursorFlash,
			SearchQuery:  m.searchQuery,
			Layout:       layout,
			RowIndex:     *rowIndex,
		}.Render()
		lines = append(lines, line)
		*currentIndex++
		*rowIndex++
	}

	return lines
}

// renderWaitingOnHeader renders the collapsible waiting on section header.
func (m *PriorityViewModel) renderWaitingOnHeader() string {
	indicator := IconCollapsed
	if !m.waitingOnCollapsed {
		indicator = IconExpanded
	}
	header := fmt.Sprintf("%s Waiting On (%d blocked)", indicator, len(m.waitingOn))
	return PriorityWaitingHeaderStyle.Render(header)
}

// renderGroup renders a group of tasks.
func (m *PriorityViewModel) renderGroup(tasks []*PriorityTask, currentIndex *int, isOverdue bool, cursorLineIdx *int, baseLine int, rowIndex *int, layout *ColumnLayout) []string {
	var lines []string
	maxTitleLen := m.Width() - 45 // Account for other elements

	for _, pt := range tasks {
		isSelected := *currentIndex == m.cursor
		if isSelected {
			*cursorLineIdx = baseLine + len(lines)
		}
		line := m.renderTaskLine(pt, isOverdue, isSelected, maxTitleLen, *rowIndex, layout)
		lines = append(lines, line)
		*currentIndex++
		*rowIndex++
	}

	return lines
}

// renderTaskLine renders a single actionable task line.
func (m *PriorityViewModel) renderTaskLine(pt *PriorityTask, isOverdue bool, isSelected bool, maxTitleLen int, rowIndex int, layout *ColumnLayout) string {
	task := pt.Task

	// Star indicator for suggested task
	isSuggested := m.suggestion != nil && m.suggestion.Task.Task.ID == task.ID
	starPrefix := "  "
	if isSuggested {
		starPrefix = SuggestionStarStyle.Render(icons.Get("star") + " ")
	}
	starPrefix = m.Checkbox(task.ID) + starPrefix

	// Adjust title width for star indicator
	adjustedMaxTitleLen := maxTitleLen
	if isSuggested {
		adjustedMaxTitleLen -= 2
	}

	csStr := ""

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
	if m.runtime != nil {
		if profile := m.runtime.Automation().GetProjectScoringProfile(m.project.ID); profile != nil {
			scoreBadge = RenderScoreBadge(task, profile)
		}
	}

	// Build indicator groups
	structureGroup := JoinGroupParts(scoreBadge, renderProgressFraction(task))
	orgGroup := strings.TrimSpace(csStr)
	descBadge := ""
	if m.runtime.Machine().IsDescriptionColumnVisible() {
		descBadge = renderDescriptionIndicator(task)
	}
	contentGroup := JoinGroupParts(descBadge, renderTruncatedTags(task.Tags, 2, 8))
	postTitle := JoinIndicatorGroups(structureGroup, orgGroup, contentGroup)
	if postTitle != "" {
		postTitle = " " + postTitle
	}

	return TaskLine{
		Task:         task,
		Project:      m.project,
		IsSelected:   isSelected,
		Queued:       m.queueIDs[task.ID],
		DailyPlanned: m.dailyPlanIDs[task.ID],
		Pinned:       task.Pinned,
		MaxTitle:     adjustedMaxTitleLen,
		Prefix:       starPrefix,
		PostTitle:    postTitle,
		DueStr:       dueStr,
		ViewWidth:    m.Width(),
		Flash:        m.cursorFlash,
		SearchQuery:  m.searchQuery,
		Layout:       layout,
		RowIndex:     rowIndex,
	}.Render()
}

// renderWaitingOnTask renders a blocked task in the waiting on section.
func (m *PriorityViewModel) renderWaitingOnTask(pt *PriorityTask) string {
	task := pt.Task

	// Blocked status icon
	statusIcon := BlockedStyle.Render(icons.Get("status.blocked"))

	// Type icon with badge style
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())

	// Blocking reason
	blockingReason := m.getBlockingReason(task)
	prefix := fmt.Sprintf("    %s %s ", statusIcon, typeIcon)
	suffix := " " + MutedStyle.Render(blockingReason)
	titleWidth := max(m.Width()-lipgloss.Width(prefix)-lipgloss.Width(suffix), 1)
	title := truncateToVisualWidth(task.Title, titleWidth)
	titleStyle := TaskTitleStyleFor(task, TaskTitleStyleOptions{})
	if m.searchQuery != "" {
		title = HighlightSubstring(title, m.searchQuery, titleStyle)
	} else {
		title = titleStyle.Render(title)
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
	if task.IsInState(models.StateWaiting) {
		return "(blocked)"
	}

	// Find incomplete dependencies
	for _, depID := range task.DependsOn {
		if dep := m.project.GetTaskByID(depID); dep != nil {
			if !dep.IsTerminalState() {
				return fmt.Sprintf("← %s", truncateToVisualWidth(dep.Title, 20))
			}
		}
	}

	return "(blocked)"
}

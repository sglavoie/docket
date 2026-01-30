package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// renderPriorityTabBar renders the tab bar for the priority view.
func (m *PriorityViewModel) renderPriorityTabBar() string {
	cardWidth := min(m.width-6, 54)
	availableWidth := cardWidth - 2

	plainStyle := lipgloss.NewStyle().Padding(0, 1)
	totalWidth := 0
	for _, name := range priorityTabNames {
		totalWidth += lipgloss.Width(plainStyle.Render(name))
	}
	fitsSingleLine := totalWidth <= availableWidth

	activeStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Padding(0, 1)

	inactiveStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	if fitsSingleLine {
		activeStyle = activeStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.ThickBorder()).
			BorderForeground(ColorPrimary)
		inactiveStyle = inactiveStyle.
			BorderBottom(true).
			BorderStyle(lipgloss.Border{Bottom: "─"}).
			BorderForeground(ColorBorder)
	}

	var tabs []string
	for i, name := range priorityTabNames {
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(name))
		} else {
			tabs = append(tabs, inactiveStyle.Render(name))
		}
	}

	row := lipgloss.JoinHorizontal(lipgloss.Bottom, tabs...)

	if fitsSingleLine {
		tabWidth := lipgloss.Width(row)
		remaining := availableWidth - tabWidth
		if remaining > 0 {
			filler := lipgloss.NewStyle().
				BorderBottom(true).
				BorderStyle(lipgloss.Border{Bottom: "─"}).
				BorderForeground(ColorBorder).
				Width(remaining).
				Render("")
			row = lipgloss.JoinHorizontal(lipgloss.Bottom, row, filler)
		}
	}

	return row
}

// renderPrefixPopup renders the prefix key popup.
func (m *PriorityViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"g": PriorityViewNavigationPrefixHints,
		"d": DueDatePrefixHints,
		"c": CreateLinkedPrefixHints,
	}, m.width)
}

// renderSuggestionSection renders the "SUGGESTED NEXT" section at the top.
func (m *PriorityViewModel) renderSuggestionSection() string {
	if m.suggestion == nil {
		return ""
	}

	task := m.suggestion.Task.Task
	boxWidth := min(m.width-6, 54) // Constrain box width

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
	var lines []string
	hasContent := false
	separatorWidth := min(m.width-10, 50)

	overdue, today, thisWeek, later, backlog, waitingOn := m.filteredGroups()

	if len(overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("OVERDUE", len(overdue), true))
		lines = append(lines, m.renderGroup(overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("DUE TODAY", len(today), false))
		lines = append(lines, m.renderGroup(today, currentIndex, false)...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("THIS WEEK", len(thisWeek), false))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroupHeader("LATER", len(later), false))
		lines = append(lines, m.renderGroup(later, currentIndex, false)...)
		hasContent = true
	}

	if len(backlog) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
		}
		lines = append(lines, m.renderGroup(backlog, currentIndex, false)...)
		hasContent = true
	}

	// Waiting On section (blocked tasks)
	if len(waitingOn) > 0 {
		if hasContent {
			lines = append(lines, MutedStyle.Render(strings.Repeat("─", separatorWidth)))
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

	// Build line with star indicator before cursor
	line := fmt.Sprintf("%s%s%s %s %s %s%s", starIndicator, cursor, statusIcon, typeIcon, priorityBadge, title, tagsStr)
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

// renderPriorityBadge renders a priority indicator.
func (m *PriorityViewModel) renderPriorityBadge(priority int) string {
	badge := fmt.Sprintf("!%d", priority)
	switch priority {
	case 1:
		return PriorityHighStyle.Render(badge)
	case 2:
		return PriorityMediumStyle.Render(badge)
	default:
		return PriorityLowStyle.Render(badge)
	}
}

package tui

import (
	"fmt"
	"strings"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *DueDateOverviewModel) View() string {
	var b strings.Builder

	allTotal := len(m.overdue) + len(m.today) + len(m.thisWeek) + len(m.later)

	// Title
	subtitle := fmt.Sprintf("%d tasks across all projects", allTotal)
	b.WriteString(renderFullscreenTitle("Due Dates", subtitle))
	b.WriteString("\n")

	if allTotal == 0 {
		b.WriteString("\n")
		if m.HasActiveFilters() {
			b.WriteString(m.renderActiveFilterIndicator())
			b.WriteString("\n\n")
			b.WriteString(MutedStyle.Render(EmptyStateFiltered))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("Press q or Esc to clear filters."))
		} else {
			b.WriteString(MutedStyle.Render("No tasks with due dates found."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("Press q or Esc to close"))
		}
		return b.String()
	}

	// Tab bar
	b.WriteString(renderWrappingTabBar(dueDateTabNames, m.activeTab, m.Width()))
	b.WriteString("\n")

	// Filter badge
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if m.totalTasks == 0 {
		if m.HasActiveFilters() {
			b.WriteString(MutedStyle.Render(EmptyStateFiltered))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("[/] h/l tabs · Fx clear filters · q/Esc clear filters"))
		} else {
			b.WriteString(MutedStyle.Render("No tasks in this category."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("[/] h/l tabs · q/Esc close"))
		}
		return b.String()
	}

	// Calculate available height for task list
	contentHeight := fullscreenContentHeight(m.Height(), 3) // tab bar(2) + blank(1)
	if filterBadge != "" {
		contentHeight--
	}

	// Render groups with scroll awareness
	currentIndex := 0
	lines := m.renderAllGroups(&currentIndex)

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	b.WriteString(renderStatusMessages(m.statusMessage, m.Error()))

	// Footer
	left := HelpStyle.Render("[/] tabs · ↑/↓ navigate · ,s/d/b/x/o status · 1-5 priority · dt/dm/dw due · e edit · ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.Width()))

	return b.String()
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *DueDateOverviewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": DueDateOverviewNavigationPrefixHints,
		"m": SecondaryViewMovePrefixHints,
	}
	// Merge filter prefix hints (no DueDate category — the view IS organized by due date)
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, true) {
		hints[k] = v
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// DueDateOverviewNavigationPrefixHints contains "g" prefix hints for due date overview.
var DueDateOverviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
}

// renderAllGroups renders all task groups and returns the lines.
func (m *DueDateOverviewModel) renderAllGroups(currentIndex *int) []string {
	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.Width())

	overdue, today, thisWeek, later := m.visibleGroups()

	if len(overdue) > 0 {
		lines = append(lines, renderGroupHeader("Overdue", len(overdue), true))
		lines = append(lines, m.renderGroup(overdue, currentIndex)...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("Today", len(today), false))
		lines = append(lines, m.renderGroup(today, currentIndex)...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("This Week", len(thisWeek), false))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex)...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, renderGroupHeader("Later", len(later), false))
		lines = append(lines, m.renderGroup(later, currentIndex)...)
	}

	return lines
}

// renderGroup renders a group of tasks.
func (m *DueDateOverviewModel) renderGroup(tasks []DueDateTask, currentIndex *int) []string {
	var lines []string
	maxTitleLen := m.Width() - 35 // Account for date, project, icons

	for _, dt := range tasks {
		isSelected := *currentIndex == m.cursor
		line := m.renderTaskLine(dt, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single task line.
func (m *DueDateOverviewModel) renderTaskLine(dt DueDateTask, isSelected bool, maxTitleLen int) string {
	dateStr := m.formatDueDate(dt.Task)
	projectName := util.Truncate(dt.Project.Name, 15)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))
	return TaskLine{
		Task:       dt.Task,
		IsSelected: isSelected,
		MaxTitle:   maxTitleLen,
		PreBadges:  dateStr + " " + projectStr + " ",
		ViewWidth:  m.Width(),
	}.Render()
}

// formatDueDate formats the due date for display.
func (m *DueDateOverviewModel) formatDueDate(task *models.Task) string {
	if task.DueDate == nil {
		return "         "
	}
	dateStr, style := ClassifyDueDate(task.DueDate)
	return style.Render(fmt.Sprintf("%-9s", dateStr))
}

package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

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
		b.WriteString(MutedStyle.Render("No tasks with due dates found."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q or Esc to close"))
		return b.String()
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	if m.totalTasks == 0 {
		b.WriteString(MutedStyle.Render("No tasks in this category."))
		b.WriteString("\n\n")
		footer := HelpStyle.Render("[/] h/l tabs • q/Esc close")
		b.WriteString(footer)
		return b.String()
	}

	// Calculate available height for task list
	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + blank(1) + footer(2) = 7
	contentHeight := m.height - 7

	// Render groups with scroll awareness
	currentIndex := 0
	lines := m.renderAllGroups(&currentIndex)

	// Apply scrolling if needed
	scrollOffset := 0
	if m.cursor >= contentHeight {
		scrollOffset = m.cursor - contentHeight + 1
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

	// Prefix popup
	if popup := m.renderPrefixPopup(); popup != "" {
		b.WriteString(popup)
		b.WriteString("\n")
	}

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
	left := HelpStyle.Render("[/] tabs • ↑/↓ navigate • ,s/d/b/x/o status • 1-5 priority • dt/dm/dw due • e edit • ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalTasks))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *DueDateOverviewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": DueDateOverviewNavigationPrefixHints,
	}, m.width)
}

// DueDateOverviewNavigationPrefixHints contains "g" prefix hints for due date overview.
var DueDateOverviewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first"},
	{Key: "p", Description: "go to project"},
}

// renderTabBar renders the tab bar for filtering.
func (m *DueDateOverviewModel) renderTabBar() string {
	return renderWrappingTabBar(dueDateTabNames, m.activeTab, m.width)
}

// renderAllGroups renders all task groups and returns the lines.
func (m *DueDateOverviewModel) renderAllGroups(currentIndex *int) []string {
	var lines []string
	hasContent := false
	separator := renderHorizontalDivider(m.width)

	overdue, today, thisWeek, later := m.visibleGroups()

	if len(overdue) > 0 {
		lines = append(lines, m.renderGroupHeader("Overdue", len(overdue), true))
		lines = append(lines, m.renderGroup(overdue, currentIndex, true)...)
		hasContent = true
	}

	if len(today) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("Today", len(today), false))
		lines = append(lines, m.renderGroup(today, currentIndex, false)...)
		hasContent = true
	}

	if len(thisWeek) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("This Week", len(thisWeek), false))
		lines = append(lines, m.renderGroup(thisWeek, currentIndex, false)...)
		hasContent = true
	}

	if len(later) > 0 {
		if hasContent {
			lines = append(lines, separator)
		}
		lines = append(lines, m.renderGroupHeader("Later", len(later), false))
		lines = append(lines, m.renderGroup(later, currentIndex, false)...)
	}

	return lines
}

// renderGroupHeader renders a group header with count.
func (m *DueDateOverviewModel) renderGroupHeader(name string, count int, isOverdue bool) string {
	header := fmt.Sprintf("%s (%d)", name, count)
	if isOverdue {
		return OverdueStyle.Bold(true).Render(header)
	}
	return DueDateGroupHeaderStyle.Render(header)
}

// renderGroup renders a group of tasks.
func (m *DueDateOverviewModel) renderGroup(tasks []DueDateTask, currentIndex *int, isOverdue bool) []string {
	var lines []string
	maxTitleLen := m.width - 35 // Account for date, project, icons

	for _, dt := range tasks {
		isSelected := *currentIndex == m.cursor
		line := m.renderTaskLine(dt, isOverdue, isSelected, maxTitleLen)
		lines = append(lines, line)
		*currentIndex++
	}

	return lines
}

// renderTaskLine renders a single task line.
func (m *DueDateOverviewModel) renderTaskLine(dt DueDateTask, isOverdue bool, isSelected bool, maxTitleLen int) string {
	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Date indicator and formatted date
	dateStr := m.formatDueDate(dt.Task, isOverdue)

	// Project name in brackets
	projectName := util.Truncate(dt.Project.Name, 15)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	// Task type icon with badge style
	typeIcon := GetTypeBadge(string(dt.Task.Type), dt.Task.Type.Icon())

	// Title
	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(dt.Task.Title, maxTitleLen)

	line := fmt.Sprintf("%s%s %s %s %s", cursor, dateStr, projectStr, typeIcon, title)

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// formatDueDate formats the due date for display.
func (m *DueDateOverviewModel) formatDueDate(task *models.Task, isOverdue bool) string {
	if task.DueDate == nil {
		return "         "
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(task.DueDate.Year(), task.DueDate.Month(), task.DueDate.Day(), 0, 0, 0, 0, now.Location())

	var dateStr string
	var style lipgloss.Style

	if isOverdue {
		dateStr = task.DueDate.Format("Jan 2")
		style = OverdueStyle
	} else if dueDay.Equal(today) {
		dateStr = "Today"
		style = DueTodayStyle
	} else if dueDay.Equal(today.AddDate(0, 0, 1)) {
		dateStr = "Tomorrow"
		style = DueDateStyle
	} else if task.DueDate.Year() != now.Year() {
		dateStr = task.DueDate.Format("Jan 2 '06")
		style = DueDateStyle
	} else {
		dateStr = task.DueDate.Format("Jan 2")
		style = DueDateStyle
	}

	// Pad to consistent width
	return style.Render(fmt.Sprintf("%-9s", dateStr))
}

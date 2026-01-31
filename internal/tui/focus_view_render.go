package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func (m *FocusViewModel) View() string {
	if len(m.tasks) == 0 {
		var b strings.Builder
		b.WriteString(MutedStyle.Render("No tasks in focus."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Open a project and mark tasks as active with *."))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press q to return to dashboard."))

		return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
			Title:    "Focus View",
			Subtitle: "no active tasks",
			Centered: true,
		})
	}

	var b strings.Builder

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Calculate available height for task list
	contentHeight := m.height - 13

	// Render task lines
	var lines []string
	maxTitleLen := m.width - 45

	for i, ft := range m.filtered {
		isSelected := i == m.cursor
		line := m.renderTaskLine(ft, isSelected, maxTitleLen)
		lines = append(lines, line)
	}

	if len(m.filtered) == 0 {
		lines = append(lines, MutedStyle.Render("No matching tasks."))
	}

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
	footer := HelpStyle.Render("j/k nav | h/l [/] tabs | * unfocus | ,s/d/b/x status | 1-5 priority | dt/dm due | Enter go to | ? help")
	b.WriteString(footer)

	subtitle := fmt.Sprintf("%d active tasks", len(m.tasks))
	if len(m.filtered) != len(m.tasks) {
		subtitle = fmt.Sprintf("%d/%d active tasks", len(m.filtered), len(m.tasks))
	}

	return renderOverlayCard(b.String(), m.width, m.height, OverlayCardConfig{
		Title:    "Focus View",
		Subtitle: subtitle,
		Centered: true,
	})
}

// renderTabBar renders the tab bar for filtering.
func (m *FocusViewModel) renderTabBar() string {
	return RenderTabBar(focusTabNames, m.activeTab, min(m.width-6, 54)-2, 1)
}

// renderPrefixPopup renders the which-key style popup for prefix key sequences.
func (m *FocusViewModel) renderPrefixPopup() string {
	return RenderPrefixPopup(m.pendingPrefix, PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": FocusViewNavigationPrefixHints,
		"c": CreateLinkedPrefixHints,
	}, m.width)
}

// FocusViewNavigationPrefixHints contains "g" prefix hints for focus view.
var FocusViewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first"},
	{Key: "p", Description: "go to project"},
}

// renderTaskLine renders a single task line.
func (m *FocusViewModel) renderTaskLine(ft FocusTask, isSelected bool, maxTitleLen int) string {
	task := ft.Task

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	projectName := util.Truncate(ft.Project.Name, 12)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	priorityBadge := m.renderPriorityBadge(task.Priority)

	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	tagsStr := ""
	if len(task.Tags) > 0 {
		tags := task.Tags
		if len(tags) > 2 {
			tags = tags[:2]
		}
		tagsStr = MutedStyle.Render(" #" + strings.Join(tags, " #"))
	}

	dueStr := ""
	if task.DueDate != nil {
		dueStr = m.formatRelativeDue(task.DueDate)
	}

	line := fmt.Sprintf("%s%s %s %s %s %s%s", cursor, projectStr, statusIcon, typeIcon, priorityBadge, title, tagsStr)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderPriorityBadge renders a priority indicator.
func (m *FocusViewModel) renderPriorityBadge(priority int) string {
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

// formatRelativeDue formats the due date for display.
func (m *FocusViewModel) formatRelativeDue(dueDate *time.Time) string {
	if dueDate == nil {
		return ""
	}

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	dueDay := time.Date(dueDate.Year(), dueDate.Month(), dueDate.Day(), 0, 0, 0, 0, now.Location())

	daysUntil := int(dueDay.Sub(today).Hours() / 24)

	switch {
	case daysUntil < -1:
		return OverdueStyle.Render(fmt.Sprintf("%d days ago", -daysUntil))
	case daysUntil == -1:
		return OverdueStyle.Render("yesterday")
	case daysUntil == 0:
		return DueTodayStyle.Render("today")
	case daysUntil == 1:
		return DueDateStyle.Render("tomorrow")
	case daysUntil < 7:
		return DueDateStyle.Render(dueDate.Format("Mon"))
	default:
		if dueDate.Year() != now.Year() {
			return DueDateStyle.Render(dueDate.Format("Jan 2 '06"))
		}
		return DueDateStyle.Render(dueDate.Format("Jan 2"))
	}
}

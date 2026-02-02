package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/sglavoie/tt/internal/util"
)

func (m *FocusViewModel) View() string {
	var b strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d active tasks", len(m.tasks))
	if len(m.filtered) != len(m.tasks) {
		subtitle = fmt.Sprintf("%d/%d active tasks", len(m.filtered), len(m.tasks))
	}
	b.WriteString(renderFullscreenTitle("Focus View", subtitle))
	b.WriteString("\n")

	if len(m.tasks) == 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("No tasks in focus."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Open a project and mark tasks as active with *."))
		b.WriteString("\n")
		b.WriteString(HelpStyle.Render("Press q to return to dashboard."))
		return b.String()
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
	}

	// Calculate available height for task list
	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + footer(2) = 6
	contentHeight := m.height - 6
	if subTabBar != "" {
		contentHeight -= 2
	}

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
	left := HelpStyle.Render("j/k nav | [/] tabs | (/) row focus | * unfocus | ,s/d/b/x status | 1-5 priority | ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderTabBar renders the tab bar for filtering.
func (m *FocusViewModel) renderTabBar() string {
	hasSubTabs := len(m.subTabNames) > 1
	if hasSubTabs {
		return renderSubTabBarWithFocus(focusTabNames, m.activeTab, !m.subTabRowFocused, m.width)
	}
	return renderWrappingTabBar(focusTabNames, m.activeTab, m.width)
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *FocusViewModel) renderSubTabBar() string {
	if len(m.subTabNames) <= 1 {
		return ""
	}

	tabs := make([]string, 0, len(m.subTabNames)+1)
	tabs = append(tabs, "All")
	tabs = append(tabs, m.subTabNames...)

	return renderSubTabBarWithFocus(tabs, m.subTab, m.subTabRowFocused, m.width)
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

// renderPriorityBadge delegates to the shared renderPriorityBadge helper.
func (m *FocusViewModel) renderPriorityBadge(priority int) string {
	return renderPriorityBadge(priority)
}

// formatRelativeDue delegates to the shared formatRelativeDue helper.
func (m *FocusViewModel) formatRelativeDue(dueDate *time.Time) string {
	return formatRelativeDue(dueDate)
}

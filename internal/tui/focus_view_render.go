package tui

import (
	"fmt"
	"strings"

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

	// Filter badge
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}

	// Sub-tab bar
	subTabBar := m.renderSubTabBar()
	if subTabBar != "" {
		b.WriteString(subTabBar)
		b.WriteString("\n")
	}

	// Calculate available height for task list
	// Fullscreen overhead: title(1) + newline(1) + tab bar(2) + footer(2) = 6
	contentHeight := m.height - 6
	if filterBadge != "" {
		contentHeight--
	}
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
		if m.HasActiveFilters() {
			lines = append(lines, MutedStyle.Render("No tasks match current filters. Press q or Esc to clear."))
		} else {
			lines = append(lines, MutedStyle.Render("No matching tasks."))
		}
	}

	b.WriteString(strings.Join(applyScroll(lines, m.cursor, contentHeight), "\n"))
	b.WriteString("\n\n")

	b.WriteString(renderStatusMessages(m.statusMessage, m.err))

	// Footer
	left := HelpStyle.Render("j/k nav | [/] tabs | (/) row focus | * unfocus | ,s/d/b/x status | 1-5 priority | ? help")
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", len(m.filtered)))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderTabBar renders the tab bar for filtering.
func (m *FocusViewModel) renderTabBar() string {
	return renderTabBarWithOptionalSubTabs(focusTabNames, m.activeTab, m.subTabNames, m.subTabRowFocused, m.width)
}

// renderSubTabBar renders the dynamic sub-tab row when applicable.
func (m *FocusViewModel) renderSubTabBar() string {
	return renderDynamicSubTabBar(m.subTabNames, m.subTab, m.subTabRowFocused, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *FocusViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"d": DueDatePrefixHints,
		"g": FocusViewNavigationPrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"c": CreateLinkedPrefixHints,
	}
	// Merge filter prefix hints (no DueDate category — view has "Due Soon" tab)
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, false, true) {
		hints[k] = v
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// FocusViewNavigationPrefixHints contains "g" prefix hints for focus view.
var FocusViewNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to project"},
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
	priorityBadge := renderPriorityBadge(task.Priority)

	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

	tagsStr := renderInlineTags(task.Tags)

	dueStr := ""
	if task.DueDate != nil {
		dueStr = formatRelativeDue(task.DueDate, true)
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

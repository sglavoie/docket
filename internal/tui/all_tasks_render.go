package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/scoring"
	"github.com/sglavoie/tt/internal/util"
)

func (m *AllTasksViewModel) View() string {
	var b strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks across %d projects", len(m.allTasks), m.projectCount())
	if m.totalVisibleTaskCount() != len(m.allTasks) {
		subtitle = fmt.Sprintf("%d/%d tasks across %d projects", m.totalVisibleTaskCount(), len(m.allTasks), m.projectCount())
	}
	b.WriteString(renderFullscreenTitle("All Tasks", subtitle))
	b.WriteString("\n")

	if len(m.allTasks) == 0 {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render("No tasks found across projects."))
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("Press q to return to dashboard."))
		return b.String()
	}

	// Tab bar
	b.WriteString(m.renderTabBar())
	b.WriteString("\n")

	// Filter badges
	filterBadge := m.renderActiveFilterIndicator()
	if filterBadge != "" {
		b.WriteString(filterBadge)
		b.WriteString("\n")
	}

	// Grouping indicator
	if m.groupBy != ATGroupByProject {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  Grouped by: %s", m.groupBy)))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Calculate available height for task list dynamically.
	// Count lines already rendered (header area) and reserve space below
	// for: blank line(1) + status/error(1) + footer separator(1) + footer text(1) = 4
	headerLines := strings.Count(b.String(), "\n")
	contentHeight := m.height - headerLines - 4

	if m.totalNavigableItems() == 0 {
		if m.HasActiveFilters() {
			b.WriteString(MutedStyle.Render("No tasks match current filters."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("[/] tabs | Fx clear filters | q/Esc clear filters"))
		} else {
			b.WriteString(MutedStyle.Render("No matching tasks."))
			b.WriteString("\n\n")
			b.WriteString(HelpStyle.Render("[/] h/l tabs | q/Esc close"))
		}
		return b.String()
	}

	// Render groups with task lines
	var lines []string
	maxTitleLen := m.width - 50
	currentIndex := 0
	cursorLineIdx := 0
	separator := renderHorizontalDivider(m.width)

	for i, group := range m.groups {
		if i > 0 {
			lines = append(lines, separator)
		}

		collapsed := m.collapsedGroups[group.Name]
		headerSelected := currentIndex == m.cursor
		if headerSelected {
			cursorLineIdx = len(lines)
		}
		lines = append(lines, m.renderGroupHeader(group.Name, len(group.Tasks), collapsed, headerSelected))
		currentIndex++

		if !collapsed {
			for _, item := range group.Tasks {
				isSelected := currentIndex == m.cursor
				if isSelected {
					cursorLineIdx = len(lines)
				}
				isChecked := m.selectedTasks[item.Task.ID]
				line := m.renderTaskLine(item, isSelected, isChecked, maxTitleLen)
				lines = append(lines, line)
				currentIndex++
			}
		}
	}

	// Adjust persistent scroll offset to keep cursor visible
	if contentHeight > 0 {
		if cursorLineIdx < m.scrollOffset {
			m.scrollOffset = cursorLineIdx
		}
		if cursorLineIdx >= m.scrollOffset+contentHeight {
			m.scrollOffset = cursorLineIdx - contentHeight + 1
		}
		maxOffset := len(lines) - contentHeight
		if maxOffset < 0 {
			maxOffset = 0
		}
		if m.scrollOffset > maxOffset {
			m.scrollOffset = maxOffset
		}
	}

	visibleLines := lines
	if m.scrollOffset > 0 && m.scrollOffset < len(lines) {
		visibleLines = lines[m.scrollOffset:]
	}
	if len(visibleLines) > contentHeight {
		visibleLines = visibleLines[:contentHeight]
	}

	b.WriteString(strings.Join(visibleLines, "\n"))
	b.WriteString("\n\n")

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
	footerLeft := "j/k nav | [/] tabs | ctrl+g group | ,s/d/b/x status | 1-5 sort | v multi | ? help"
	if m.multiSelectMode {
		footerLeft = "MULTI-SELECT: space toggle | a select all | A deselect | ,s/d/b status | D delete | v exit"
	}
	left := HelpStyle.Render(footerLeft)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalVisibleTaskCount()))
	b.WriteString(renderStatusBarFooter(left, right, m.width))

	return b.String()
}

// renderTabBar renders the tab bar for filtering.
func (m *AllTasksViewModel) renderTabBar() string {
	return renderWrappingTabBar(allTasksTabNames, m.activeTab, m.width)
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *AllTasksViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	hints := PrefixHints{
		",": StatusPrefixHints,
		"!": PriorityPrefixHints,
		"d": DueDatePrefixHints,
		"g": AllTasksNavigationPrefixHints,
		"m": SecondaryViewMovePrefixHints,
		"c": CreateLinkedPrefixHints,
		"z": SectionFoldPrefixHints,
	}
	// Merge filter prefix hints
	for k, v := range buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true) {
		hints[k] = v
	}
	// Add project filter hint to the F prefix (before the clear-all entry)
	if fHints, ok := hints["F"]; ok {
		detail := "(all)"
		if len(m.projectFilter) > 0 {
			detail = fmt.Sprintf("%d projects", len(m.projectFilter))
		}
		// Insert before the last entry (clear-all)
		projectHint := PrefixHint{Key: "P", Description: "project", Detail: detail, Active: len(m.projectFilter) > 0}
		last := fHints[len(fHints)-1]
		hints["F"] = append(fHints[:len(fHints)-1], projectHint, last)
	}
	return &PrefixPopupInfo{
		Prefix: m.pendingPrefix,
		Hints:  hints,
	}
}

// AllTasksNavigationPrefixHints contains "g" prefix hints for all tasks view.
var AllTasksNavigationPrefixHints = []PrefixHint{
	{Key: "g", Description: "first", Detail: "Jump to first item"},
	{Key: "p", Description: "go to project", Detail: "Navigate to task in project tree"},
}

// renderGroupHeader renders a group header with task count and collapse indicator.
// Uses inline styling instead of DueDateGroupHeaderStyle to avoid MarginTop(1)
// which would make each entry multi-line and break contentHeight line counting.
func (m *AllTasksViewModel) renderGroupHeader(name string, count int, collapsed bool, isSelected bool) string {
	indicator := "▼"
	if collapsed {
		indicator = "▶"
	}
	cursor := "  "
	if isSelected {
		cursor = "> "
	}
	header := fmt.Sprintf("%s%s %s (%d)", cursor, indicator, name, count)
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(ColorSecondary)
	styled := headerStyle.Render(header)
	if isSelected {
		return SelectedStyle.Render(styled)
	}
	return styled
}

// renderTaskLine renders a single task line.
func (m *AllTasksViewModel) renderTaskLine(item AllTasksItem, isSelected bool, isChecked bool, maxTitleLen int) string {
	task := item.Task

	cursor := "  "
	if isSelected {
		cursor = "> "
	}

	// Multi-select checkbox
	checkbox := ""
	if m.multiSelectMode {
		if isChecked {
			checkbox = "[x] "
		} else {
			checkbox = "[ ] "
		}
	}

	// Project name
	projectName := util.Truncate(item.Project.Name, 15)
	projectStr := MutedStyle.Render(fmt.Sprintf("[%s]", projectName))

	// Task badges
	statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
	typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
	priorityBadge := renderPriorityBadge(task.Priority)

	if maxTitleLen < 10 {
		maxTitleLen = 10
	}
	title := util.Truncate(task.Title, maxTitleLen)

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
	if task.DueDate != nil {
		dueStr = formatRelativeDue(task.DueDate)
	}

	// Score badge
	scoreBadge := ""
	if profile := m.config.GetProjectScoringProfile(item.Project.ID); profile != nil {
		if HasManualScores(task, profile) {
			score := scoring.ComputeScore(task, profile)
			scoreBadge = fmt.Sprintf(" [S:%d]", int(score))
		}
	}

	line := fmt.Sprintf("%s%s%s %s %s %s %s%s%s", cursor, checkbox, projectStr, statusIcon, typeIcon, priorityBadge, title, tagsStr, scoreBadge)
	if dueStr != "" {
		line = fmt.Sprintf("%-*s %s", m.width-15, line, dueStr)
	}

	if isSelected {
		line = SelectedStyle.Render(line)
	}

	return line
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
func (m *AllTasksViewModel) renderActiveFilterIndicator() string {
	common := renderCommonFilterIndicator(&m.filter)
	if len(m.projectFilter) > 0 {
		projBadge := fmt.Sprintf(" [%d projects]", len(m.projectFilter))
		if common == "" {
			return FilterBadgeStyle.Render("Filters:" + projBadge)
		}
		return common + FilterBadgeStyle.Render(projBadge)
	}
	return common
}

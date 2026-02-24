package tui

import (
	"fmt"
	"strings"
)

func (m *AllTasksViewModel) View() string {
	if m.layoutMode == ATLayoutKanban {
		return m.renderKanbanView()
	}

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

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.tabs.RenderBar(m.Width()),
		m.renderActiveFilterIndicator(),
		false,
	)
	b.WriteString(chrome)

	// Grouping indicator
	if m.groupBy != ATGroupByProject {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("  Grouped by: %s", m.groupBy)))
		b.WriteString("\n")
		chromeLines++
	}

	b.WriteString("\n")
	chromeLines++ // trailing blank

	// Calculate available height for task list
	contentHeight := fullscreenContentHeight(m.Height(), chromeLines)

	if m.totalNavigableItems() == 0 {
		b.WriteString(renderFilterAwareEmptyState(m.HasActiveFilters(),
			EmptyStateFiltered, "[/] h/l tabs · q/Esc close",
			"[/] tabs · Fx clear filters · q/Esc clear filters"))
		return b.String()
	}

	// Render groups with task lines
	var lines []string
	maxTitleLen := m.Width() - 50
	currentIndex := 0
	cursorLineIdx := 0
	separator := renderHorizontalDivider(m.Width())

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

	b.WriteString(strings.Join(applyPersistentScroll(lines, cursorLineIdx, contentHeight, &m.scrollOffset), "\n"))

	// Footer
	footerLeft := "j/k nav · [/] tabs · ctrl+k Kanban · ctrl+g group · ,s/d/b/x status · 1-5 sort · v multi"
	if m.multiSelectMode {
		footerLeft = "MULTI-SELECT: space toggle · a select all · A deselect · ,s/d/b status · D delete · v exit"
	}
	left := HelpStyle.Render(footerLeft)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalVisibleTaskCount()))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.Width()))

	return b.String()
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
func (m *AllTasksViewModel) renderGroupHeader(name string, count int, collapsed bool, isSelected bool) string {
	return renderCollapsibleHeader(name, count, collapsed, isSelected, SectionHeaderStyle)
}

// renderTaskLine renders a single task line.
func (m *AllTasksViewModel) renderTaskLine(item CrossProjectTask, isSelected bool, isChecked bool, maxTitleLen int) string {
	task := item.Task

	// Multi-select checkbox
	checkbox := ""
	if m.multiSelectMode {
		if isChecked {
			checkbox = "[x] "
		} else {
			checkbox = "[ ] "
		}
	}

	dueStr := ""
	if task.DueDate != nil {
		dueStr = FormatRelativeDue(task.DueDate, true)
	}

	return RenderCrossProjectTaskLine(task, item.Project, m.config, isSelected, m.Width(), maxTitleLen, CrossProjectTaskLineOpts{
		ShowTags:      true,
		ShowScore:     true,
		MaxProjectLen: 15,
		Prefix:        checkbox,
		DueStr:        dueStr,
	})
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

package tui

import (
	"fmt"
	"maps"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *AllTasksViewModel) View() tea.View {
	var b strings.Builder

	// Title
	subtitle := fmt.Sprintf("%d tasks across %d projects", len(m.allTasks), m.projectCount())
	if m.totalVisibleTaskCount() != len(m.allTasks) {
		subtitle = fmt.Sprintf("%d/%d tasks across %d projects", m.totalVisibleTaskCount(), len(m.allTasks), m.projectCount())
	}
	b.WriteString(renderFullscreenTitle("All Tasks", subtitle, m.AccentColor()))
	b.WriteString("\n")

	if len(m.allTasks) == 0 {
		b.WriteString("\n")
		b.WriteString(renderEmptyState("No tasks found across projects.", "Press q to return to dashboard."))
		return tea.NewView(b.String())
	}

	// Tab bar + filter badge
	chrome, chromeLines := renderFullscreenChrome(
		m.tabs.RenderBar(m.Width(), m.AccentColor()),
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
		return tea.NewView(b.String())
	}

	// Reserve one column for the scrollbar when content overflows.
	lineWidth := m.Width()
	if m.totalNavigableItems() > contentHeight {
		lineWidth--
	}

	// Compute column layout from visible tasks for badge alignment.
	var colLayout *ColumnLayout
	if lineWidth >= MinColumnAlignWidth {
		var visibleTasks []*models.Task
		for _, group := range m.groups {
			if !m.collapsedGroups[group.Name] {
				for _, item := range group.Tasks {
					visibleTasks = append(visibleTasks, item.Task)
				}
			}
		}
		colLayout = ComputeTaskLineColumnLayout(visibleTasks, false)
	}

	// Render groups with task lines
	var lines []string
	maxTitleLen := lineWidth - 50
	currentIndex := 0
	cursorLineIdx := 0
	separator := renderHorizontalDivider(lineWidth)

	for i, group := range m.groups {
		if i > 0 {
			lines = append(lines, separator)
		}

		collapsed := m.collapsedGroups[group.Name]
		headerSelected := currentIndex == m.cursor
		if headerSelected {
			cursorLineIdx = len(lines)
		}
		lines = append(lines, m.renderGroupHeader(group.Name, len(group.Tasks), collapsed, headerSelected, lineWidth))
		currentIndex++

		if !collapsed {
			for _, item := range group.Tasks {
				isSelected := currentIndex == m.cursor
				if isSelected {
					cursorLineIdx = len(lines)
				}
				isChecked := m.selectedTasks[item.Task.ID]
				line := m.renderTaskLine(item, isSelected, isChecked, maxTitleLen, currentIndex, lineWidth, colLayout)
				lines = append(lines, line)
				currentIndex++
			}
		}
	}

	// Insert inline preview lines after cursor task
	if m.previewMode && cursorLineIdx >= 0 && cursorLineIdx < len(lines) {
		task, _ := m.getTaskRef()
		if task != nil {
			indent := "        " // align with cross-project task line content
			pLines := inlinePreviewLines(task, indent, lineWidth)
			if len(pLines) > 0 {
				insertAt := cursorLineIdx + 1
				if insertAt <= len(lines) {
					tail := append([]string{}, lines[insertAt:]...)
					lines = append(lines[:insertAt], pLines...)
					lines = append(lines, tail...)
					contentHeight -= len(pLines)
					if contentHeight < 1 {
						contentHeight = 1
					}
				}
			}
		}
	}

	scrolledLines := applyPersistentScroll(lines, cursorLineIdx, contentHeight, &m.scrollOffset)
	scrollbar := RenderScrollbar(len(lines), contentHeight, m.scrollOffset)
	b.WriteString(AttachScrollbar(strings.Join(scrolledLines, "\n"), scrollbar))

	// Footer
	footerLeft := "j/k nav · [/] tabs · Ctrl+G group · ,s/d/b/x status · 1-5 sort · v multi"
	if m.multiSelectMode {
		footerLeft = "MULTI-SELECT: space toggle · a select all · A deselect · ,s/d/b status · D delete · v exit"
	}
	left := HelpStyle.Render(footerLeft)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalVisibleTaskCount()))
	b.WriteString(renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.Width(), m.AccentColor()))

	return tea.NewView(b.String())
}

// GetPrefixPopupInfo returns prefix popup data for overlay rendering.
func (m *AllTasksViewModel) GetPrefixPopupInfo() *PrefixPopupInfo {
	if m.pendingPrefix == "" {
		return nil
	}
	var currentStage string
	var currentPriority int
	if ct := m.getTaskAtCursor(); ct != nil {
		currentStage = ct.Task.Stage
		currentPriority = ct.Task.Priority
	}
	hints := CommonTaskActionPrefixHints(m.config.Stages, currentStage, currentPriority)
	hints["g"] = AllTasksNavigationPrefixHints
	hints["z"] = append(SectionFoldPrefixHints, PrefixHint{Key: "p", Description: "preview", Detail: "Toggle inline preview"})
	// Merge filter prefix hints
	maps.Copy(hints, buildCommonFilterPrefixPopupHints(&m.filter, m.config, true, true))
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
func (m *AllTasksViewModel) renderGroupHeader(name string, count int, collapsed bool, isSelected bool, width int) string {
	return renderCollapsibleHeader(name, count, collapsed, isSelected, m.cursorFlash, SectionHeaderStyle, width)
}

// renderTaskLine renders a single task line.
func (m *AllTasksViewModel) renderTaskLine(item CrossProjectTask, isSelected bool, isChecked bool, maxTitleLen int, rowIndex int, lineWidth int, layout *ColumnLayout) string {
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

	return RenderCrossProjectTaskLine(task, item.Project, m.config, isSelected, lineWidth, maxTitleLen, CrossProjectTaskLineOpts{
		Queued:        m.queueIDs[task.ID],
		ShowTags:      true,
		ShowScore:     true,
		MaxProjectLen: 15,
		Prefix:        checkbox,
		DueStr:        dueStr,
		Flash:         m.cursorFlash,
		RowIndex:      rowIndex,
		Layout:        layout,
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

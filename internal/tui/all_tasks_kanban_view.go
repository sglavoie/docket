package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/util"
)

// renderKanbanView renders the AllTasks kanban layout: title + tabs + columns + footer.
func (m *AllTasksViewModel) renderKanbanView() string {
	var viewParts []string

	// Title
	subtitle := fmt.Sprintf("%d tasks across %d projects", len(m.allTasks), m.projectCount())
	if m.totalVisibleTaskCount() != len(m.allTasks) {
		subtitle = fmt.Sprintf("%d/%d tasks across %d projects", m.totalVisibleTaskCount(), len(m.allTasks), m.projectCount())
	}
	viewParts = append(viewParts, renderFullscreenTitle("All Tasks", subtitle))

	if len(m.allTasks) == 0 {
		viewParts = append(viewParts, "")
		viewParts = append(viewParts, MutedStyle.Render("No tasks found across projects."))
		viewParts = append(viewParts, "")
		viewParts = append(viewParts, HelpStyle.Render("Press q to return to dashboard."))
		return strings.Join(viewParts, "\n")
	}

	// Tab bar + filter badge
	tabBar := m.tabs.RenderBar(m.Width())
	filterBadge := m.renderActiveFilterIndicator()
	chrome, _ := renderFullscreenChrome(tabBar, filterBadge, false)
	viewParts = append(viewParts, chrome)

	// Grouping indicator
	groupLine := MutedStyle.Render(fmt.Sprintf("  Grouped by: %s  [kanban]", m.groupBy))
	viewParts = append(viewParts, groupLine)

	// Horizontal scroll indicator when columns exceed max visible
	maxVisCols := m.maxVisibleKanbanColumns()
	if len(m.groups) > maxVisCols {
		visStart, visEnd := m.visibleKanbanColumnRange()
		var scrollParts []string
		if visStart > 0 {
			scrollParts = append(scrollParts, fmt.Sprintf("◀ %d", visStart))
		}
		scrollParts = append(scrollParts, fmt.Sprintf("Columns %d–%d / %d", visStart+1, visEnd, len(m.groups)))
		if visEnd < len(m.groups) {
			scrollParts = append(scrollParts, fmt.Sprintf("%d ▶", len(m.groups)-visEnd))
		}
		viewParts = append(viewParts, MutedStyle.Render(strings.Join(scrollParts, "  ")))
	}

	if len(m.groups) == 0 {
		viewParts = append(viewParts, renderFilterAwareEmptyState(m.HasActiveFilters(),
			EmptyStateFiltered, "Ctrl+K list · q/Esc close",
			"Ctrl+K list · Fx clear filters · q/Esc close"))
		return strings.Join(viewParts, "\n")
	}

	// Compute kanban sizing
	m.setKanbanSize()

	// Render visible columns
	visStart, visEnd := m.visibleKanbanColumnRange()
	var columns []string
	for i := visStart; i < visEnd; i++ {
		columns = append(columns, m.renderKanbanColumn(i))
	}

	// Join columns horizontally
	gap := " "
	var parts []string
	for i, col := range columns {
		parts = append(parts, col)
		if i < len(columns)-1 {
			parts = append(parts, gap)
		}
	}
	kanbanPane := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	viewParts = append(viewParts, kanbanPane)

	// Footer
	footerLeft := "h/l cols · j/k nav · Ctrl+K list · Ctrl+G group · [/] tabs"
	if m.multiSelectMode {
		footerLeft = "MULTI-SELECT: space toggle · a select col · A deselect · ,s/d/b status · D delete · v exit"
	}
	left := HelpStyle.Render(footerLeft)
	right := MutedStyle.Render(fmt.Sprintf("%d tasks", m.totalVisibleTaskCount()))
	viewParts = append(viewParts, renderFullscreenFooterBlock(m.statusMessage, m.err, left, right, m.Width()))

	return strings.Join(viewParts, "\n")
}

// renderKanbanColumn renders a single column for the given group index.
func (m *AllTasksViewModel) renderKanbanColumn(groupIdx int) string {
	group := m.groups[groupIdx]
	isActive := groupIdx == m.kanbanActiveCol
	cs := m.kanbanColumns[group.Name]
	if cs == nil {
		cs = &ColumnState{}
		m.kanbanColumns[group.Name] = cs
	}

	var b strings.Builder

	// Header
	header := m.renderKanbanColumnHeader(group.Name, len(group.Tasks), isActive)
	b.WriteString(header)
	b.WriteString("\n")

	// Viewport height for tasks (kanbanHeight minus header line)
	viewportHeight := m.kanbanHeight - 1
	if viewportHeight < 1 {
		viewportHeight = 1
	}

	cs.ValidateScroll(len(group.Tasks), viewportHeight)

	if len(group.Tasks) == 0 {
		b.WriteString(MutedStyle.Render("(empty)"))
	} else {
		startIdx := cs.ScrollOffset
		endIdx := startIdx + viewportHeight
		if endIdx > len(group.Tasks) {
			endIdx = len(group.Tasks)
		}

		var rowBuilder strings.Builder
		for i := startIdx; i < endIdx; i++ {
			isSelected := i == cs.Cursor && isActive
			line := m.renderKanbanTaskRow(group.Tasks[i], isSelected)
			rowBuilder.WriteString(line)
			if i < endIdx-1 {
				rowBuilder.WriteString("\n")
			}
		}

		scrollbar := RenderScrollbar(len(group.Tasks), viewportHeight, startIdx)
		b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))
	}

	// Pad to exact height
	content := b.String()
	lines := strings.Split(content, "\n")
	for len(lines) < m.kanbanHeight {
		lines = append(lines, "")
	}
	if len(lines) > m.kanbanHeight {
		lines = lines[:m.kanbanHeight]
	}

	columnStyle := lipgloss.NewStyle().Width(m.kanbanColWidth)
	return columnStyle.Render(strings.Join(lines, "\n"))
}

// renderKanbanColumnHeader renders a column header with styling based on groupBy mode.
func (m *AllTasksViewModel) renderKanbanColumnHeader(name string, count int, isActive bool) string {
	headerText := fmt.Sprintf("%s (%d)", name, count)
	if isActive {
		headerText = "▸ " + headerText
	}

	// Use group-by-aware styling when possible
	var style lipgloss.Style
	switch m.groupBy {
	case ATGroupByStatus:
		style = GetColumnHeaderStyle(GroupByStatus, name)
	case ATGroupByPriority:
		style = GetColumnHeaderStyle(GroupByPriority, name)
	case ATGroupBySection:
		style = GetColumnHeaderStyle(GroupBySection, name)
	default:
		style = SectionHeaderStyle
	}
	return style.Render(headerText)
}

// renderKanbanTaskRow renders a compact task row for a kanban column.
func (m *AllTasksViewModel) renderKanbanTaskRow(item CrossProjectTask, isSelected bool) string {
	task := item.Task
	statusIcon := task.Status.Icon()
	statusStyle := GetStatusStyle(string(task.Status))

	// Checkbox for multi-select
	var checkbox string
	if m.multiSelectMode {
		if m.selectedTasks[task.ID] {
			checkbox = "[x] "
		} else {
			checkbox = "[ ] "
		}
	}

	// Project badge (truncated to fit narrow columns)
	maxProjLen := 8
	projBadge := MutedStyle.Render(fmt.Sprintf("[%s]", util.Truncate(item.Project.Name, maxProjLen)))

	// Calculate available title width
	// Layout: cursor(3) + checkbox(4?) + status(1) + space(1) + [proj](maxProjLen+2) + space(1) + title
	checkboxLen := 0
	if m.multiSelectMode {
		checkboxLen = 4
	}
	prefixLen := 3 + checkboxLen + 1 + 1 + maxProjLen + 2 + 1
	maxTitleLen := m.kanbanColWidth - prefixLen
	if maxTitleLen < 5 {
		maxTitleLen = 5
	}
	title := util.Truncate(task.Title, maxTitleLen)

	// Apply active styling
	if isSelected {
		if task.Active {
			title = ActiveStyle.Bold(true).Render(title)
		}
	} else if task.Active {
		title = ActiveStyle.Render(title)
	}

	content := checkbox + statusStyle.Render(statusIcon) + " " + projBadge + " " + title
	return RenderCursorPrefix(isSelected, task.Active, task.Pinned) + content
}

// setKanbanSize computes column width and height based on terminal dimensions.
func (m *AllTasksViewModel) setKanbanSize() {
	// Chrome overhead: title(1) + blank(1) + tab bar(1) + grouping line(1) + footer(4 approx)
	chromeHeight := 8
	if len(m.groups) > m.maxVisibleKanbanColumns() {
		chromeHeight++ // horizontal scroll line
	}
	m.kanbanHeight = m.Height() - chromeHeight
	if m.kanbanHeight < 3 {
		m.kanbanHeight = 3
	}

	numColumns := len(m.groups)
	maxVisCols := m.maxVisibleKanbanColumns()
	if numColumns > maxVisCols {
		numColumns = maxVisCols
	}
	if numColumns < 1 {
		numColumns = 1
	}
	numGaps := numColumns - 1
	availableWidth := m.Width() - numGaps
	columnWidth := availableWidth / numColumns
	if columnWidth < KanbanMinColumnWidth {
		columnWidth = KanbanMinColumnWidth
	}
	m.kanbanColWidth = columnWidth
}

// maxVisibleKanbanColumns returns the max columns that fit given terminal width.
func (m *AllTasksViewModel) maxVisibleKanbanColumns() int {
	maxCols := 5 // default max
	if m.Width() > 0 {
		fitCols := m.Width() / KanbanMinColumnWidth
		if fitCols < 1 {
			fitCols = 1
		}
		if maxCols > fitCols {
			maxCols = fitCols
		}
	}
	if maxCols > len(m.groups) {
		maxCols = len(m.groups)
	}
	if maxCols < 1 {
		maxCols = 1
	}
	return maxCols
}

// visibleKanbanColumnRange returns the start (inclusive) and end (exclusive)
// indices of visible columns.
func (m *AllTasksViewModel) visibleKanbanColumnRange() (int, int) {
	total := len(m.groups)
	maxVisible := m.maxVisibleKanbanColumns()
	if total <= maxVisible {
		return 0, total
	}
	start := m.kanbanColScroll
	if start < 0 {
		start = 0
	}
	if start > total-maxVisible {
		start = total - maxVisible
	}
	end := start + maxVisible
	if end > total {
		end = total
	}
	return start, end
}

// ensureActiveKanbanColumnVisible adjusts kanbanColScroll so the active column
// is within the visible range.
func (m *AllTasksViewModel) ensureActiveKanbanColumnVisible() {
	total := len(m.groups)
	maxVisible := m.maxVisibleKanbanColumns()
	if total <= maxVisible {
		m.kanbanColScroll = 0
		return
	}
	if m.kanbanActiveCol < m.kanbanColScroll {
		m.kanbanColScroll = m.kanbanActiveCol
	}
	if m.kanbanActiveCol >= m.kanbanColScroll+maxVisible {
		m.kanbanColScroll = m.kanbanActiveCol - maxVisible + 1
	}
	if m.kanbanColScroll < 0 {
		m.kanbanColScroll = 0
	}
	if m.kanbanColScroll > total-maxVisible {
		m.kanbanColScroll = total - maxVisible
	}
}

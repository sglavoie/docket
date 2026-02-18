package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/util"
)

// renderSwimlaneBoardView renders the kanban board in swimlane (2D grid) mode.
// Called from View() when swimlanes are active.
func (m *KanbanViewModel) renderSwimlaneBoardView() string {
	var viewParts []string

	// Custom section tab bar
	hasCustomSections := m.customSectionTabs() != nil && IsSectionGroupBy(m.groupBy)
	if hasCustomSections {
		unfocusedPad := " "
		csBar := RenderColoredTabBar(m.customSectionTabs(), m.customSectionTab, m.Width()-1, 1, m.Project.SectionColors)
		if m.customSectionRowFocused {
			csBar = FocusIndicator + csBar
		} else {
			csBar = unfocusedPad + csBar
		}
		viewParts = append(viewParts, csBar)
	}

	// Preset tab bar
	presetBar := m.renderKanbanTabBar()
	if hasCustomSections {
		unfocusedPad := " "
		if !m.customSectionRowFocused {
			presetBar = FocusIndicator + presetBar
		} else {
			presetBar = unfocusedPad + presetBar
		}
	}
	viewParts = append(viewParts, presetBar)

	// Swimlane mode indicator
	colLabel := GroupByLabel(m.groupBy)
	laneLabel := GroupByLabel(m.swimlaneBy)
	indicator := MutedStyle.Render("Columns: ") + lipgloss.NewStyle().Foreground(ColorPrimary).Render(colLabel) +
		MutedStyle.Render(" | Swimlanes: ") + lipgloss.NewStyle().Foreground(ColorPrimary).Render(laneLabel)
	viewParts = append(viewParts, indicator)

	// Search input if in search mode
	if m.IsSearchMode() {
		searchLine := "Search: " + m.SearchInputView()
		viewParts = append(viewParts, searchLine)
	}

	// Score filter input
	if m.scoreFilterInput.IsActive() {
		scoreLine := SubtitleStyle.Render("Score threshold: ") + m.scoreFilterInput.View() + " " + HelpStyle.Render("(Enter: apply, Esc: cancel)")
		viewParts = append(viewParts, scoreLine)
	}

	// Horizontal scroll indicator
	visColStart, visColEnd := m.visibleColumnRange()
	if len(m.columnDefs) > m.maxVisibleColumns() {
		var scrollParts []string
		if visColStart > 0 {
			scrollParts = append(scrollParts, fmt.Sprintf("◀ %d", visColStart))
		}
		scrollParts = append(scrollParts, fmt.Sprintf("Columns %d–%d / %d", visColStart+1, visColEnd, len(m.columnDefs)))
		if visColEnd < len(m.columnDefs) {
			scrollParts = append(scrollParts, fmt.Sprintf("%d ▶", len(m.columnDefs)-visColEnd))
		}
		viewParts = append(viewParts, MutedStyle.Render(strings.Join(scrollParts, "  ")))
	}

	// Column headers row
	headerRow := m.renderSwimlaneColumnHeaders(visColStart, visColEnd)
	viewParts = append(viewParts, headerRow)

	// Calculate available height for swimlane rows
	headerLines := len(viewParts) + 1 // +1 for separator after headers
	statusBarHeight := 0
	if m.statusMessage != "" {
		statusBarHeight = 1
	}
	quickAddHeight := 0
	if m.quickAddMode {
		quickAddHeight = m.getQuickAddHeight()
	}
	quickEditHeight := m.getQuickEditHeight()
	searchBarHeight := 0
	if m.IsSearchMode() {
		searchBarHeight = 1
	}
	_ = searchBarHeight // already counted in viewParts

	availableHeight := m.Height() - headerLines - statusBarHeight - quickAddHeight - quickEditHeight
	if availableHeight < 4 {
		availableHeight = 4
	}

	// Vertical swimlane scroll indicator
	laneStart, laneEnd := m.visibleSwimlaneRange()
	if len(m.swimlaneDefs) > m.maxVisibleSwimlanes() {
		// Reserve 1 line for scroll indicator at the bottom
		availableHeight--
	}

	// Distribute height evenly across visible swimlane rows
	visibleLaneCount := laneEnd - laneStart
	if visibleLaneCount < 1 {
		visibleLaneCount = 1
	}
	rowHeight := availableHeight / visibleLaneCount
	if rowHeight < 2 {
		rowHeight = 2 // Minimum: 1 header + 1 task line
	}

	// Render each visible swimlane row
	for li := laneStart; li < laneEnd; li++ {
		lane := m.swimlaneDefs[li]
		isActiveLane := li == m.activeSwimlane
		rowContent := m.renderSwimlaneRow(lane.Key, li, isActiveLane, visColStart, visColEnd, rowHeight)
		viewParts = append(viewParts, rowContent)
	}

	// Vertical swimlane scroll indicator
	if len(m.swimlaneDefs) > m.maxVisibleSwimlanes() {
		var scrollParts []string
		if laneStart > 0 {
			scrollParts = append(scrollParts, fmt.Sprintf("▲ %d", laneStart))
		}
		scrollParts = append(scrollParts, fmt.Sprintf("Rows %d–%d / %d", laneStart+1, laneEnd, len(m.swimlaneDefs)))
		if laneEnd < len(m.swimlaneDefs) {
			scrollParts = append(scrollParts, fmt.Sprintf("▼ %d", len(m.swimlaneDefs)-laneEnd))
		}
		viewParts = append(viewParts, MutedStyle.Render(strings.Join(scrollParts, "  ")))
	}

	// Quick add input
	if m.quickAddMode {
		viewParts = append(viewParts, m.renderQuickAddInput())
	}

	// Quick edit bar
	if m.quickEdit != nil && m.quickEdit.IsActive() {
		viewParts = append(viewParts, m.renderQuickEditBar())
	}

	// Status message
	if m.statusMessage != "" {
		statusStyle := lipgloss.NewStyle().Foreground(ColorWarning)
		if m.confirm.IsActive() {
			statusStyle = lipgloss.NewStyle().Foreground(ColorError).Bold(true)
		}
		viewParts = append(viewParts, statusStyle.Render(m.statusMessage))
	}

	return lipgloss.JoinVertical(lipgloss.Left, viewParts...)
}

// renderSwimlaneColumnHeaders renders the column header row for swimlane mode.
func (m *KanbanViewModel) renderSwimlaneColumnHeaders(visColStart, visColEnd int) string {
	gap := " "
	var headers []string

	for i := visColStart; i < visColEnd; i++ {
		def := m.columnDefs[i]
		style := GetColumnHeaderStyle(m.groupBy, def.Key)
		label := def.Label

		// Count total tasks in this column across all lanes
		totalCount := 0
		for _, lane := range m.swimlaneDefs {
			totalCount += m.swimlaneCellNodeCount(lane.Key, def.Key)
		}

		headerText := fmt.Sprintf("%s (%d)", label, totalCount)
		if i == m.ActiveColumn {
			headerText = "▸ " + headerText
		}

		rendered := style.Render(headerText)
		// Apply column width
		rendered = lipgloss.NewStyle().Width(m.ColumnWidth).Render(rendered)
		headers = append(headers, rendered)
	}

	var parts []string
	for i, h := range headers {
		parts = append(parts, h)
		if i < len(headers)-1 {
			parts = append(parts, gap)
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, parts...)
}

// renderSwimlaneRow renders a single swimlane row (header + task cells).
func (m *KanbanViewModel) renderSwimlaneRow(laneKey string, laneIdx int, isActiveLane bool, visColStart, visColEnd, rowHeight int) string {
	gap := " "

	// Row header (1 line): marker + lane label + task count
	laneLabel := m.swimlaneLabel(laneKey)
	laneCount := m.swimlaneLaneTaskCount(laneKey)
	marker := "─"
	if isActiveLane {
		marker = "▸"
	}

	laneStyle := GetColumnHeaderStyle(m.swimlaneBy, laneKey)
	headerLine := laneStyle.Render(fmt.Sprintf("%s %s (%d)", marker, laneLabel, laneCount))

	// Pad header to full width with dotted line
	headerWidth := lipgloss.Width(headerLine)
	remaining := m.Width() - headerWidth
	if remaining > 0 {
		dots := MutedStyle.Render(strings.Repeat("·", remaining))
		headerLine = headerLine + dots
	}

	// Task viewport height per cell (row minus header)
	cellHeight := rowHeight - 1
	if cellHeight < 1 {
		cellHeight = 1
	}

	// Render each visible column cell
	var cells []string
	for i := visColStart; i < visColEnd; i++ {
		colDef := m.columnDefs[i]
		isActiveCell := isActiveLane && i == m.ActiveColumn
		cellContent := m.renderSwimlaneCell(laneKey, colDef.Key, isActiveCell, cellHeight)
		cells = append(cells, cellContent)
	}

	var cellParts []string
	for i, c := range cells {
		cellParts = append(cellParts, c)
		if i < len(cells)-1 {
			// Render vertical gap for the cell height
			gapLines := make([]string, cellHeight)
			for j := range gapLines {
				gapLines[j] = gap
			}
			cellParts = append(cellParts, strings.Join(gapLines, "\n"))
		}
	}

	cellRow := lipgloss.JoinHorizontal(lipgloss.Top, cellParts...)
	return lipgloss.JoinVertical(lipgloss.Left, headerLine, cellRow)
}

// renderSwimlaneCell renders a single cell in the swimlane grid.
func (m *KanbanViewModel) renderSwimlaneCell(laneKey, colKey string, isActive bool, height int) string {
	nodes := m.swimlaneNodes[laneKey][colKey]
	cs := m.swimlaneCells[laneKey][colKey]

	var b strings.Builder

	if len(nodes) == 0 {
		b.WriteString(MutedStyle.Render("·"))
	} else {
		// Always validate with reduced height to ensure cursor remains visible when
		// the "↑ N more" indicator appears. The indicator takes 1 line, so we
		// conservatively assume it will be shown during validation.
		reducedHeight := height - 1
		if reducedHeight < 1 {
			reducedHeight = 1
		}
		cs.ValidateScroll(len(nodes), reducedHeight)

		// Determine if indicator is actually needed after validation
		hasUpIndicator := cs.ScrollOffset > 0
		effectiveHeight := height
		if hasUpIndicator {
			effectiveHeight = reducedHeight
		}

		startIdx := cs.ScrollOffset
		endIdx := startIdx + effectiveHeight
		if endIdx > len(nodes) {
			endIdx = len(nodes)
		}

		if hasUpIndicator {
			b.WriteString(ScrollUpIndicator(cs.ScrollOffset))
			b.WriteString("\n")
		}

		for i := startIdx; i < endIdx; i++ {
			node := nodes[i]
			isSelected := i == cs.Cursor && isActive
			line := m.renderSwimlaneCellTask(node, isSelected, isActive)
			b.WriteString(line)
			if i < endIdx-1 {
				b.WriteString("\n")
			}
		}

		// "↓ N more" indicator
		if endIdx < len(nodes) {
			b.WriteString("\n")
			b.WriteString(ScrollDownIndicator(len(nodes) - endIdx))
		}
	}

	// Pad to exact height
	content := b.String()
	lines := strings.Split(content, "\n")
	for len(lines) < height {
		lines = append(lines, "")
	}
	if len(lines) > height {
		lines = lines[:height]
	}

	// Active cell gets a subtle border indication
	style := lipgloss.NewStyle().Width(m.ColumnWidth)
	if isActive {
		style = style.BorderLeft(true).
			BorderStyle(lipgloss.Border{Left: "│"}).
			BorderForeground(ColorPrimary).
			Width(m.ColumnWidth - 1) // Account for border width
	}

	return style.Render(strings.Join(lines, "\n"))
}

// renderSwimlaneCellTask renders a single task within a swimlane cell.
// Simplified version of renderTaskRow: no hierarchy, no expand/collapse.
func (m *KanbanViewModel) renderSwimlaneCellTask(node *KanbanNode, isSelected, isActiveCell bool) string {
	task := node.Task
	statusIcon := task.Status.Icon()
	title := task.Title

	// Multi-select checkbox
	var checkbox string
	if m.multiSelectMode {
		if m.isTaskSelected(task.ID) {
			checkbox = SelectionStyle.Render("[x]") + " "
		} else {
			checkbox = MutedStyle.Render("[ ]") + " "
		}
	}

	// Calculate available space for title
	checkboxLen := 0
	if m.multiSelectMode {
		checkboxLen = KanbanCheckboxWidth
	}
	// Layout: cursor(3) + checkbox + status(1) + space(1) + title
	prefixLen := KanbanCursorWidth + checkboxLen + 1 + 1
	maxTitleLen := m.ColumnWidth - prefixLen
	if isActiveCell {
		// Active cell border takes 1 char
		maxTitleLen--
	}
	if maxTitleLen < KanbanMinTitleWidth {
		maxTitleLen = KanbanMinTitleWidth
	}
	title = util.Truncate(title, maxTitleLen)

	// Apply styling
	if isSelected {
		if task.Active {
			title = ActiveStyle.Bold(true).Render(title)
		}
	} else if task.Active {
		title = ActiveStyle.Render(title)
	}

	statusStyle := GetStatusStyle(string(task.Status))
	content := checkbox + statusStyle.Render(statusIcon) + " " + title

	if isSelected {
		return SelectionCursorStyle.Render(">") + "  " + content
	}
	return "   " + content
}

// swimlaneLabel returns the display label for a swimlane key.
func (m *KanbanViewModel) swimlaneLabel(key string) string {
	for _, def := range m.swimlaneDefs {
		if def.Key == key {
			return def.Label
		}
	}
	return sectionDisplayName(key)
}

// selectAllInSwimlaneCell selects all visible tasks in the active cell.
func (m *KanbanViewModel) selectAllInSwimlaneCell() {
	laneKey := m.activeLaneKey()
	colKey := m.activeColumnKey()
	if nodes, ok := m.swimlaneNodes[laneKey]; ok {
		if cellNodes, ok := nodes[colKey]; ok {
			for _, node := range cellNodes {
				m.selectedTasks[node.Task.ID] = true
			}
		}
	}
	m.updateSelectionStatus()
}

// selectAllAcrossSwimlaneCells selects all visible tasks across all cells.
func (m *KanbanViewModel) selectAllAcrossSwimlaneCells() {
	for _, lane := range m.swimlaneDefs {
		for _, col := range m.columnDefs {
			if nodes, ok := m.swimlaneNodes[lane.Key]; ok {
				if cellNodes, ok := nodes[col.Key]; ok {
					for _, node := range cellNodes {
						m.selectedTasks[node.Task.ID] = true
					}
				}
			}
		}
	}
	m.updateSelectionStatus()
}

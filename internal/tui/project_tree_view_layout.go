package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderFullWidth renders the traditional full-width tree view (sidebar disabled).
func (m *ProjectTreeModel) renderFullWidth() string { //nolint:gocyclo
	var b strings.Builder

	// Title with inline filter/sort
	titleParams := &TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     m.sort.Field,
		SortAscending: m.sort.Ascending,
		Width:         m.width,
	}
	b.WriteString(renderProjectTitle(m.project, m.config, titleParams))
	b.WriteString("\n")

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, m.width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	// Filter bar (view mode tabs only)
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
		if m.ac.IsActive() {
			b.WriteString(m.ac.RenderDropdown("  "))
		}
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.SearchQuery())))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	}

	// Score filter custom value input
	if m.scoreFilterInput.IsActive() {
		b.WriteString(SubtitleStyle.Render("Score threshold: "))
		b.WriteString(m.scoreFilterInput.View())
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Enter: apply, Esc: cancel)"))
		b.WriteString("\n")
	}

	// Focus mode indicator
	if m.focusedTask != nil {
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, truncateToVisualWidth(m.focusedTask.Title, FocusTitleMaxWidth))
		b.WriteString(FocusModeStyle.Render(focusIndicator))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc to exit)"))
		b.WriteString("\n")
	}

	// Multi-select mode indicator
	if m.multiSelectMode {
		b.WriteString(m.renderMultiSelectIndicator(m.width))
	}

	// Stats by status
	statusCounts := make(map[models.TaskStatus]int)
	for _, task := range m.project.Tasks {
		statusCounts[task.Status]++
	}

	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	if storage.NotesExist(m.project) {
		b.WriteString(MutedStyle.Render("[Notes]"))
		b.WriteString(" ")
	}
	b.WriteString(MutedStyle.Render("Tasks: "))
	b.WriteString(strings.Join(statusStats, " "))
	b.WriteString("\n")

	visible := m.visibleNodes()

	// Build card content (task list + scrollbar)
	var cardContent strings.Builder

	cardContentWidth := m.width - CardBorderOverhead

	// Calculate search help height (needed even when no tasks match)
	headerOffset := TreeHeaderOffset
	if m.customSectionTabs() != nil {
		headerOffset++
	}
	// Account for autocomplete dropdown in header
	if m.ac.IsActive() {
		acLines := min(len(m.ac.Items), acVisibleCount)
		if m.ac.ViewportTop > 0 {
			acLines++
		}
		if m.ac.ViewportTop+acVisibleCount < len(m.ac.Items) {
			acLines++
		}
		headerOffset += acLines
	}
	var searchHelpHeight int
	if m.IsSearchHelpVisible() {
		totalAvail := m.height - headerOffset - CardVerticalBorderOverhead
		searchHelpHeight = max(totalAvail/2, 1)
	}

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(renderEmptyState(EmptyStateNoTasks, "Press 'a' to add a task"))
		} else {
			cardContent.WriteString(renderEmptyState(EmptyStateFiltered, "Press Esc to clear filters"))
		}
	} else {
		// Calculate available height for task list
		// Card border consumes 2 lines (top + bottom)
		availableHeight := m.height - headerOffset - CardVerticalBorderOverhead

		// Reserve space for search help pane (takes half the content area)
		if searchHelpHeight > 0 {
			availableHeight -= searchHelpHeight
			if availableHeight < 1 {
				availableHeight = 1
			}
		}

		// Reserve space for inline preview lines when preview mode is on
		var previewLines int
		if m.previewMode && m.cursor >= 0 && m.cursor < len(visible) {
			previewLines = inlinePreviewLineCount(visible[m.cursor].Task)
			availableHeight -= previewLines
			if availableHeight < 1 {
				availableHeight = 1
			}
		}

		// Reserve 1 char for scrollbar when content overflows
		needsScrollbar := len(visible) > availableHeight
		if needsScrollbar {
			cardContentWidth--
		}

		// Determine visible range
		startIdx := 0
		endIdx := len(visible)

		if len(visible) > availableHeight {
			// Center cursor in view
			halfView := availableHeight / 2
			startIdx = max(m.cursor-halfView, 0)
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = max(endIdx-availableHeight, 0)
			}
		}

		// Compute column layout for the visible slice (alignment guard: >= MinColumnAlignWidth)
		if cardContentWidth >= MinColumnAlignWidth {
			m.colLayout = ComputeTreeColumnLayout(visible[startIdx:endIdx], m.columnConfig)
		} else {
			m.colLayout = nil
		}

		// Render visible tasks
		var lines []string
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			inSelection := m.selectedTasks[node.Task.ID]
			lines = append(lines, m.renderNodeWithWidth(node, i == m.cursor, inSelection, cardContentWidth))
			// Append inline preview lines after the cursor task
			if m.previewMode && i == m.cursor {
				indent := strings.Repeat(" ", (node.Level+1)*2)
				lines = append(lines, inlinePreviewLines(node.Task, indent, cardContentWidth)...)
			}
		}
		content := strings.Join(lines, "\n")

		// Attach scrollbar when content overflows
		if needsScrollbar {
			scrollbar := RenderScrollbar(len(visible), availableHeight, startIdx)
			content = AttachScrollbar(content, scrollbar)
		}
		cardContent.WriteString(content)
	}

	if sn := viewModeToSection(m.viewMode); sn != "" {
		count := sectionTaskCount(m.project.Tasks, sn)
		limit := m.config.GetSectionLimit(sn)
		if limit > 0 && count > limit {
			cardContent.WriteString("\n")
			cardContent.WriteString(PreviewWarningStyle.Render(sectionLimitWarning(count-limit, limit)))
		}
	}

	// Render the card with view mode as title
	// Full-width mode has no sidebar, so always focused
	// Use section color accent if exactly one section is filtered
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), m.width, true, sectionAccent))
	b.WriteString("\n")

	// Render search help pane below the card
	if m.IsSearchHelpVisible() {
		b.WriteString(renderSearchHelpPane(m.width, searchHelpHeight))
		b.WriteString("\n")
	}

	return b.String()
}

// renderMultiSelectIndicator renders the multi-select mode indicator with adaptive formatting.
// It adjusts the layout based on available width: full help on same line when wide enough,
// wrapped to second line for medium widths, and abbreviated for narrow displays.
func (m *ProjectTreeModel) renderMultiSelectIndicator(width int) string {
	count := len(m.selectedTasks)
	badge := SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count))
	badgeWidth := lipgloss.Width(badge)

	fullHelp := "Space: toggle · a: all · gX: deselect · v/Esc: exit · e: bulk edit · ,s/d/b/x/o: status · D: delete · T: tag"
	shortHelp := "Spc:sel a:all gX:desel v:exit e:edit ,s/d/b/x/o:status D:del T:tag"

	// Wide terminal: badge and full help on same line
	if width >= badgeWidth+1+len(fullHelp) {
		return badge + " " + MutedStyle.Render(fullHelp) + "\n"
	}

	// Medium width: badge on first line, full help on second line
	if width >= len(fullHelp) {
		return badge + "\n" + MutedStyle.Render(fullHelp) + "\n"
	}

	// Narrow width: badge on first line, short help on second line
	if width >= len(shortHelp) {
		return badge + "\n" + MutedStyle.Render(shortHelp) + "\n"
	}

	// Very narrow: just badge
	return badge + "\n"
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *ProjectTreeModel) renderWithSidebar() string {
	// Calculate widths: 55% tree, 1 for divider, remainder for sidebar
	treeWidth := max(
		// Ensure minimum width for tree
		(m.width*55)/100, TreeMinWidth)

	// Account for divider (1 char)
	dividerWidth := 1

	// Sidebar takes remaining space after tree and divider
	sidebarWidth := m.width - treeWidth - dividerWidth

	// Ensure minimum width for sidebar
	if sidebarWidth < SidebarMinWidth {
		sidebarWidth = SidebarMinWidth
		// If sidebar needs more space, reduce tree width
		treeWidth = m.width - sidebarWidth - dividerWidth
	}

	// Render tree content (reuse most of renderFullWidth logic)
	treeContent := m.renderTreeContent(treeWidth)

	// Get currently selected task
	selectedTask := m.SidebarSelectedTask()

	// Render sidebar
	// Note: SidebarStyle has border (2 chars) + padding (2 chars) = 4 chars overhead
	// lipgloss .Width() sets CONTENT width, so subtract overhead to get desired total width
	// Add 1 to account for rounding to ensure we use full width
	sidebarContent := renderSidebarContent(m, selectedTask, sidebarWidth-SidebarContentOverhead, m.height-CardVerticalBorderOverhead)

	// Apply width constraint to tree to ensure it fills its allocated space
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)

	// Render vertical divider
	divider := renderVerticalDivider(m.height-2, m.sidebar.IsFocused())

	// Dim the unfocused pane to reinforce which pane has focus
	if m.sidebar.IsFocused() {
		treeContentStyled = DimStyle.Render(treeContentStyled)
	} else {
		sidebarContent = DimStyle.Render(sidebarContent)
	}

	// Join horizontally: tree | divider | sidebar
	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, divider, sidebarContent)
}

// renderTreeContent renders just the tree portion (for split-pane layout).
func (m *ProjectTreeModel) renderTreeContent(width int) string { //nolint:gocyclo
	var b strings.Builder

	// Title with inline filter/sort
	titleParams := &TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     m.sort.Field,
		SortAscending: m.sort.Ascending,
		Width:         width,
	}
	b.WriteString(renderProjectTitle(m.project, m.config, titleParams))
	b.WriteString("\n")

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	// Filter bar (view mode tabs only)
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
		if m.ac.IsActive() {
			b.WriteString(m.ac.RenderDropdown("  "))
		}
	} else if m.SearchQuery() != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Search: \"%s\" ", m.SearchQuery())))
		b.WriteString(HelpStyle.Render("(/ to edit, Esc to clear)"))
		b.WriteString("\n")
	}

	// Score filter custom value input
	if m.scoreFilterInput.IsActive() {
		b.WriteString(SubtitleStyle.Render("Score threshold: "))
		b.WriteString(m.scoreFilterInput.View())
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Enter: apply, Esc: cancel)"))
		b.WriteString("\n")
	}

	// Focus mode indicator
	if m.focusedTask != nil {
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, truncateToVisualWidth(m.focusedTask.Title, FocusTitleMaxWidth))
		b.WriteString(FocusModeStyle.Render(focusIndicator))
		b.WriteString(" ")
		b.WriteString(HelpStyle.Render("(Esc to exit)"))
		b.WriteString("\n")
	}

	// Multi-select mode indicator
	if m.multiSelectMode {
		b.WriteString(m.renderMultiSelectIndicator(width))
	}

	// Stats by status (compact for narrower width)
	statusCounts := make(map[models.TaskStatus]int)
	for _, task := range m.project.Tasks {
		statusCounts[task.Status]++
	}

	var statusStats []string
	for _, s := range models.AllTaskStatuses() {
		if count := statusCounts[s]; count > 0 {
			style := GetStatusStyle(string(s))
			statusStats = append(statusStats, style.Render(fmt.Sprintf("%s:%d", s.Icon(), count)))
		}
	}

	if storage.NotesExist(m.project) {
		b.WriteString(MutedStyle.Render("[Notes]"))
		b.WriteString(" ")
	}
	b.WriteString(MutedStyle.Render("Tasks: "))
	b.WriteString(strings.Join(statusStats, " "))
	b.WriteString("\n")

	visible := m.visibleNodes()

	// Build card content (task list + scrollbar)
	var cardContent strings.Builder

	// Calculate search help height (needed even when no tasks match)
	sbHeaderOffset := TreeHeaderOffset
	if m.customSectionTabs() != nil {
		sbHeaderOffset++
	}
	// Account for autocomplete dropdown in header
	if m.ac.IsActive() {
		acLines := min(len(m.ac.Items), acVisibleCount)
		if m.ac.ViewportTop > 0 {
			acLines++
		}
		if m.ac.ViewportTop+acVisibleCount < len(m.ac.Items) {
			acLines++
		}
		sbHeaderOffset += acLines
	}
	var sidebarSearchHelpHeight int
	if m.IsSearchHelpVisible() {
		totalAvail := m.height - sbHeaderOffset - CardVerticalBorderOverhead
		sidebarSearchHelpHeight = max(totalAvail/2, 1)
	}

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(renderEmptyState(EmptyStateNoTasks, "Press 'a' to add a task"))
		} else {
			cardContent.WriteString(renderEmptyState(EmptyStateFiltered, "Press Esc to clear filters"))
		}
	} else {
		// Calculate available height for task list
		// Card border consumes 2 lines (top + bottom)
		availableHeight := m.height - sbHeaderOffset - CardVerticalBorderOverhead

		// Reserve space for search help pane (takes half the content area)
		if sidebarSearchHelpHeight > 0 {
			availableHeight -= sidebarSearchHelpHeight
			if availableHeight < 1 {
				availableHeight = 1
			}
		}

		// Reserve space for inline preview lines when preview mode is on
		var previewLines int
		if m.previewMode && m.cursor >= 0 && m.cursor < len(visible) {
			previewLines = inlinePreviewLineCount(visible[m.cursor].Task)
			availableHeight -= previewLines
			if availableHeight < 1 {
				availableHeight = 1
			}
		}

		// Card has 4 chars horizontal overhead (2 border + 2 padding)
		cardContentWidth := width - 4

		// Reserve 1 char for scrollbar when content overflows
		needsScrollbar := len(visible) > availableHeight
		if needsScrollbar {
			cardContentWidth--
		}

		// Determine visible range
		startIdx := 0
		endIdx := len(visible)

		if len(visible) > availableHeight {
			// Center cursor in view
			halfView := availableHeight / 2
			startIdx = max(m.cursor-halfView, 0)
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = max(endIdx-availableHeight, 0)
			}
		}

		// Compute column layout for the visible slice (alignment guard: >= MinColumnAlignWidth)
		if cardContentWidth >= MinColumnAlignWidth {
			m.colLayout = ComputeTreeColumnLayout(visible[startIdx:endIdx], m.columnConfig)
		} else {
			m.colLayout = nil
		}

		// Render visible tasks (with adjusted width for card content)
		var lines []string
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			inSelection := m.selectedTasks[node.Task.ID]
			lines = append(lines, m.renderNodeWithWidth(node, i == m.cursor, inSelection, cardContentWidth))
			// Append inline preview lines after the cursor task
			if m.previewMode && i == m.cursor {
				indent := strings.Repeat(" ", (node.Level+1)*2)
				lines = append(lines, inlinePreviewLines(node.Task, indent, cardContentWidth)...)
			}
		}
		content := strings.Join(lines, "\n")

		// Attach scrollbar when content overflows
		if needsScrollbar {
			scrollbar := RenderScrollbar(len(visible), availableHeight, startIdx)
			content = AttachScrollbar(content, scrollbar)
		}
		cardContent.WriteString(content)
	}

	if sn := viewModeToSection(m.viewMode); sn != "" {
		count := sectionTaskCount(m.project.Tasks, sn)
		limit := m.config.GetSectionLimit(sn)
		if limit > 0 && count > limit {
			cardContent.WriteString("\n")
			cardContent.WriteString(PreviewWarningStyle.Render(sectionLimitWarning(count-limit, limit)))
		}
	}

	// Render the card with view mode as title
	// Main tree is focused when sidebar is not focused
	// Use section color accent if exactly one section is filtered
	focused := !m.sidebar.IsFocused()
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), width, focused, sectionAccent))
	b.WriteString("\n")

	// Render search help pane below the card
	if m.IsSearchHelpVisible() {
		b.WriteString(renderSearchHelpPane(width, sidebarSearchHelpHeight))
		b.WriteString("\n")
	}

	return b.String()
}

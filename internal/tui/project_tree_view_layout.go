package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/util"
)

// renderFullWidth renders the traditional full-width tree view (sidebar disabled).
func (m *ProjectTreeModel) renderFullWidth() string {
	var b strings.Builder

	// Title
	b.WriteString(renderProjectTitle(m.project, m.config))
	b.WriteString("\n")

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, m.width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	// Filter bar
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
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
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, util.Truncate(m.focusedTask.Title, FocusTitleMaxWidth))
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

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(MutedStyle.Render("No tasks yet. Add one with: tt task add <title>"))
		} else {
			cardContent.WriteString(MutedStyle.Render(EmptyStateFiltered))
		}
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		// Card border consumes 2 lines (top + bottom)
		headerOffset := TreeHeaderOffset
		if m.hasActiveFilters() {
			headerOffset = TreeHeaderOffsetWithFilterBar
		}
		if m.customSectionTabs() != nil {
			headerOffset++
		}
		availableHeight := m.height - headerOffset - CardVerticalBorderOverhead

		// Reserve space for inline preview lines when preview mode is on
		previewLines := 0
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
			startIdx = m.cursor - halfView
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = endIdx - availableHeight
				if startIdx < 0 {
					startIdx = 0
				}
			}
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
				for _, pl := range inlinePreviewLines(node.Task, indent, cardContentWidth) {
					lines = append(lines, pl)
				}
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

	// Render the card with view mode as title
	// Full-width mode has no sidebar, so always focused
	// Use section color accent if exactly one section is filtered
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), m.width, true, sectionAccent))
	b.WriteString("\n")

	// Filter and sort status - outside the card
	var filters []string
	if m.filter.Type != "" {
		filters = append(filters, fmt.Sprintf("type: %s", m.filter.Type))
	}
	if m.filter.Status != "" {
		filters = append(filters, fmt.Sprintf("status: %s", m.filter.Status))
	}
	if m.filter.Priority == 1 {
		filters = append(filters, "priority: high")
	}
	if len(m.filter.Tags) > 0 {
		tags := make([]string, 0, len(m.filter.Tags))
		for tag := range m.filter.Tags {
			tags = append(tags, tag)
		}
		sort.Strings(tags)
		filters = append(filters, fmt.Sprintf("tags: %s", strings.Join(tags, ", ")))
	}
	if len(m.filter.Sections) > 0 {
		sections := make([]string, 0, len(m.filter.Sections))
		for section := range m.filter.Sections {
			sections = append(sections, section)
		}
		sort.Strings(sections)
		filters = append(filters, fmt.Sprintf("sections: %s", strings.Join(sections, ", ")))
	}
	if m.filter.DueDate != DueDateAll {
		var dueName string
		switch m.filter.DueDate {
		case DueDateOverdue:
			dueName = "overdue"
		case DueDateToday:
			dueName = "today"
		case DueDateThisWeek:
			dueName = "this week"
		case DueDateNone:
			dueName = "no due date"
		}
		filters = append(filters, fmt.Sprintf("due: %s", dueName))
	}
	if m.SearchQuery() != "" {
		filters = append(filters, fmt.Sprintf("search: \"%s\"", m.SearchQuery()))
	}

	// Build status line
	var statusParts []string
	if len(filters) > 0 {
		// Use prominent badge style when filters are active
		filterBadge := FilterBadgeStyle.Render(fmt.Sprintf("[%d filters]", len(filters)))
		filterText := strings.Join(filters, ", ")
		statusParts = append(statusParts, filterBadge+" "+filterText)
	}
	if m.sort.Field != SortByDefault {
		direction := "↑"
		if !m.sort.Ascending {
			direction = "↓"
		}
		statusParts = append(statusParts, fmt.Sprintf("Sort: %s %s", m.sort.Field, direction))
	}
	if len(statusParts) > 0 {
		b.WriteString(strings.Join(statusParts, " | "))
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
	treeWidth := (m.width * 55) / 100

	// Ensure minimum width for tree
	if treeWidth < TreeMinWidth {
		treeWidth = TreeMinWidth
	}

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
func (m *ProjectTreeModel) renderTreeContent(width int) string {
	var b strings.Builder

	// Title
	b.WriteString(renderProjectTitle(m.project, m.config))
	b.WriteString("\n")

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	// Filter bar
	b.WriteString(m.renderFilterBar())

	// Search bar
	if m.IsSearchMode() {
		b.WriteString(SubtitleStyle.Render("Search: "))
		b.WriteString(m.SearchInputView())
		b.WriteString("\n")
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
		focusIndicator := fmt.Sprintf("[Focus: #%d %s]", m.focusedTask.ShortID, util.Truncate(m.focusedTask.Title, FocusTitleMaxWidth))
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

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(MutedStyle.Render("No tasks yet."))
		} else {
			cardContent.WriteString(MutedStyle.Render(EmptyStateFiltered))
		}
	} else {
		// Calculate available height for task list
		// Extra line needed when filter bar is enabled (filters on own line)
		// Card border consumes 2 lines (top + bottom)
		headerOffset := TreeHeaderOffset
		if m.hasActiveFilters() {
			headerOffset = TreeHeaderOffsetWithFilterBar
		}
		if m.customSectionTabs() != nil {
			headerOffset++
		}
		availableHeight := m.height - headerOffset - CardVerticalBorderOverhead

		// Reserve space for inline preview lines when preview mode is on
		previewLines := 0
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
			startIdx = m.cursor - halfView
			if startIdx < 0 {
				startIdx = 0
			}
			endIdx = startIdx + availableHeight
			if endIdx > len(visible) {
				endIdx = len(visible)
				startIdx = endIdx - availableHeight
				if startIdx < 0 {
					startIdx = 0
				}
			}
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
				for _, pl := range inlinePreviewLines(node.Task, indent, cardContentWidth) {
					lines = append(lines, pl)
				}
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

	// Render the card with view mode as title
	// Main tree is focused when sidebar is not focused
	// Use section color accent if exactly one section is filtered
	focused := !m.sidebar.IsFocused()
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCard(m.getViewModeTitle(), cardContent.String(), width, focused, sectionAccent))
	b.WriteString("\n")

	// Filter status (compact) - outside the card
	var filters []string
	if m.filter.Type != "" {
		filters = append(filters, string(m.filter.Type))
	}
	if m.filter.Priority == 1 {
		filters = append(filters, "!high")
	}
	if len(m.filter.Tags) > 0 {
		filters = append(filters, fmt.Sprintf("%d tags", len(m.filter.Tags)))
	}

	if len(filters) > 0 {
		b.WriteString(FilterBadgeStyle.Render(fmt.Sprintf("[%s]", strings.Join(filters, ","))))
		b.WriteString(" ")
	}
	if m.sort.Field != SortByDefault {
		direction := "↑"
		if !m.sort.Ascending {
			direction = "↓"
		}
		b.WriteString(MutedStyle.Render(fmt.Sprintf("Sort: %s %s", m.sort.Field, direction)))
	}

	return b.String()
}

package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// renderFullWidth renders the traditional full-width tree view (sidebar disabled).
func (m *ProjectTreeModel) renderFullWidth() string {
	// Full-width mode has no sidebar, so always focused
	return m.renderTreeContent(m.width, true)
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
	treeContent := m.renderTreeContent(treeWidth, !m.sidebar.IsFocused())

	// Get currently selected task
	selectedTask := m.SidebarSelectedTask()

	paneHeight := max(m.height-CardVerticalBorderOverhead, 1)

	// Render sidebar
	// Note: SidebarStyle has border (2 chars) + padding (2 chars) = 4 chars overhead
	// lipgloss .Width() sets CONTENT width, so subtract overhead to get desired total width
	// Add 1 to account for rounding to ensure we use full width
	sidebarContent := renderSidebarContent(m, selectedTask, sidebarWidth-SidebarContentOverhead, paneHeight)

	// Apply width constraint to tree to ensure it fills its allocated space
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)

	// Render vertical divider
	divider := renderVerticalDivider(paneHeight, m.sidebar.IsFocused())

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
func (m *ProjectTreeModel) renderTreeContent(width int, focused bool) string { //nolint:gocyclo
	var b strings.Builder

	// Title with inline filter/sort
	titleParams := &TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     m.sort.Field,
		SortAscending: m.sort.Ascending,
		Width:         width,
		Counter:       FormatOpenLoops(m.openLoops),
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
	sectionWarning := ""
	if sn := viewModeToSection(m.viewMode); sn != "" {
		count := sectionTaskCount(m.project.Tasks, sn)
		limit := m.config.GetSectionLimit(sn)
		if limit > 0 && count > limit {
			sectionWarning = sectionLimitWarning(count-limit, limit)
		}
	}

	// Build card content (task list + scrollbar)
	var cardContent strings.Builder

	// Calculate search help height (needed even when no tasks match)
	sbHeaderOffset := TreeHeaderOffset
	if m.fillTreeCardToViewHeight {
		sbHeaderOffset = renderedLineCount(strings.TrimSuffix(b.String(), "\n"))
	} else {
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
	}
	var sidebarSearchHelpHeight int
	if m.IsSearchHelpVisible() {
		totalAvail := m.height - sbHeaderOffset - CardVerticalBorderOverhead
		sidebarSearchHelpHeight = max(totalAvail/2, 1)
	}
	cardHeight := 0
	if m.fillTreeCardToViewHeight {
		// Leave one compositor row so the card bottom border stays visible
		// directly above the Project Cycle popup.
		cardHeight = max(m.height-sbHeaderOffset-1, CardVerticalBorderOverhead)
	}

	if len(visible) == 0 {
		if len(m.project.Tasks) == 0 {
			cardContent.WriteString(renderEmptyState(EmptyStateNoTasks, "Press 'i' to add a task"))
		} else {
			cardContent.WriteString(renderEmptyState(EmptyStateFiltered, "Press Esc to clear filters"))
		}
	} else {
		// Calculate available height for task list
		// Card border consumes 2 lines (top + bottom)
		availableHeight := m.height - sbHeaderOffset - CardVerticalBorderOverhead
		if m.fillTreeCardToViewHeight {
			availableHeight = cardHeight - CardVerticalBorderOverhead
		}
		if m.fillTreeCardToViewHeight && sectionWarning != "" {
			availableHeight--
		}

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

		cardContentWidth := width - CardBorderOverhead

		totalRenderableLines := treeRenderLineCount(visible, 0, len(visible))
		needsScrollbar := totalRenderableLines > availableHeight
		if needsScrollbar {
			// Reserve 1 char for scrollbar when content overflows.
			cardContentWidth--
		}

		// Determine visible range, accounting for separator line cost between groups.
		startIdx := 0
		endIdx := len(visible)
		if needsScrollbar {
			startIdx, endIdx = persistentProjectTreeWindowByLines(visible, m.cursor, availableHeight, &m.viewScrollOffset)
		}

		// Compute column layout for the visible slice (alignment guard: >= MinColumnAlignWidth)
		if cardContentWidth >= MinColumnAlignWidth {
			m.colLayout = ComputeTreeColumnLayout(visible[startIdx:endIdx], m.columnConfig)
			// Augment with status/type/stage widths from the same node set.
			tasks := make([]*models.Task, 0, endIdx-startIdx)
			for _, n := range visible[startIdx:endIdx] {
				if n != nil && n.Task != nil {
					tasks = append(tasks, n.Task)
				}
			}
			if badge := ComputeBadgeWidths(tasks, BadgeWidthOpts{
				IncludeStatus: true,
				IncludeType:   true,
				IncludeStage:  true,
			}); badge != nil {
				if m.colLayout == nil {
					m.colLayout = badge
				} else {
					m.colLayout.HasStatus = badge.HasStatus
					m.colLayout.StatusWidth = badge.StatusWidth
					m.colLayout.HasType = badge.HasType
					m.colLayout.TypeWidth = badge.TypeWidth
					m.colLayout.HasStage = badge.HasStage
					m.colLayout.StageWidth = badge.StageWidth
				}
			}
		} else {
			m.colLayout = nil
		}

		// Render visible tasks (with adjusted width for card content)
		var lines []string
		prevGroupIdx := -1
		for i := startIdx; i < endIdx; i++ {
			node := visible[i]
			if prevGroupIdx >= 0 && node.TopGroupIndex != prevGroupIdx {
				lines = append(lines, "")
			}
			prevGroupIdx = node.TopGroupIndex
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
			scrollOffset := treeRenderLineCount(visible, 0, startIdx)
			scrollbar := RenderScrollbar(totalRenderableLines, availableHeight, scrollOffset)
			content = attachScrollbarAtWidth(content, scrollbar, cardContentWidth)
		}
		cardContent.WriteString(content)
	}

	if sectionWarning != "" {
		cardContent.WriteString("\n")
		cardContent.WriteString(PreviewWarningStyle.Render(sectionWarning))
	}

	// Render the card with view mode as title
	// Use section color accent if exactly one section is filtered
	sectionAccent := m.getSingleFilteredSectionColor()
	b.WriteString(renderSidebarCardWithHeight(m.getViewModeTitle(), cardContent.String(), width, focused, sectionAccent, cardHeight))
	if !m.fillTreeCardToViewHeight {
		b.WriteString("\n")
	}

	// Render search help pane below the card
	if m.IsSearchHelpVisible() {
		b.WriteString(renderSearchHelpPane(width, sidebarSearchHelpHeight))
		b.WriteString("\n")
	}

	return b.String()
}

func renderedLineCount(content string) int {
	if content == "" {
		return 0
	}
	return strings.Count(content, "\n") + 1
}

// treeRenderLineCount counts rendered lines for [start,end), including blank
// separator lines inserted between top-level groups.
func treeRenderLineCount(visible []*TreeNode, start, end int) int {
	if len(visible) == 0 {
		return 0
	}
	start = max(start, 0)
	end = min(end, len(visible))
	if start >= end {
		return 0
	}

	lines := 0
	prevGroupIdx := -1
	for i := start; i < end; i++ {
		groupIdx := visible[i].TopGroupIndex
		if prevGroupIdx >= 0 && groupIdx != prevGroupIdx {
			lines++
		}
		lines++
		prevGroupIdx = groupIdx
	}
	return lines
}

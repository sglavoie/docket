package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

// Tree connector characters for outliner rendering.
const (
	connectorBranch   = "├── "
	connectorLastItem = "└── "
	connectorPipe     = "│   "
	connectorSpace    = "    "
)

// outlinerRow represents a single flattened row for rendering.
type outlinerRow struct {
	task            *models.Task
	prefix          string
	IsSectionHeader bool   // True if this row is a section header (not a task)
	SectionName     string // Section name for header rows
	SectionCount    int    // Number of tasks in this section
}

// buildTreePrefix computes the connector prefix string from the ancestor
// is-last-child boolean stack. Each true entry means that ancestor was the
// last child of its parent (so we render spaces), while false means we need
// a continuation pipe.
func buildTreePrefix(ancestorIsLast []bool, isLast bool) string {
	var b strings.Builder
	for _, last := range ancestorIsLast {
		if last {
			b.WriteString(connectorSpace)
		} else {
			b.WriteString(connectorPipe)
		}
	}
	if isLast {
		b.WriteString(connectorLastItem)
	} else {
		b.WriteString(connectorBranch)
	}
	return b.String()
}

// flattenTaskTree recursively walks the task tree and produces a flat list of
// outlinerRows with pre-computed connector prefixes. The ancestorIsLast stack
// tracks whether each ancestor is its parent's last child, which determines
// whether continuation pipes or spaces are drawn.
func flattenTaskTree(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 {
			flattenTaskTree(task.Children, append(ancestorIsLast, isLast), rows)
		}
	}
}

// matchesSearch checks if a task matches the search query by fuzzy-matching
// the title, tags, description, and comments.
func matchesSearch(task *models.Task, query string) bool {
	return TaskMatchesSearch(task, query)
}

// flattenTaskTreeWithFolds recursively walks the task tree and produces a flat
// list of outlinerRows, respecting fold state. Collapsed nodes (not in
// expandedNodes or explicitly false) stop recursion, showing only the parent.
// When expandedNodes is empty (default), all nodes are expanded.
func (m *OutlinerModel) flattenTaskTreeWithFolds(tasks []*models.Task, ancestorIsLast []bool, rows *[]outlinerRow) {
	for i, task := range tasks {
		isLast := i == len(tasks)-1
		prefix := ""
		if len(ancestorIsLast) > 0 || len(tasks) > 0 {
			prefix = buildTreePrefix(ancestorIsLast, isLast)
		}
		*rows = append(*rows, outlinerRow{task: task, prefix: prefix})

		if len(task.Children) > 0 && m.isExpanded(task.ID) {
			m.flattenTaskTreeWithFolds(task.Children, append(ancestorIsLast, isLast), rows)
		}
	}
}

// isExpanded returns true if the node should show its children.
// When the expandedNodes map is empty (default), all nodes are expanded.
func (m *OutlinerModel) isExpanded(taskID string) bool {
	if len(m.expandedNodes) == 0 {
		return true
	}
	expanded, exists := m.expandedNodes[taskID]
	if !exists {
		return true // Default to expanded
	}
	return expanded
}

// breadcrumb returns the rendered breadcrumb string showing the ancestor chain
// from the first zoom level to the current zoom target. Returns empty string
// when not zoomed. If the breadcrumb exceeds maxWidth, leftmost segments are
// dropped and replaced with "..." until it fits.
func (m *OutlinerModel) breadcrumb(maxWidth int) string {
	if len(m.zoomStack) == 0 || m.project == nil {
		return ""
	}

	separator := " > "
	var segments []string
	for _, entry := range m.zoomStack {
		if t := m.project.GetTaskByID(entry.TaskID); t != nil {
			segments = append(segments, t.Title)
		}
	}
	if len(segments) == 0 {
		return ""
	}

	full := strings.Join(segments, separator)
	rendered := MutedStyle.Render(full)
	if lipgloss.Width(rendered) <= maxWidth {
		return rendered
	}

	// Truncate from the left, replacing dropped segments with "...".
	ellipsis := "..."
	for len(segments) > 1 {
		segments = segments[1:]
		candidate := ellipsis + separator + strings.Join(segments, separator)
		rendered = MutedStyle.Render(candidate)
		if lipgloss.Width(rendered) <= maxWidth {
			return rendered
		}
	}

	// Even the last segment alone with ellipsis doesn't fit; render it anyway.
	return MutedStyle.Render(ellipsis + separator + segments[0])
}

// View implements tea.Model. It renders the full outliner tree with connector
// prefixes computed on each render pass from the project's task hierarchy.
func (m *OutlinerModel) View() string {
	if m.project == nil {
		return "No project loaded"
	}

	// Rebuild flat row list with perspective filter and clamp cursor.
	m.rebuildRows()

	if m.sidebar.IsEnabled() {
		return m.renderWithSidebar()
	}
	return m.renderFullWidth()
}

// renderFullWidth renders the outliner tree using the full terminal width.
func (m *OutlinerModel) renderFullWidth() string {
	return m.renderOutlinerTreeContent(m.width)
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *OutlinerModel) renderWithSidebar() string {
	treeWidth := (m.width * 55) / 100
	if treeWidth < TreeMinWidth {
		treeWidth = TreeMinWidth
	}

	dividerWidth := 1
	sidebarWidth := m.width - treeWidth - dividerWidth
	if sidebarWidth < SidebarMinWidth {
		sidebarWidth = SidebarMinWidth
		treeWidth = m.width - sidebarWidth - dividerWidth
	}

	treeContent := m.renderOutlinerTreeContent(treeWidth)

	var selectedTask *models.Task
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) {
		row := m.rows[m.cursor]
		if !row.IsSectionHeader {
			selectedTask = row.task
		}
	}

	sidebarContent := renderSidebarContent(m, selectedTask, sidebarWidth-SidebarContentOverhead, m.height-CardVerticalBorderOverhead)
	treeContentStyled := lipgloss.NewStyle().Width(treeWidth).Render(treeContent)
	divider := renderVerticalDivider(m.height-2, m.sidebar.IsFocused())

	// Dim the unfocused pane to reinforce which pane has focus
	if m.sidebar.IsFocused() {
		treeContentStyled = DimStyle.Render(treeContentStyled)
	} else {
		sidebarContent = DimStyle.Render(sidebarContent)
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, treeContentStyled, divider, sidebarContent)
}

// renderOutlinerTreeContent renders the tree portion of the outliner, constrained to
// the given width. Used by both full-width and split-pane layouts.
func (m *OutlinerModel) renderOutlinerTreeContent(width int) string {
	// Capture and clear the transient zoom notice (shown for one render).
	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()

	// Show perspective label only for non-All perspectives.
	perspectiveLabel := ""
	if perspective.Name != "All" {
		perspectiveLabel = "  " + MutedStyle.Render("["+perspective.Name+"]")
	}

	bc := m.breadcrumb(width)

	// Build search indicator (stays inline on title)
	searchIndicator := ""
	if m.IsSearchMode() {
		searchIndicator = "  " + MutedStyle.Render("Search: ") + m.SearchInputView()
	} else if m.SearchQuery() != "" {
		searchIndicator = "  " + MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}

	// Filter bar rendered on its own dedicated line
	filterBarLine := m.renderFilterBar()

	if len(m.rows) == 0 {
		header := renderProjectTitle(m.project, m.config) + perspectiveLabel + searchIndicator
		emptyMsg := EmptyStateNoTasks
		if perspective.Name != "All" && len(m.zoomStack) > 0 {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			if t := m.project.GetTaskByID(topID); t != nil {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective within %q.", perspective.Name, t.Title)
			} else {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective.", perspective.Name)
			}
		} else if perspective.Name != "All" {
			emptyMsg = fmt.Sprintf("No tasks match the %s perspective.", perspective.Name)
		}
		result := header + "\n"
		if filterBarLine != "" {
			result += filterBarLine + "\n"
		}
		if tabs := m.customSectionTabs(); tabs != nil {
			result += RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors) + "\n"
		}
		if bc != "" {
			result += bc + "\n"
		}
		result += MutedStyle.Render(emptyMsg)
		if notice != "" {
			result += "\n\n" + PreviewWarningStyle.Render(notice)
		}
		return result
	}

	var b strings.Builder
	b.WriteString(renderProjectTitle(m.project, m.config))
	b.WriteString(perspectiveLabel)
	b.WriteString(searchIndicator)
	if m.multiSelectMode {
		count := m.SelectedCount()
		b.WriteString("  ")
		b.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count)))
	}
	b.WriteString("\n")

	// Filter bar on dedicated line (below title, above tabs/breadcrumb)
	if filterBarLine != "" {
		b.WriteString(filterBarLine)
		b.WriteString("\n")
	}

	// Custom section tab bar
	if tabs := m.customSectionTabs(); tabs != nil {
		b.WriteString(RenderColoredTabBar(tabs, m.customSectionTab, width, 1, m.project.SectionColors))
		b.WriteString("\n")
	}

	if bc != "" {
		b.WriteString(bc)
		b.WriteString("\n")
	}

	// Compute scroll window.
	visibleRows := m.contentHeight()

	// Reserve space for inline preview lines when preview mode is on
	previewLines := 0
	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
		previewLines = inlinePreviewLineCount(m.rows[m.cursor].task)
		visibleRows -= previewLines
		if visibleRows < 1 {
			visibleRows = 1
		}
	}

	startIdx := calcScrollOffset(m.cursor, visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.rows))

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		// Handle section header rows
		if row.IsSectionHeader {
			// Display name for empty section (Active)
			displayName := row.SectionName
			if displayName == "" {
				displayName = "Active"
			}

			// Get colored style for this section
			sectionStyle := GetViewModeStyleForSection(row.SectionName)

			// Triangle indicator (▾ expanded, ▸ collapsed)
			indicator := "▾"
			if m.collapsedSections[row.SectionName] {
				indicator = "▸"
			}

			// Cursor prefix
			cursorPrefix := "  "
			if selected {
				cursorPrefix = SelectionCursorStyle.Render(">") + " "
			}

			// Render colored badge with @ symbol at the very left
			badge := sectionStyle.Render(fmt.Sprintf("@ %s", displayName))
			line := fmt.Sprintf("%s%s %s (%d)", cursorPrefix, MutedStyle.Render(indicator), badge, row.SectionCount)

			if selected {
				rowStyle := SelectionRowStyle
				if m.cursorFlash {
					rowStyle = CursorFlashRowStyle
				}
				line = rowStyle.Width(width).Render(line)
			}

			rowBuilder.WriteString(line)
			if i < endIdx-1 {
				rowBuilder.WriteString("\n")
			}
			continue
		}

		// Regular task row rendering
		task := row.task
		// Fold indicator for parent tasks
		foldIndicator := ""
		if len(task.Children) > 0 {
			if m.isExpanded(task.ID) {
				foldIndicator = MutedStyle.Render("▾") + " "
			} else {
				foldIndicator = MutedStyle.Render("▸") + " "
			}
		}

		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		if m.project.HasIncompleteDependencies(task) {
			statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
		}

		// Title with priority coloring
		titleStyle := NormalStyle
		if task.Status == models.StatusCanceled {
			titleStyle = CanceledStyle
		} else if task.Active {
			titleStyle = ActiveStyle
		} else if task.Pinned {
			titleStyle = SuccessStyle
		}

		// Build indicators in semantic groups for visual clustering
		// Organization group: short ID, custom section, section badge
		var orgParts []string
		if m.columnConfig.IsVisible(ColShortID) {
			orgParts = append(orgParts, MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
		}
		if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" {
			csColor := m.project.GetSectionColor(task.CustomSection)
			csStyle := GetSectionColorStyle(csColor)
			orgParts = append(orgParts, csStyle.Render("@ "+task.CustomSection))
		}
		if p := m.currentPerspective(); p.Name != "All" {
			sectionName := task.Section
			if sectionName == "" {
				sectionName = "Active"
			}
			sectionStyle := GetViewModeStyleForSection(task.Section)
			orgParts = append(orgParts, sectionStyle.Render("@ "+sectionName))
		}
		orgGroup := JoinGroupParts(orgParts...)

		// Lifecycle group: priority, due date, snooze, recurrence
		var lifecycleParts []string
		if m.columnConfig.ShouldShowPriority(task.Priority) {
			lifecycleParts = append(lifecycleParts, renderPriorityBadge(task.Priority))
		}
		if m.columnConfig.IsVisible(ColDueDate) {
			if dueStr := formatDueDate(task); dueStr != "" {
				lifecycleParts = append(lifecycleParts, dueStr)
			}
		}
		if m.columnConfig.IsVisible(ColSnooze) && task.SnoozedUntil != nil {
			snoozeStr := task.SnoozedUntil.Format("Jan 2")
			if task.SnoozedUntil.Year() != time.Now().Year() {
				snoozeStr = task.SnoozedUntil.Format("Jan 2 '06")
			}
			lifecycleParts = append(lifecycleParts, MutedStyle.Render("zzz "+snoozeStr))
		}
		if m.columnConfig.IsVisible(ColRecurrence) && task.IsRecurring() {
			recurStr := "↻"
			if task.Recurrence.Streak > 0 {
				recurStr = fmt.Sprintf("↻%d", task.Recurrence.Streak)
			}
			lifecycleParts = append(lifecycleParts, MutedStyle.Render(recurStr))
		}
		lifecycleGroup := JoinGroupParts(lifecycleParts...)

		// Structure group: progress, dependencies, score
		var structureParts []string
		if m.columnConfig.IsVisible(ColProgress) {
			if frac := renderProgressFraction(task); frac != "" {
				structureParts = append(structureParts, frac)
			}
		}
		if m.columnConfig.IsVisible(ColDependencies) && !m.sidebar.IsEnabled() {
			if s := renderDependencyIndicators(task, m.project); s != "" {
				structureParts = append(structureParts, s)
			}
		}
		if m.columnConfig.IsVisible(ColScore) && m.config != nil {
			if profile := m.config.GetProjectScoringProfile(m.project.ID); profile != nil {
				if HasManualScores(task, profile) {
					structureParts = append(structureParts, RenderScoreBadge(task, profile))
				}
			}
		}
		structureGroup := JoinGroupParts(structureParts...)

		// Content group: tags, comments, attachments, checklist
		var contentParts []string
		if m.columnConfig.IsVisible(ColTags) {
			if s := renderTruncatedTags(task.Tags, 2, 8); s != "" {
				contentParts = append(contentParts, s)
			}
		}
		if m.columnConfig.IsVisible(ColComments) {
			if s := renderCommentIndicator(task); s != "" {
				contentParts = append(contentParts, s)
			}
		}
		if m.columnConfig.IsVisible(ColAttachments) {
			if s := renderAttachmentIndicator(task); s != "" {
				contentParts = append(contentParts, s)
			}
		}
		if m.columnConfig.IsVisible(ColChecklist) {
			if s := renderChecklistIndicator(task); s != "" {
				contentParts = append(contentParts, s)
			}
		}
		contentGroup := JoinGroupParts(contentParts...)

		indicatorStr := JoinIndicatorGroups(orgGroup, lifecycleGroup, structureGroup, contentGroup)

		cursorPrefix := RenderCursorPrefix(selected, task.Active, task.Pinned)

		// Selection checkbox (only in multi-select mode)
		checkbox := ""
		if m.multiSelectMode {
			if m.IsSelected(task.ID) {
				checkbox = SelectionCheckStyle.Render("[x]") + " "
			} else {
				checkbox = MutedStyle.Render("[ ]") + " "
			}
		}

		// Build final indicator string with leading space
		indicators := indicatorStr
		if indicators != "" {
			indicators = " " + indicators
		}

		// Truncate title to fit available width
		availableWidth := max(width-lipgloss.Width(row.prefix)-lipgloss.Width(cursorPrefix)-lipgloss.Width(checkbox)-12-lipgloss.Width(indicators), 10)
		truncatedTitle := util.Truncate(task.Title, availableWidth)
		var title string
		query := strings.TrimSpace(m.SearchQuery())
		if query != "" {
			_, positions := fuzzy.ScoreTermsWithPositions(truncatedTitle, query)
			if len(positions) > 0 {
				title = HighlightText(truncatedTitle, positions, titleStyle)
			} else {
				title = titleStyle.Render(truncatedTitle)
			}
		} else {
			title = titleStyle.Render(truncatedTitle)
		}

		line := fmt.Sprintf("%s%s%s%s%s %s %s%s",
			cursorPrefix,
			checkbox,
			MutedStyle.Render(row.prefix),
			foldIndicator,
			typeIcon, statusIcon, title, indicators)

		if selected {
			rowStyle := SelectionRowStyle
			if m.cursorFlash {
				rowStyle = CursorFlashRowStyle
			}
			line = rowStyle.Width(width).Render(line)
		}

		rowBuilder.WriteString(line)
		// Append inline preview lines after the cursor task
		if m.previewMode && i == m.cursor && !row.IsSectionHeader {
			indent := row.prefix + "    " // indent past the tree connector
			for _, pl := range inlinePreviewLines(task, indent, width) {
				rowBuilder.WriteString("\n")
				rowBuilder.WriteString(pl)
			}
		}
		if i < endIdx-1 {
			rowBuilder.WriteString("\n")
		}
	}

	// Attach scrollbar to the right edge when content overflows
	scrollbar := RenderScrollbar(len(m.rows), visibleRows, startIdx)
	b.WriteString(AttachScrollbar(rowBuilder.String(), scrollbar))

	if notice != "" {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
		b.WriteString("\n")
	}

	// Render quick edit bar if active
	if m.quickEdit.IsActive() {
		b.WriteString("\n")
		b.WriteString(m.renderQuickEditBar())
		b.WriteString("\n")
	}

	// Render status message
	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	return b.String()
}

// hasActiveFilters returns true if any filter, sort, or saved view is currently active.
func (m *OutlinerModel) hasActiveFilters() bool {
	return m.activeSavedView != "" ||
		m.sort.Field != SortByDefault ||
		hasActiveCommonFilters(&m.filter.CommonFilterState) ||
		len(m.filter.Sections) > 0 ||
		m.filter.Actionable
}

// renderFilterBar renders the filter bar on its own dedicated line.
// Returns empty string when filter bar is disabled or no filters/sort are active.
func (m *OutlinerModel) renderFilterBar() string {
	if !m.filterBar.IsEnabled() || !m.hasActiveFilters() {
		return ""
	}

	var parts []string

	// Filter indicator
	if indicator := m.renderActiveFilterIndicator(); indicator != "" {
		parts = append(parts, indicator)
	}

	// Sort badge
	if m.sort.Field != SortByDefault {
		direction := "↑"
		if !m.sort.Ascending {
			direction = "↓"
		}
		parts = append(parts, MutedStyle.Render(fmt.Sprintf("Sort: %s %s", m.sort.Field, direction)))
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "  ")
}

// clearActiveSavedView clears the active saved view indicator when the user
// manually changes filter/sort state.
func (m *OutlinerModel) clearActiveSavedView() {
	m.activeSavedView = ""
}

// renderActiveFilterIndicator returns a compact indicator of all active filters.
// Returns empty string when no filters are active.
func (m *OutlinerModel) renderActiveFilterIndicator() string {
	var parts []string

	// Show saved view badge if active
	if m.activeSavedView != "" {
		parts = append(parts, SavedViewBadgeStyle.Render("View: "+m.activeSavedView))
	}

	// Add common filter parts (type, status, priority, tags, due date, attrs, score)
	parts = append(parts, commonFilterParts(&m.filter.CommonFilterState)...)

	// Add view-specific filter parts
	if len(m.filter.Sections) > 0 {
		var secNames []string
		for sec := range m.filter.Sections {
			secNames = append(secNames, "@"+sec)
		}
		parts = append(parts, strings.Join(secNames, " "))
	}
	if m.filter.Actionable {
		parts = append(parts, "ready")
	}

	return renderFilterBadge(parts)
}

// renderQuickEditBar renders the quick edit input bar at the bottom of the view.
func (m *OutlinerModel) renderQuickEditBar() string {
	if !m.quickEdit.IsActive() {
		return ""
	}

	var lines []string

	mode := "Quick Edit"
	if m.quickEdit.IsRenameOnly() {
		mode = "Rename"
	}
	modeLabel := lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(mode + ":")

	inputLine := modeLabel + " " + m.quickEdit.GetInput().View()
	lines = append(lines, inputLine)

	// Preview line (only for full quick edit mode)
	if !m.quickEdit.IsRenameOnly() {
		preview := m.quickEdit.GetPreview()
		original := m.quickEdit.GetOriginal()
		if preview != nil && original != nil {
			previewStr := renderQuickEditPreview(preview, original)
			if previewStr != "" {
				lines = append(lines, MutedStyle.Render(previewStr))
			}
		}
	}

	helpHint := MutedStyle.Render("Enter=save  Esc=cancel")
	if !m.quickEdit.IsRenameOnly() {
		helpHint = MutedStyle.Render("!1-5=priority #tag @date /section $status · Enter=save Esc=cancel")
	}
	lines = append(lines, helpHint)

	return strings.Join(lines, "\n")
}

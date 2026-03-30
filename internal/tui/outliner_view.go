package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/models"
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
	CollapseKey     string // Key for collapsedSections map; "_xp:sectionName" sentinel in All-perspective cross-project mode (shared headers, no per-project prefix needed), empty in single-project mode
	ProjectID       string // Project that owns this row; set on task rows and cross-project section headers
	IsSeparator     bool   // True if this row is a visual separator between projects
}

// collapseKey returns the effective key for collapsedSections lookups.
// Cross-project headers set CollapseKey to "projectID:sectionName"; single-project
// headers leave it empty and fall back to SectionName.
func (r outlinerRow) effectiveCollapseKey() string {
	if r.CollapseKey != "" {
		return r.CollapseKey
	}
	return r.SectionName
}

// projectNameByID looks up a project name from m.loadedProjects by ID.
// Returns the ID itself as a fallback if the project is not found.
func (m *OutlinerModel) projectNameByID(id string) string {
	for _, p := range m.loadedProjects {
		if p.ID == id {
			return p.Name
		}
	}
	return id
}

// projectSlugByID looks up a project slug from m.loadedProjects by ID.
// Falls back to projectNameByID when the slug is empty.
func (m *OutlinerModel) projectSlugByID(id string) string {
	for _, p := range m.loadedProjects {
		if p.ID == id {
			if p.Slug != "" {
				return p.Slug
			}
			return p.Name
		}
	}
	return id
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
			// Copy before appending to prevent slice aliasing: without the
			// copy, siblings that recurse into children could share the same
			// backing array, corrupting each other's ancestor trail when the
			// capacity isn't exhausted.
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			flattenTaskTree(task.Children, child, rows)
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
			// Copy before appending to prevent slice aliasing (see flattenTaskTree).
			child := make([]bool, len(ancestorIsLast)+1)
			copy(child, ancestorIsLast)
			child[len(ancestorIsLast)] = isLast
			m.flattenTaskTreeWithFolds(task.Children, child, rows)
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
func (m *OutlinerModel) View() tea.View {
	if m.project == nil {
		// Spinner/loading states handled by crossProjectView.
		if m.crossSpinnerVisible || m.crossLoading {
			return tea.NewView(m.crossProjectView())
		}
		m.crossProjectRebuildRows()
		if m.sidebar.IsEnabled() {
			return tea.NewView(m.renderWithSidebar())
		}
		return tea.NewView(m.renderOutlinerTreeContent(m.width))
	}

	// Rebuild flat row list with perspective filter and clamp cursor.
	m.rebuildRows()

	if m.sidebar.IsEnabled() {
		return tea.NewView(m.renderWithSidebar())
	}
	return tea.NewView(m.renderFullWidth())
}

// renderFullWidth renders the outliner tree using the full terminal width.
func (m *OutlinerModel) renderFullWidth() string {
	return m.renderOutlinerTreeContent(m.width)
}

// renderWithSidebar renders the split-pane view with tree on left and sidebar on right.
func (m *OutlinerModel) renderWithSidebar() string {
	treeWidth := max((m.width*55)/100, TreeMinWidth)

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
func (m *OutlinerModel) renderOutlinerTreeContent(width int) string { //nolint:gocyclo
	// Capture and clear the transient zoom notice (shown for one render).
	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()

	presets := PerspectivePresets()
	perspCount := len(presets)
	prevIdx := (m.activePerspective + perspCount - 1) % perspCount
	nextIdx := (m.activePerspective + 1) % perspCount

	currentStyle := GetViewModeStyleForSection(perspective.Name)
	prevName := presets[prevIdx].Name
	nextName := presets[nextIdx].Name

	perspectiveLabel := "  " +
		MutedStyle.Render("‹ "+prevName) + " " +
		currentStyle.Render("@ "+perspective.Name) + " " +
		MutedStyle.Render(nextName+" ›")

	var bc string
	if m.project == nil {
		bc = m.crossProjectBreadcrumb(width)
	} else {
		bc = m.breadcrumb(width)
	}

	// Build search indicator (stays inline on title)
	searchIndicator := ""
	if m.IsSearchMode() {
		searchIndicator = "  " + MutedStyle.Render("Search: ") + m.SearchInputView()
	} else if m.SearchQuery() != "" {
		searchIndicator = "  " + MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}

	if len(m.rows) == 0 {
		var headerStr string
		if m.project == nil {
			projectCount := len(m.loadedProjects)
			title := fmt.Sprintf("Cross-project outliner (%d project", projectCount)
			if projectCount != 1 {
				title += "s"
			}
			title += ")"
			headerStr = TitleStyle.Render(title)
			headerStr += buildTitleFilterSuffix(&TitleParams{
				SortField:     m.sort.Field,
				SortAscending: m.sort.Ascending,
				Width:         width,
			}, len(title))
		} else {
			titleParams := &TitleParams{
				FilterParts:   m.activeFilterParts(),
				SortField:     m.sort.Field,
				SortAscending: m.sort.Ascending,
				Width:         width,
			}
			headerStr = renderProjectTitle(m.project, m.config, titleParams)
		}
		header := headerStr + perspectiveLabel + searchIndicator
		emptyMsg := EmptyStateNoTasks
		if m.project == nil && len(m.loadedProjects) == 0 {
			emptyMsg = "No projects in current context. Press C to switch context."
		} else if m.project == nil {
			emptyMsg = "No tasks found."
		} else if perspective.Name != "All" && len(m.zoomStack) > 0 && m.project != nil {
			topID := m.zoomStack[len(m.zoomStack)-1].TaskID
			if t := m.project.GetTaskByID(topID); t != nil {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective within %q. Try switching perspectives with [/].", perspective.Name, t.Title)
			} else {
				emptyMsg = fmt.Sprintf("No tasks match the %s perspective. Try switching perspectives with [/].", perspective.Name)
			}
		} else if perspective.Name != "All" {
			emptyMsg = fmt.Sprintf("No tasks match the %s perspective. Try switching perspectives with [/].", perspective.Name)
		} else if m.SearchQuery() != "" {
			emptyMsg = fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
		} else if hasActiveCommonFilters(&m.filter.CommonFilterState) || len(m.filter.Sections) > 0 || m.filter.Actionable {
			emptyMsg = EmptyStateFiltered
		}
		result := header + "\n"
		if m.project != nil {
			if tabs := m.customSectionTabs(); tabs != nil {
				result += RenderColoredTabBar(tabs, m.customSectionTabIndex(), width, 1, m.project.SectionColors) + "\n"
			}
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
	if m.project == nil {
		projectCount := len(m.loadedProjects)
		title := fmt.Sprintf("Cross-project outliner (%d project", projectCount)
		if projectCount != 1 {
			title += "s"
		}
		title += ")"
		b.WriteString(TitleStyle.Render(title))
		b.WriteString(buildTitleFilterSuffix(&TitleParams{
			SortField:     m.sort.Field,
			SortAscending: m.sort.Ascending,
			Width:         width,
		}, len(title)))
	} else {
		titleParams := &TitleParams{
			FilterParts:   m.activeFilterParts(),
			SortField:     m.sort.Field,
			SortAscending: m.sort.Ascending,
			Width:         width,
		}
		b.WriteString(renderProjectTitle(m.project, m.config, titleParams))
	}
	b.WriteString(perspectiveLabel)
	b.WriteString(searchIndicator)
	if m.multiSelectMode {
		count := m.SelectedCount()
		b.WriteString("  ")
		b.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", count)))
	}
	b.WriteString("\n")

	// Custom section tab bar (single-project only)
	if m.project != nil {
		if tabs := m.customSectionTabs(); tabs != nil {
			b.WriteString(RenderColoredTabBar(tabs, m.customSectionTabIndex(), width, 1, m.project.SectionColors))
			b.WriteString("\n")
		}
	}

	if bc != "" {
		b.WriteString(bc)
		b.WriteString("\n")
	}

	// Compute scroll window.
	visibleRows := m.contentHeight()

	// Reserve space for inline preview lines when preview mode is on
	var previewLines int
	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
		previewLines = inlinePreviewLineCount(m.rows[m.cursor].task)
		visibleRows -= previewLines
		if visibleRows < 1 {
			visibleRows = 1
		}
	}

	startIdx := calcScrollOffset(m.cursor, visibleRows)
	endIdx := min(startIdx+visibleRows, len(m.rows))

	// Reserve 1 column for the scrollbar when content overflows the viewport.
	rowWidth := width
	if len(m.rows) > visibleRows {
		rowWidth = width - 1
	}

	var rowBuilder strings.Builder
	for i := startIdx; i < endIdx; i++ {
		row := m.rows[i]
		selected := i == m.cursor

		// Handle visual separator rows (cross-project mode)
		if row.IsSeparator {
			rowBuilder.WriteString("")
			if i < endIdx-1 {
				rowBuilder.WriteString("\n")
			}
			continue
		}

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
			if m.collapsedSections[row.effectiveCollapseKey()] {
				indicator = "▸"
			}

			// Cursor prefix
			cursorPrefix := "  "
			if selected {
				cursorPrefix = SelectionCursorStyle.Render("→") + " "
			}

			// Render colored badge with @ symbol at the very left
			badge := sectionStyle.Render(fmt.Sprintf("@ %s", displayName))
			line := fmt.Sprintf("%s%s %s (%d)", cursorPrefix, MutedStyle.Render(indicator), badge, row.SectionCount)

			if limit := m.config.GetSectionLimit(row.SectionName); limit > 0 && row.SectionCount > limit {
				line += "  " + PreviewWarningStyle.Render(sectionLimitWarning(row.SectionCount-limit, limit))
			}

			// In cross-project mode, append a project name badge to distinguish
			// identically-named sections from different projects.
			if row.ProjectID != "" {
				projName := m.projectNameByID(row.ProjectID)
				line += "  " + MutedStyle.Render("["+projName+"]")
			}

			if selected {
				rowStyle := SelectionRowStyle
				if m.cursorFlash {
					rowStyle = CursorFlashRowStyle
				}
				line = rowStyle.Width(rowWidth).Render(line)
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

		// Project badge — shown when the row has a ProjectID (cross-project mode).
		// Uses the project slug for compact, accessible text alongside the color.
		projBadge := ""
		if row.ProjectID != "" {
			projName := m.projectNameByID(row.ProjectID)
			projSlug := m.projectSlugByID(row.ProjectID)
			badgeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(projectBadgeColor(projName)))
			projBadge = badgeStyle.Render("["+projSlug+"]") + " "
		}

		// Title with priority coloring
		proj := m.resolveProjectForRow(row)
		typeIcon := GetTypeBadge(string(task.Type), task.Type.Icon())
		statusIcon := GetStatusBadge(string(task.Status), task.Status.Icon())
		if proj != nil && proj.HasIncompleteDependencies(task) {
			statusIcon = BlockedByDepsStyle.Render(task.Status.Icon())
		}

		stageBadge := GetStageBadge(task.Stage)
		stageSegment := ""
		if stageBadge != "" {
			stageSegment = stageBadge + " "
		}

		titleStyle := NormalStyle
		if task.Status == models.StatusCanceled {
			titleStyle = CanceledStyle
		} else if task.Active {
			titleStyle = ActiveStyle
		} else if task.Pinned {
			titleStyle = SuccessStyle
		}
		if selected && !task.Dimmed {
			titleStyle = SelectedTaskTitleStyle
		}

		// Build indicators in semantic groups for visual clustering
		// Organization group: short ID, custom section, section badge
		var orgParts []string
		if m.columnConfig.IsVisible(ColShortID) {
			orgParts = append(orgParts, MutedStyle.Render(fmt.Sprintf("#%d", task.ShortID)))
		}
		if m.columnConfig.IsVisible(ColCustomSection) && task.CustomSection != "" {
			if proj != nil {
				csColor := proj.GetSectionColor(task.CustomSection)
				csStyle := GetSectionColorStyle(csColor)
				orgParts = append(orgParts, csStyle.Render("@ "+task.CustomSection))
			}
		}
		if p := m.currentPerspective(); p.Name != "All" && m.project != nil {
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
			if proj != nil {
				if s := renderDependencyIndicators(task, proj); s != "" {
					structureParts = append(structureParts, s)
				}
			}
		}
		if m.columnConfig.IsVisible(ColScore) && m.config != nil && proj != nil {
			if profile := m.config.GetProjectScoringProfile(proj.ID); profile != nil {
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

		cursorPrefix := RenderCursorPrefix(selected, task.Active, task.Pinned, task.Dimmed)

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

		// Truncate title to fit available width.
		// Account for every non-title component plus the two literal spaces in the format string.
		// The EA delta compensates for terminals that render East Asian Ambiguous
		// characters (box-drawing, geometric shapes, middle dots) at double width.
		nonTitleWidth := lipgloss.Width(cursorPrefix) + lipgloss.Width(checkbox) +
			lipgloss.Width(row.prefix) + lipgloss.Width(foldIndicator) +
			lipgloss.Width(projBadge) + lipgloss.Width(typeIcon) + 1 +
			lipgloss.Width(stageSegment) + lipgloss.Width(statusIcon) + 1 +
			lipgloss.Width(indicators)
		availableWidth := max(rowWidth-nonTitleWidth, 10)
		truncatedTitle := truncateToVisualWidth(task.Title, availableWidth)
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

		line := fmt.Sprintf("%s%s%s%s%s%s %s%s %s%s",
			cursorPrefix,
			checkbox,
			MutedStyle.Render(row.prefix),
			foldIndicator,
			projBadge,
			typeIcon, stageSegment, statusIcon, title, indicators)

		if task.Dimmed && !selected {
			line = DimLine(line)
		} else if task.Dimmed && selected {
			line = DimSelectedLine(line, rowWidth)
		}

		if selected && !task.Dimmed {
			rowStyle := SelectionRowStyle
			if m.cursorFlash {
				rowStyle = CursorFlashRowStyle
			}
			line = rowStyle.Width(rowWidth).Render(line)
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

// clearActiveSavedFilter clears the active saved filter indicator and any
// criteria filter. Called when the user manually changes filter/sort state.
func (m *OutlinerModel) clearActiveSavedFilter() {
	m.activeSavedFilter = ""
	m.activeCriteriaFilter = nil
}

// activeFilterParts returns the list of active filter description strings.
func (m *OutlinerModel) activeFilterParts() []string {
	var parts []string

	// Show saved filter badge if active
	if m.activeSavedFilter != "" {
		parts = append(parts, SavedFilterBadgeStyle.Render("View: "+m.activeSavedFilter))
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

	return parts
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
	modeLabel := TitleStyle.Render(mode + ":")

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

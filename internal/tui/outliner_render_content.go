package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
)

type outlinerViewport struct {
	visibleRows      int
	searchHelpHeight int
	startIdx         int
	endIdx           int
	rowWidth         int
}

// renderOutlinerTreeContent renders the tree portion of the outliner, constrained to
// the given width. Used by both full-width and split-pane layouts.
func (m *OutlinerModel) renderOutlinerTreeContent(width int) string {
	if m.modePickerActive() {
		return m.renderModePicker(width)
	}

	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()
	var perspectiveLabel string
	if !m.isActiveCrossPriority() {
		perspectiveLabel = m.buildPerspectiveLabel(perspective)
	}
	breadcrumb := m.renderOutlinerBreadcrumb(width)
	searchIndicator := m.buildOutlinerSearchIndicator()

	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width, perspective, perspectiveLabel, searchIndicator, breadcrumb, notice)
	}

	var b strings.Builder
	m.writeOutlinerHeader(&b, width, perspectiveLabel, searchIndicator)

	// The title row is budgeted as a single line by contentHeight(), but the
	// multi-select badge / long filter suffix can push it past the render width
	// and wrap under the enclosing Width() style (notably the narrow tree pane
	// in split-pane mode). Reserve the extra wrapped rows so the viewport does
	// not overflow terminal height.
	titleExtra := wrappedLineCount(m.buildOutlinerTitleLine(width, perspectiveLabel, searchIndicator), width) - 1

	// Reserve one row for the selected-task metadata strip whenever a row is
	// selectable. The strip stays blank when the cursor sits on a section
	// header or separator so layout does not jitter as the selection moves.
	stripReserved := m.cursor >= 0 && m.cursor < len(m.rows)
	var stripTask *models.Task
	if stripReserved {
		row := m.rows[m.cursor]
		if row.isTaskRow() {
			stripTask = row.task
		}
	}

	extraFooter := 0
	if titleExtra > 0 {
		extraFooter += titleExtra
	}
	if stripReserved {
		extraFooter++
	}
	if notice != "" {
		extraFooter += 2
	}
	if m.quickEdit.IsActive() {
		extraFooter += 2
	}
	if m.statusMessage != "" {
		// One blank separator line plus the status text's own wrapped height.
		// A long status wraps to multiple lines at narrow widths, so reserving
		// a fixed 2 lines under-counts and overflows terminal height.
		extraFooter += 1 + max(wrappedLineCount(m.statusMessage, width), 1)
	}

	viewport := m.computeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport, width))

	if stripReserved {
		var strip string
		if stripTask != nil {
			alreadyShown := TaskMetadataAlreadyShown{Project: m.project != nil}
			if m.columnConfig.ShouldShowPriority(stripTask.Priority) {
				alreadyShown.Priority = true
			}
			if m.columnConfig.IsVisible(ColDueDate) && dueIsInlineHero(stripTask) {
				alreadyShown.Due = true
			}
			strip = renderTaskMetadataStrip(TaskMetadataStripOpts{
				Task:         stripTask,
				Project:      m.resolveProjectForRow(m.rows[m.cursor]),
				Runtime:      m.runtime,
				Width:        width,
				AlreadyShown: alreadyShown,
				ColumnConfig: m.columnConfig,
			})
		}
		b.WriteString("\n")
		b.WriteString(strip)
	}

	if viewport.searchHelpHeight > 0 {
		b.WriteString("\n")
		b.WriteString(renderSearchHelpPane(width, viewport.searchHelpHeight))
	}

	if notice != "" {
		b.WriteString("\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
		b.WriteString("\n")
	}

	if m.quickEdit.IsActive() {
		b.WriteString("\n")
		b.WriteString(m.renderQuickEditBar())
		b.WriteString("\n")
	}

	if m.statusMessage != "" {
		b.WriteString("\n")
		b.WriteString(MutedStyle.Width(width).Render(m.statusMessage))
		b.WriteString("\n")
	}

	return b.String()
}

// wrappedLineCount reports how many terminal rows s occupies once wrapped to
// width. Returns 0 for an empty string or a non-positive width.
func wrappedLineCount(s string, width int) int {
	if s == "" || width <= 0 {
		return 0
	}
	return lipgloss.Height(lipgloss.NewStyle().Width(width).Render(s))
}

func (m *OutlinerModel) buildPerspectiveLabel(perspective Perspective) string {
	activeMode := m.activeOutlinerModeEntry()
	if activeMode.Perspective == noPerspective {
		return "  " + GetViewModeStyleForSection(activeMode.Label).Render("◆ "+activeMode.Label)
	}
	if m.smartFocus != nil {
		return "  " + GetViewModeStyleForSection(m.smartFocus.String()).Render("@ "+m.smartFocus.String()) +
			" " + MutedStyle.Render("P modes")
	}
	prevID := CyclePerspective(perspective.ID, -1)
	nextID := CyclePerspective(perspective.ID, 1)
	previous, _ := PerspectiveByID(prevID)
	next, _ := PerspectiveByID(nextID)

	currentStyle := GetViewModeStyleForSection(perspective.Name)

	return "  " +
		MutedStyle.Render("‹ "+previous.Name) + " " +
		currentStyle.Render("@ "+perspective.Name) + " " +
		MutedStyle.Render(next.Name+" ›")
}

func (m *OutlinerModel) renderOutlinerBreadcrumb(width int) string {
	if m.project == nil {
		return m.crossProjectBreadcrumb(width)
	}
	return m.breadcrumb(width)
}

func (m *OutlinerModel) buildOutlinerSearchIndicator() string {
	if m.IsSearchMode() {
		return "  " + MutedStyle.Render("Search: ") + m.SearchInputView()
	}
	if m.SearchQuery() != "" {
		return "  " + MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}
	return ""
}

func (m *OutlinerModel) renderOutlinerTitle(width int) string {
	if m.project == nil {
		projectCount := len(m.loadedProjects)
		prefix := "Outliner"
		if m.isActiveCrossPriority() {
			prefix = "Outliner: Priority"
		}
		var title string
		if m.isProjectScoped() {
			// Scoped: identify the project instead of reporting the full loaded
			// count (which would mislabel a single-project scope as "N projects").
			title = prefix + " — " + m.projectDisplayNameByID(m.scopedProjectID)
		} else {
			if !m.activeContext.IsAll() {
				// The context is scope chrome, so it names the title and supplies the
				// project count instead of appearing as a filter badge.
				prefix += " — Context: " + m.contextScopeLabel()
				projectCount = len(m.contextScopeProjects())
			}
			title = fmt.Sprintf("%s (%d project", prefix, projectCount)
			if projectCount != 1 {
				title += "s"
			}
			title += ")"
		}
		sortField := m.sort.Field
		sortAscending := m.sort.Ascending
		if m.isActiveCrossPriority() {
			sortField = SortByDefault
			sortAscending = false
		}
		return TitleStyle.Render(title) + buildTitleFilterSuffix(&TitleParams{
			FilterParts:   m.activeFilterParts(),
			SortField:     sortField,
			SortAscending: sortAscending,
			Width:         width,
		}, len(title))
	}

	titleParams := &TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     m.sort.Field,
		SortAscending: m.sort.Ascending,
		Width:         width,
	}
	return renderProjectTitle(m.project, m.runtime, titleParams)
}

// appendOpenLoopsCounter right-aligns the open-loops counter at the end of an
// already-assembled title line. Must be called after every inline title-row
// element (perspective label, search indicator, multi-select badge) has been
// appended so the counter doesn't push them off the right edge.
func (m *OutlinerModel) appendOpenLoopsCounter(line string, width int) string {
	outlinerLoops := m.openLoops
	outlinerLoops.TodayDone = 0
	outlinerLoops.TodayTotal = 0
	counter := FormatOpenLoops(outlinerLoops)
	if counter == "" || width <= 0 {
		return line
	}
	rendered := MutedStyle.Render(counter)
	gap := width - lipgloss.Width(line) - lipgloss.Width(rendered) - titleCounterAlertReserve
	if gap <= 0 {
		return line
	}
	return line + strings.Repeat(" ", gap) + rendered
}

// buildOutlinerTitleLine assembles the single title row (title, perspective
// label, search indicator, optional multi-select badge, right-aligned
// open-loops counter). Shared by writeOutlinerHeader and the height budget so
// both measure the same line.
func (m *OutlinerModel) buildOutlinerTitleLine(width int, perspectiveLabel, searchIndicator string) string {
	var line strings.Builder
	line.WriteString(m.renderOutlinerTitle(width))
	line.WriteString(perspectiveLabel)
	line.WriteString(searchIndicator)
	if m.multiSelectMode {
		line.WriteString("  ")
		line.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", m.SelectedCount())))
	}
	return m.appendOpenLoopsCounter(line.String(), width)
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int, perspectiveLabel, searchIndicator string) {
	b.WriteString(m.buildOutlinerTitleLine(width, perspectiveLabel, searchIndicator))
	b.WriteString("\n")

	if m.ac.IsActive() {
		b.WriteString(m.ac.RenderDropdown("    "))
	}

	if tabBar := m.renderPinnedTagTabBar(width); tabBar != "" {
		b.WriteString(tabBar)
		b.WriteString("\n")
	}

	if breadcrumb := m.renderOutlinerBreadcrumb(width); breadcrumb != "" {
		b.WriteString(breadcrumb)
		b.WriteString("\n")
	}
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int, perspective Perspective, perspectiveLabel, searchIndicator, breadcrumb, notice string) string {
	var b strings.Builder
	var line strings.Builder
	line.WriteString(m.renderOutlinerTitle(width))
	line.WriteString(perspectiveLabel)
	line.WriteString(searchIndicator)
	b.WriteString(m.appendOpenLoopsCounter(line.String(), width))
	b.WriteString("\n")

	if m.ac.IsActive() {
		b.WriteString(m.ac.RenderDropdown("    "))
	}

	if tabBar := m.renderPinnedTagTabBar(width); tabBar != "" {
		b.WriteString(tabBar)
		b.WriteString("\n")
	}

	if breadcrumb != "" {
		b.WriteString(breadcrumb)
		b.WriteString("\n")
	}

	emptyMsg := m.outlinerEmptyMessage(perspective)
	b.WriteString(MutedStyle.Render(emptyMsg))
	if emptyMsg == EmptyStateNoTasks && m.project != nil &&
		m.SearchQuery() == "" && (m.Dispatcher == nil || !m.Dispatcher.HasPendingPrefix()) {
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("i add · : commands · ? help · esc back"))
	}
	if notice != "" {
		b.WriteString("\n\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
	}
	if m.IsSearchHelpVisible() {
		helpHeight := max(m.contentHeight()/2, 1)
		b.WriteString("\n")
		b.WriteString(renderSearchHelpPane(width, helpHeight))
	}

	return b.String()
}

func (m *OutlinerModel) renderPinnedTagTabBar(width int) string {
	tabs := m.pinnedTagTabs()
	if tabs == nil {
		return ""
	}
	m.activePinnedTagFilter()
	return RenderTabBar(tabs, m.pinnedTagTabIndex(), width, 1)
}

func (m *OutlinerModel) outlinerEmptyMessage(perspective Perspective) string {
	if m.project == nil && len(m.loadedProjects) == 0 {
		return "No projects in current context. Press C to switch context."
	}
	if m.workNextMode {
		return "Next is empty — press q to shortlist a task, Tab for Backlog, or P to pick a mode."
	}
	if m.workBacklogMode {
		return "Backlog is empty — everything actionable is already in Today or Next. Press Tab to switch work modes, or P for other modes."
	}
	if m.project == nil {
		return "No tasks found."
	}
	if m.smartFocus != nil {
		return fmt.Sprintf("No tasks match the %s focus. Try P to pick a perspective or F to adjust filters.", m.smartFocus.String())
	}
	if perspective.Name != "All" && len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if task := m.project.GetTaskByID(topID); task != nil {
			return fmt.Sprintf("No tasks match the %s perspective within %q. Try P to pick a perspective or h/l to cycle.", perspective.Name, task.Title)
		}
		return fmt.Sprintf("No tasks match the %s perspective. Try P to pick a perspective or h/l to cycle.", perspective.Name)
	}
	if perspective.Name != "All" {
		return fmt.Sprintf("No tasks match the %s perspective. Try P to pick a perspective or h/l to cycle.", perspective.Name)
	}
	if m.SearchQuery() != "" {
		return fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) || len(m.filter.Sections) > 0 || m.filter.Actionable {
		return EmptyStateFiltered
	}
	return EmptyStateNoTasks
}

func (m *OutlinerModel) computeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	visibleRows := max(m.contentHeight()-extraFooterLines, 1)

	if m.ac.IsActive() {
		acLines := min(len(m.ac.Items), acVisibleCount)
		if m.ac.ViewportTop > 0 {
			acLines++
		}
		if m.ac.ViewportTop+acVisibleCount < len(m.ac.Items) {
			acLines++
		}
		visibleRows = max(visibleRows-acLines, 1)
	}

	searchHelpHeight := 0
	if m.IsSearchHelpVisible() {
		searchHelpHeight = visibleRows / 2
		visibleRows = max(visibleRows-searchHelpHeight, 1)
	}

	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isTaskRow() {
		visibleRows = max(visibleRows-inlinePreviewLineCount(m.rows[m.cursor].task), 1)
	}

	startIdx, endIdx := persistentIndexWindow(m.cursor, len(m.rows), visibleRows, &m.viewScrollOffset)
	rowWidth := width
	if len(m.rows) > visibleRows {
		rowWidth = width - 1
	}

	return outlinerViewport{
		visibleRows:      visibleRows,
		searchHelpHeight: searchHelpHeight,
		startIdx:         startIdx,
		endIdx:           endIdx,
		rowWidth:         rowWidth,
	}
}

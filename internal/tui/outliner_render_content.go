package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

type outlinerViewport struct {
	visibleRows int
	startIdx    int
	endIdx      int
	rowWidth    int
}

// renderOutlinerTreeContent renders the full-width Outliner tree constrained to
// the given width.
func (m *OutlinerModel) renderOutlinerTreeContent(width int) string {
	notice := m.zoomNotice
	m.zoomNotice = ""

	modeLabel := m.buildModeLabel()
	breadcrumb := m.renderOutlinerBreadcrumb(width)
	searchIndicator := m.buildOutlinerSearchIndicator()

	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width, modeLabel, searchIndicator, breadcrumb, notice)
	}

	var b strings.Builder
	m.writeOutlinerHeader(&b, width, modeLabel, searchIndicator)

	// The title row is budgeted as a single line by contentHeight(), but the
	// multi-select badge / long filter suffix can push it past the render width
	// and wrap under the enclosing Width() style (notably the narrow tree pane
	// in split-pane mode). Reserve the extra wrapped rows so the viewport does
	// not overflow terminal height.
	titleExtra := wrappedLineCount(m.buildOutlinerTitleLine(width, modeLabel, searchIndicator), width) - 1

	extraFooter := 0
	if titleExtra > 0 {
		extraFooter += titleExtra
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

func (m *OutlinerModel) buildModeLabel() string {
	activeMode := m.activeOutlinerModeEntry()
	return "  " + GetOutlinerModeStyle(activeMode.ID).Render("◆ "+activeMode.Label)
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
		if m.primaryMode != ModeAll {
			prefix += ": " + m.activeOutlinerModeEntry().Label
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
		if m.activeOutlinerModeEntry().FixedOrder() {
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
// element (mode label, search indicator, multi-select badge) has been
// appended so the counter doesn't push them off the right edge. width is the
// available title-row budget.
func (m *OutlinerModel) appendOpenLoopsCounter(line string, width int) string {
	outlinerLoops := m.openLoops
	outlinerLoops.TodayDone = 0
	outlinerLoops.TodayTotal = 0
	counter := FormatOpenLoops(outlinerLoops)
	if counter == "" || width <= 0 {
		return line
	}
	rendered := MutedStyle.Render(counter)
	gap := width - lipgloss.Width(line) - lipgloss.Width(rendered)
	if gap <= 0 {
		return line
	}
	return line + strings.Repeat(" ", gap) + rendered
}

// buildOutlinerTitleLine assembles the single title row (title, mode
// label, search indicator, optional multi-select badge, right-aligned
// open-loops counter). Shared by writeOutlinerHeader and the height budget so
// both measure the same line.
func (m *OutlinerModel) buildOutlinerTitleLine(width int, modeLabel, searchIndicator string) string {
	var suffix strings.Builder
	suffix.WriteString(modeLabel)
	suffix.WriteString(searchIndicator)
	if m.multiSelectMode {
		suffix.WriteString("  ")
		suffix.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", m.SelectedCount())))
	}
	return m.composeOutlinerTitleRow(width, suffix.String())
}

// composeOutlinerTitleRow lays out the title row within width. The filter/sort
// badge and open-loops counter degrade before the title is truncated.
func (m *OutlinerModel) composeOutlinerTitleRow(width int, suffix string) string {
	budget := max(width, 1)
	suffix = truncateToVisualWidth(suffix, budget)
	titleWidth := max(budget-lipgloss.Width(suffix), 1)
	title := truncateToVisualWidth(m.renderOutlinerTitle(budget), titleWidth)
	return m.appendOpenLoopsCounter(title+suffix, budget)
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int, modeLabel, searchIndicator string) {
	b.WriteString(m.buildOutlinerTitleLine(width, modeLabel, searchIndicator))
	b.WriteString("\n")

	if tabBar := m.renderPinnedTagTabBar(width); tabBar != "" {
		b.WriteString(tabBar)
		b.WriteString("\n")
	}

	if breadcrumb := m.renderOutlinerBreadcrumb(width); breadcrumb != "" {
		b.WriteString(breadcrumb)
		b.WriteString("\n")
	}
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int, modeLabel, searchIndicator, breadcrumb, notice string) string {
	var b strings.Builder
	b.WriteString(m.composeOutlinerTitleRow(width, modeLabel+searchIndicator))
	b.WriteString("\n")

	if tabBar := m.renderPinnedTagTabBar(width); tabBar != "" {
		b.WriteString(tabBar)
		b.WriteString("\n")
	}

	if breadcrumb != "" {
		b.WriteString(breadcrumb)
		b.WriteString("\n")
	}

	emptyMsg := m.outlinerEmptyMessage()
	b.WriteString(MutedStyle.Render(emptyMsg))
	if emptyMsg == EmptyStateNoTasks && m.project != nil &&
		m.SearchQuery() == "" {
		b.WriteString("\n\n")
		b.WriteString(HelpStyle.Render("a capture · Tab modes · c scope · ? help"))
	}
	if notice != "" {
		b.WriteString("\n\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
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

func (m *OutlinerModel) outlinerEmptyMessage() string {
	if m.project == nil && len(m.loadedProjects) == 0 {
		return "No projects in this scope. Press c to choose another scope."
	}
	if m.primaryMode == ModeNext {
		return "Next is empty — press n on a task to add it or Tab for Backlog."
	}
	if m.primaryMode == ModeBacklog {
		return "Backlog is empty — no Ready tasks remain outside Today. Tab cycles modes."
	}
	if m.primaryMode == ModeInbox {
		return "Inbox is empty — press a to capture or Tab to continue."
	}
	if m.primaryMode != ModeAll && len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if task := m.project.GetTaskByID(topID); task != nil {
			return fmt.Sprintf("No tasks match %s within %q. Use Tab/Shift+Tab to change modes or Ctrl+Esc to reset.", m.activeOutlinerModeEntry().Label, task.Title)
		}
		return fmt.Sprintf("No tasks match %s. Use Tab/Shift+Tab to change modes or Ctrl+Esc to reset.", m.activeOutlinerModeEntry().Label)
	}
	if m.primaryMode != ModeAll {
		return fmt.Sprintf("No tasks match %s. Use Tab/Shift+Tab to change modes or Ctrl+Esc to reset.", m.activeOutlinerModeEntry().Label)
	}
	if m.SearchQuery() != "" {
		return fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
	}
	if hasActiveCommonFilters(&m.filter.CommonFilterState) || m.filter.Actionable {
		return EmptyStateFiltered
	}
	if m.project == nil {
		if mode := m.activeOutlinerModeEntry(); mode.ID != ModeAll {
			return fmt.Sprintf("%s is empty — use Tab/Shift+Tab for another mode or Ctrl+Esc to reset.", mode.Label)
		}
	}
	return EmptyStateNoTasks
}

func (m *OutlinerModel) computeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	visibleRows := max(m.contentHeight()-extraFooterLines, 1)

	if m.peekMode && m.cursor >= 0 && m.cursor < len(m.rows) && m.rows[m.cursor].isTaskRow() {
		// The shell footer may wrap to three lines at minimum width while views
		// retain the standard two-line size budget. Reserve one guard row so a
		// Peek attached to the last visible task is never the line shell clipping
		// removes.
		visibleRows = max(visibleRows-m.inlinePeekLineCount(m.rows[m.cursor], width-1)-1, 1)
	}

	startIdx, endIdx := persistentIndexWindow(m.cursor, len(m.rows), visibleRows, &m.viewScrollOffset)
	rowWidth := width
	if len(m.rows) > visibleRows {
		rowWidth = width - 1
	}

	return outlinerViewport{
		visibleRows: visibleRows,
		startIdx:    startIdx,
		endIdx:      endIdx,
		rowWidth:    rowWidth,
	}
}

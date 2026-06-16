package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
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
	notice := m.zoomNotice
	m.zoomNotice = ""

	perspective := m.currentPerspective()
	perspectiveLabel := m.buildPerspectiveLabel(perspective)
	breadcrumb := m.renderOutlinerBreadcrumb(width)
	searchIndicator := m.buildOutlinerSearchIndicator()

	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width, perspective, perspectiveLabel, searchIndicator, breadcrumb, notice)
	}

	var b strings.Builder
	m.writeOutlinerHeader(&b, width, perspectiveLabel, searchIndicator)

	extraFooter := 0
	if notice != "" {
		extraFooter += 2
	}
	if m.quickEdit.IsActive() {
		extraFooter += 2
	}
	if m.statusMessage != "" {
		extraFooter += 2
	}

	viewport := m.computeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport, width))

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
		b.WriteString(MutedStyle.Render(m.statusMessage))
		b.WriteString("\n")
	}

	return b.String()
}

func (m *OutlinerModel) buildPerspectiveLabel(perspective Perspective) string {
	presets := PerspectivePresets()
	perspCount := len(presets)
	prevIdx := (m.activePerspective + perspCount - 1) % perspCount
	nextIdx := (m.activePerspective + 1) % perspCount

	currentStyle := GetViewModeStyleForSection(perspective.Name)
	prevName := presets[prevIdx].Name
	nextName := presets[nextIdx].Name

	return "  " +
		MutedStyle.Render("‹ "+prevName) + " " +
		currentStyle.Render("@ "+perspective.Name) + " " +
		MutedStyle.Render(nextName+" ›")
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
		title := fmt.Sprintf("Cross-project outliner (%d project", projectCount)
		if projectCount != 1 {
			title += "s"
		}
		title += ")"
		return TitleStyle.Render(title) + buildTitleFilterSuffix(&TitleParams{
			FilterParts:   m.activeFilterParts(),
			SortField:     m.sort.Field,
			SortAscending: m.sort.Ascending,
			Width:         width,
		}, len(title))
	}

	titleParams := &TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     m.sort.Field,
		SortAscending: m.sort.Ascending,
		Width:         width,
	}
	return renderProjectTitle(m.project, m.config, titleParams)
}

// appendOpenLoopsCounter right-aligns the open-loops counter at the end of an
// already-assembled title line. Must be called after every inline title-row
// element (perspective label, search indicator, multi-select badge) has been
// appended so the counter doesn't push them off the right edge.
func (m *OutlinerModel) appendOpenLoopsCounter(line string, width int) string {
	counter := FormatOpenLoops(m.openLoops)
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

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int, perspectiveLabel, searchIndicator string) {
	var line strings.Builder
	line.WriteString(m.renderOutlinerTitle(width))
	line.WriteString(perspectiveLabel)
	line.WriteString(searchIndicator)
	if m.multiSelectMode {
		line.WriteString("  ")
		line.WriteString(SelectionCountStyle.Render(fmt.Sprintf("[MULTI-SELECT: %d selected]", m.SelectedCount())))
	}
	b.WriteString(m.appendOpenLoopsCounter(line.String(), width))
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
	if m.project == nil {
		return "No tasks found."
	}
	if perspective.Name != "All" && len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		if task := m.project.GetTaskByID(topID); task != nil {
			return fmt.Sprintf("No tasks match the %s perspective within %q. Try switching perspectives with shift+←/→.", perspective.Name, task.Title)
		}
		return fmt.Sprintf("No tasks match the %s perspective. Try switching perspectives with shift+←/→.", perspective.Name)
	}
	if perspective.Name != "All" {
		return fmt.Sprintf("No tasks match the %s perspective. Try switching perspectives with shift+←/→.", perspective.Name)
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
	visibleRows := m.contentHeight() - extraFooterLines

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

	if m.previewMode && m.cursor >= 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
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

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
	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width)
	}

	var b strings.Builder
	m.writeOutlinerHeader(&b, width)

	extraFooter := m.statusStripHeight(width)
	if m.quickEdit.IsActive() {
		extraFooter += m.quickEditExtraLineCount()
		if spec, ok := m.ContextualGuidance(); ok {
			extraFooter += max(lipgloss.Height(renderGuidance(spec, width))-2, 0)
		}
	}

	viewport := m.computeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport))

	return b.String()
}

// statusStripBody builds the focus notice and status message the shell pins
// above the footer. It is pure: the height reservation and the strip itself
// must measure the same string.
func (m *OutlinerModel) statusStripBody(width int) string {
	width = max(width, 1)
	lines := make([]string, 0, 2)
	if m.focusNotice != "" {
		lines = append(lines, PreviewWarningStyle.Width(width).Render(m.focusNotice))
	}
	if m.statusMessage != "" {
		lines = append(lines, MutedStyle.Width(width).Render(m.statusMessage))
	}
	return strings.Join(lines, "\n")
}

// statusStripHeight is the strip's divider plus its wrapped lines. The Outliner
// reserves it from its own row budget, so the strip takes rows from the list
// rather than pushing the footer off screen.
func (m *OutlinerModel) statusStripHeight(width int) int {
	body := m.statusStripBody(width)
	if body == "" {
		return 0
	}
	return 1 + lipgloss.Height(body)
}

// ViewStatusStrip hands the strip to the shell and consumes the one-shot focus
// notice. AppModel.View renders the view before the status bar, so the
// reservation above and this call see the same inputs in the same frame.
func (m *OutlinerModel) ViewStatusStrip(width int) string {
	body := m.statusStripBody(width)
	m.focusNotice = ""
	return body
}

func (m *OutlinerModel) outlinerFocusContext() string {
	if m.project == nil {
		return m.crossProjectFocusContext()
	}
	return m.projectFocusContext()
}

// buildOutlinerSearchIndicator renders the search chrome, appending a badge only
// when the title row can hold it: at narrow widths the query itself is the more
// useful half of the segment. A refused sigil wins over the case badge, because a
// query that cannot run explains an empty list better than how it was matched.
func (m *OutlinerModel) buildOutlinerSearchIndicator(width int) string {
	var indicator string
	switch {
	case m.IsSearchMode():
		indicator = MutedStyle.Render("Search: ") + m.SearchInputView()
	case m.SearchQuery() != "":
		indicator = MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	default:
		return ""
	}
	if guidance := m.searchGate.guidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.searchCaseRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

func appendSearchBadge(indicator, text string, width int) string {
	badge := MutedStyle.Render(text)
	if lipgloss.Width(indicator)+lipgloss.Width(badge) > width {
		return indicator
	}
	return indicator + badge
}

func (m *OutlinerModel) outlinerScopeLabel() string {
	if m.project == nil {
		if m.isProjectScoped() {
			return m.scopedProjectChromeLabel(m.scopedProjectID)
		}
		return m.baseScopeChromeLabel()
	}
	return m.project.Name
}

func (m *OutlinerModel) buildOutlinerFilterContext() string {
	sortField := m.sort.Field
	sortAscending := m.sort.Ascending
	if m.activeOutlinerModeEntry().FixedOrder() {
		sortField = SortByDefault
		sortAscending = false
	}
	suffix := buildTitleFilterSuffix(&TitleParams{
		FilterParts:   m.activeFilterParts(),
		SortField:     sortField,
		SortAscending: sortAscending,
	}, 0)
	return strings.TrimPrefix(suffix, "  ")
}

func (m *OutlinerModel) buildOutlinerInteractionContext(width int) []string {
	context := make([]string, 0, 4)
	if m.multiSelectMode {
		context = append(context, SelectionCountStyle.Render(fmt.Sprintf("MULTI-SELECT: %d selected", m.SelectedCount())))
	}
	if search := m.buildOutlinerSearchIndicator(width); search != "" {
		context = append(context, search)
	}
	if filter := m.buildOutlinerFilterContext(); filter != "" {
		context = append(context, filter)
	}
	if focus := m.outlinerFocusContext(); focus != "" {
		context = append(context, MutedStyle.Render(focus))
	}
	return context
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title.
func (m *OutlinerModel) buildOutlinerTitleLine(width int) string {
	var pace *todayPaceReadout
	if m.primaryMode == ModeToday && m.completionActivity.HasHistory() {
		pace = &todayPaceReadout{
			openCount:    m.workspacePulse.Today,
			medianPerDay: m.completionActivity.MedianPerDay(),
		}
	}
	return renderOperationalTitle(operationalTitleSpec{
		Scope:   TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:    operationalTitleModeSegment(m.activeOutlinerModeEntry()),
		Context: m.buildOutlinerInteractionContext(max(width, 1)),
		Pace:    pace,
		Pulse:   &m.workspacePulse,
	}, max(width, 1))
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int) string {
	var b strings.Builder
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")
	b.WriteString(MutedStyle.Render(m.outlinerEmptyMessage()))
	return b.String()
}

func (m *OutlinerModel) outlinerEmptyMessage() string {
	switch m.classifyEmptyCause() {
	case outlinerEmptyNoProjects:
		return "No projects are available. Press p to open Projects."
	case outlinerEmptyProjectFiltered:
		return "No projects match project filters. Press Fx to clear project filters."
	case outlinerEmptySearch:
		return fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
	case outlinerEmptySearchInvalid:
		return fmt.Sprintf("%s. Press Esc to clear.", m.searchGate.guidanceText())
	case outlinerEmptyFiltered:
		return EmptyStateFiltered
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. Press Esc to return."
	case outlinerEmptyMode:
		mode := m.activeOutlinerModeEntry()
		if mode.ID == ModeInbox {
			return "Inbox is empty. Press a to capture or Tab to change modes."
		}
		return fmt.Sprintf("%s is empty. Press Tab or Shift+Tab to change modes.", mode.Label)
	default:
		return EmptyStateNoTasks
	}
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

	m.lastViewport = outlinerViewport{
		visibleRows: visibleRows,
		startIdx:    startIdx,
		endIdx:      endIdx,
		rowWidth:    rowWidth,
	}
	return m.lastViewport
}

// visibleRowWindow returns the half-open row range the last render drew,
// clamped to the current row count. computeOutlinerViewport is the single
// window authority and mutates viewScrollOffset, so a key handler consumes its
// cached result here instead of calling it again.
func (m *OutlinerModel) visibleRowWindow() (int, int) {
	start, end := m.lastViewport.startIdx, m.lastViewport.endIdx
	if m.lastViewport.visibleRows <= 0 {
		start = m.viewScrollOffset
		end = start + m.contentHeight()
	}
	start = min(max(start, 0), len(m.rows))
	end = min(max(end, start), len(m.rows))
	return start, end
}

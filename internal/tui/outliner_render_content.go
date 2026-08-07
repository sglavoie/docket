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

	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width, notice)
	}

	var b strings.Builder
	m.writeOutlinerHeader(&b, width)

	extraFooter := 0
	if notice != "" {
		extraFooter += 2
	}
	if m.quickEdit.IsActive() {
		extraFooter += 1 + lipgloss.Height(m.renderQuickEditBar())
		if spec, ok := m.ContextualGuidance(); ok {
			extraFooter += max(lipgloss.Height(renderGuidance(spec, width))-2, 0)
		}
	}
	if m.statusMessage != "" {
		// One blank separator line plus the status text's own wrapped height.
		// A long status wraps to multiple lines at narrow widths, so reserving
		// a fixed 2 lines under-counts and overflows terminal height.
		extraFooter += 1 + max(wrappedLineCount(m.statusMessage, width), 1)
	}

	viewport := m.computeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport))

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

func (m *OutlinerModel) outlinerZoomContext() string {
	if m.project == nil {
		return m.crossProjectZoomContext()
	}
	return m.projectZoomContext()
}

func (m *OutlinerModel) buildOutlinerSearchIndicator() string {
	if m.IsSearchMode() {
		return MutedStyle.Render("Search: ") + m.SearchInputView()
	}
	if m.SearchQuery() != "" {
		return MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	}
	return ""
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

func (m *OutlinerModel) buildOutlinerInteractionContext() []string {
	context := make([]string, 0, 4)
	if m.multiSelectMode {
		context = append(context, SelectionCountStyle.Render(fmt.Sprintf("MULTI-SELECT: %d selected", m.SelectedCount())))
	}
	if search := m.buildOutlinerSearchIndicator(); search != "" {
		context = append(context, search)
	}
	if filter := m.buildOutlinerFilterContext(); filter != "" {
		context = append(context, filter)
	}
	if zoom := m.outlinerZoomContext(); zoom != "" {
		context = append(context, MutedStyle.Render(zoom))
	}
	return context
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title.
func (m *OutlinerModel) buildOutlinerTitleLine(width int) string {
	return renderOperationalTitle(operationalTitleSpec{
		Surface: TitleStyle.Render("Outliner"),
		Scope:   TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:    operationalTitleModeSegment(m.activeOutlinerModeEntry()),
		Context: m.buildOutlinerInteractionContext(),
		Pulse:   &m.workspacePulse,
	}, max(width, 1))
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int, notice string) string {
	var b strings.Builder
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")

	emptyMsg := m.outlinerEmptyMessage()
	b.WriteString(MutedStyle.Render(emptyMsg))
	if notice != "" {
		b.WriteString("\n\n")
		b.WriteString(PreviewWarningStyle.Render(notice))
	}
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
	case outlinerEmptyFiltered:
		return EmptyStateFiltered
	case outlinerEmptyZoom:
		return "No tasks are visible in this zoom. Press Esc to zoom out."
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

	return outlinerViewport{
		visibleRows: visibleRows,
		startIdx:    startIdx,
		endIdx:      endIdx,
		rowWidth:    rowWidth,
	}
}

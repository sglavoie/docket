package tui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/pulse"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/theme"
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
	// An open menu hangs off the label it was opened from, so it is drawn before
	// anything else on the frame — and it is the whole frame. The branch is keyed
	// on the drawn block rather than on the menuBar sub-state's gate, because the
	// dropdown returns
	// nothing for a width or a resolution it cannot draw, and blanking the view
	// over a menu nobody can see would be the worse failure.
	if menu := m.renderOutlinerMenuDropdown(width); menu != "" {
		return m.renderOutlinerMenuFrame(width, menu)
	}
	if len(m.rows) == 0 {
		return m.renderOutlinerEmptyState(width)
	}

	var b strings.Builder
	// The bar leads, so the title row and its workspace pulse simply sit one line
	// down rather than sharing a row with chrome. Its cost is already out of the
	// budget contentHeight reports, which is why nothing is measured here.
	if m.menuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	m.writeOutlinerHeader(&b, width)

	// Render the pane once and measure what came back: RenderDropdownWithLimit
	// decides for itself how much of its budget goes to overflow hints, so a
	// predicted height and a drawn one would drift apart.
	dropdown := m.renderSearchAutocompleteDropdown(width)

	extraFooter := m.statusStripHeight(width)
	if dropdown != "" {
		b.WriteString(dropdown + "\n")
		extraFooter += lipgloss.Height(dropdown)
	}
	if m.quickEdit.IsActive() {
		extraFooter += m.quickEditExtraLineCount()
		if spec, ok := m.ContextualGuidance(); ok {
			extraFooter += max(lipgloss.Height(chrome.RenderGuidance(spec, width))-2, 0)
		}
	}

	viewport := m.computeOutlinerViewport(width, extraFooter)
	b.WriteString(m.renderOutlinerRows(viewport))

	return b.String()
}

// statusStripBody builds the outstanding-conflict indicator, the focus notice,
// the status message, and the expanded query readout the shell pins above the
// footer. It is pure: the height reservation and the strip itself must measure
// the same string.
//
// The indicator is an ambient advisory, so it is gated by advisoryLine: hiding
// it drops the row it reserved too, because the reservation measures this same
// body.
//
// The indicator leads because it is the one durable line here — it stands until
// the CLI resolves the last bundle, while the lines below it come and go — so a
// transient line arriving never shifts it to another row. The readout sits last
// for the same reason from the other end: it is the tallest thing the strip can
// hold, and growing it must not push the three one-line signals around.
func (m *OutlinerModel) statusStripBody(width int) string {
	width = max(width, 1)
	lines := make([]string, 0, 4)
	if indicator := m.advisoryLine(m.conflictIndicator()); indicator != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(indicator))
	}
	if m.focusNotice != "" {
		lines = append(lines, theme.PreviewWarningStyle.Width(width).Render(m.focusNotice))
	}
	if m.StatusMessage != "" {
		lines = append(lines, theme.MutedStyle.Width(width).Render(m.StatusMessage))
	}
	// The expanded stack changes this return value rather than painting outside
	// it, so statusStripHeight reserves the rows it needs from the list.
	if readout := m.queryReadoutBody(width); readout != "" {
		lines = append(lines, readout)
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
	case m.editingLensQuery():
		// The lens badge reads the lens compilation rather than the search one,
		// because those are the rows on screen.
		return m.appendLensQueryBadge(theme.MutedStyle.Render("Lens query: ")+m.SearchInputView(), width)
	case m.hasActiveLens() && m.lensQuery != "":
		return m.appendLensQueryBadge(theme.MutedStyle.Render(fmt.Sprintf("Lens query: %q", m.lensQuery)), width)
	case m.IsSearchMode():
		indicator = theme.MutedStyle.Render("Search: ") + m.SearchInputView()
	case m.SearchQuery() != "":
		indicator = theme.MutedStyle.Render(fmt.Sprintf("Search: \"%s\"", m.SearchQuery()))
	default:
		return ""
	}
	if guidance := m.searchGate.GuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.searchCaseRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

// appendLensQueryBadge gives the lens query the badges ordinary search already
// uses, read off the lens compilation: a refused sigil wins over the case badge
// there for the same reason it does in search.
func (m *OutlinerModel) appendLensQueryBadge(indicator string, width int) string {
	if guidance := m.lensGuidanceText(); guidance != "" {
		return appendSearchBadge(indicator, " "+guidance, width)
	}
	if !m.lensNarrowRelaxed {
		return indicator
	}
	return appendSearchBadge(indicator, " (case ignored)", width)
}

func appendSearchBadge(indicator, text string, width int) string {
	badge := theme.MutedStyle.Render(text)
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
	// A fixed-order mode owns its ordering, so a sort recorded there is not on
	// screen and must not be advertised. Under a search projection the mode
	// underneath is irrelevant — the sort is live over the ranked rows — so
	// zeroing the segment there would silently hide it.
	if m.activeOutlinerModeEntry().FixedOrder() && !m.showsRankedList() {
		sortField = query.SortByDefault
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
	context := make([]string, 0, 5)
	if m.MultiSelectMode {
		context = append(context, theme.SelectionCountStyle.Render(fmt.Sprintf("MULTI-SELECT: %d selected", m.SelectedCount())))
	}
	// The lens names the whole context the segments after it describe, so it is
	// read first and its marker states whether they still match what was saved.
	if lens := m.activeLensLabel(); lens != "" {
		context = append(context, theme.MutedStyle.Render(lens))
	}
	if search := m.buildOutlinerSearchIndicator(width); search != "" {
		context = append(context, search)
	}
	if filter := m.buildOutlinerFilterContext(); filter != "" {
		context = append(context, filter)
	}
	if focus := m.outlinerFocusContext(); focus != "" {
		context = append(context, theme.MutedStyle.Render(focus))
	}
	// Last, so that the one joined Context region loses the zoom before it loses
	// a narrowing the user cannot leave with a single Esc.
	if m.zoom.active() && m.zoom.name != "" {
		context = append(context, theme.MutedStyle.Render("Section: "+m.zoom.name))
	}
	return context
}

// buildOutlinerTitleLine assembles the Outliner's one-row operational title.
func (m *OutlinerModel) buildOutlinerTitleLine(width int) string {
	var pace *pulse.TodayPaceReadout
	if m.primaryMode == chrome.ModeToday && m.CompletionActivity.HasHistory() {
		pace = &pulse.TodayPaceReadout{
			OpenCount:    m.workspacePulse.Today,
			MedianPerDay: m.CompletionActivity.MedianPerDay(),
		}
	}
	return pulse.RenderOperationalTitle(pulse.OperationalTitleSpec{
		Scope:   theme.TitleStyle.Render(m.outlinerScopeLabel()),
		Mode:    chrome.OperationalTitleModeSegment(m.activeOutlinerModeEntry()),
		Context: m.buildOutlinerInteractionContext(max(width, 1)),
		Pace:    pace,
		Pulse:   &m.workspacePulse,
	}, max(width, 1))
}

func (m *OutlinerModel) writeOutlinerHeader(b *strings.Builder, width int) {
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")
}

// renderOutlinerMenuFrame is the whole frame while a menu is open: the bar row
// and the block hanging off the open label, and nothing under them. The list is
// not shrunk to make room, it is not drawn at all, so opening and closing a menu
// moves no row on screen — the rows behind a dropdown were unreadable anyway,
// and a list that shifted twice per menu press was the cost of pretending
// otherwise.
//
// Nothing here calls computeOutlinerViewport, deliberately. It is the single
// window authority and the only writer of ViewScrollOffset, so a frame that
// draws no rows must leave the window the last one drew exactly as it was: that
// is the window the close redraws, and the window a key re-emitted through the
// non-modal arm pages against.
func (m *OutlinerModel) renderOutlinerMenuFrame(width int, menu string) string {
	var b strings.Builder
	b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	b.WriteString(menu)
	return b.String()
}

func (m *OutlinerModel) renderOutlinerEmptyState(width int) string {
	var b strings.Builder
	// The empty state carries the bar for the same reason it carries the
	// suggestion pane: a view with no rows is exactly where a user reaches for the
	// map of what they can do next. An open menu never reaches here — it is its
	// own frame, drawn before the row count is even consulted.
	if m.menuBarVisible() {
		b.WriteString(m.renderOutlinerMenuBar(width) + "\n")
	}
	b.WriteString(m.buildOutlinerTitleLine(width))
	b.WriteString("\n")
	// A query that matches nothing yet is exactly when suggestions are worth the
	// most, so the empty state carries the pane too.
	if dropdown := m.renderSearchAutocompleteDropdown(width); dropdown != "" {
		b.WriteString(dropdown + "\n")
	}
	b.WriteString(theme.MutedStyle.Render(m.outlinerEmptyMessage()))
	return b.String()
}

func (m *OutlinerModel) outlinerEmptyMessage() string {
	switch m.classifyEmptyCause() {
	case outlinerEmptyNoProjects:
		return "No projects are available. Press P to open Projects."
	case outlinerEmptyProjectFiltered:
		return "No projects match project filters. Press Fx to clear project filters."
	case outlinerEmptySearch:
		return fmt.Sprintf("No tasks match search %q. Press Esc to clear.", m.SearchQuery())
	case outlinerEmptySearchFiltered:
		return fmt.Sprintf(
			"No tasks match search %q under the active filters. Press fx to clear filters or Esc to clear the search.",
			m.SearchQuery())
	case outlinerEmptySearchInvalid:
		return fmt.Sprintf("%s. Press Esc to clear.", m.searchGate.GuidanceText())
	case outlinerEmptyLens:
		return "No tasks match this lens. Press Esc to restore the view it replaced."
	case outlinerEmptyFiltered:
		return listsurface.EmptyStateFiltered
	case outlinerEmptyFocus:
		return "No tasks are visible in this Focus. Press Esc to return."
	case outlinerEmptySection:
		return "No tasks are left in this section. Press Esc to return."
	case outlinerEmptyMode:
		mode := m.activeOutlinerModeEntry()
		if mode.ID == chrome.ModeInbox {
			return "Inbox is empty. Press a to capture or Tab to change modes."
		}
		return fmt.Sprintf("%s is empty. Press Tab or Shift+Tab to change modes.", mode.Label)
	default:
		return listsurface.EmptyStateNoTasks
	}
}

func (m *OutlinerModel) computeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	visibleRows := max(m.contentHeight()-extraFooterLines, 1)

	if m.peekMode && m.Cursor >= 0 && m.Cursor < len(m.rows) && m.rows[m.Cursor].isTaskRow() {
		// The shell footer may wrap to three lines at minimum width while views
		// retain the standard two-line size budget. Reserve one guard row so a
		// Peek attached to the last visible task is never the line shell clipping
		// removes.
		visibleRows = max(visibleRows-m.inlinePeekLineCount(m.rows[m.Cursor], width-1)-1, 1)
	}

	startIdx, endIdx := chrome.PersistentIndexWindow(m.Cursor, len(m.rows), visibleRows, &m.ViewScrollOffset)
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
// window authority and mutates ViewScrollOffset, so a key handler consumes its
// cached result here instead of calling it again.
func (m *OutlinerModel) visibleRowWindow() (int, int) {
	start, end := m.lastViewport.startIdx, m.lastViewport.endIdx
	if m.lastViewport.visibleRows <= 0 {
		start = m.ViewScrollOffset
		end = start + m.contentHeight()
	}
	start = min(max(start, 0), len(m.rows))
	end = min(max(end, start), len(m.rows))
	return start, end
}

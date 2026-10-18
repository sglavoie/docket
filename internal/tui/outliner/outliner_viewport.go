package outliner

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/chrome"
)

// ComputeOutlinerRowLayout is the one measurement of a projection's shared row
// geometry: the fold column and the badge columns the leading segments align in.
// It reads the whole
// row set it is handed rather than the slice a window happens to show, which is
// what keeps the titles where they are while the list scrolls — a measurement
// taken from the window would move every row's title each time a wider badge
// scrolled into view.
//
// It is the seam every task-row consumer takes: the Outliner's own render, the
// inline editor that has to open in the column the title was drawn in, and the
// Hub's preview painter, which is a standalone render of the same rows. Badge
// alignment keeps its width floor, because padding a badge is a wide-terminal
// nicety that a cramped row is better off without.
func (m *OutlinerModel) ComputeOutlinerRowLayout(rows []OutlinerRow, rowWidth int) *chrome.ColumnLayout {
	return m.computeRowLayout(rows, rowWidth, nil)
}

// computeRowLayout measures the drawn rows together with what the projection's
// folds hide. The Outliner's own render hands it the model's accumulator, so a
// fold opening or closing moves no title; every other caller measures the rows
// it holds and nothing else. A projection drawing no task row still measures
// nothing, since there is no row for a hidden one to align with.
func (m *OutlinerModel) computeRowLayout(rows []OutlinerRow, rowWidth int, hidden *hiddenRowLayout) *chrome.ColumnLayout {
	if rowWidth < chrome.MinColumnAlignWidth {
		return nil
	}
	tasks := make([]*models.Task, 0, len(rows))
	hasFold := false
	var slugs map[*models.Task]string
	for _, row := range rows {
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		tasks = append(tasks, row.Task)
		hasFold = hasFold || rowDrawsFold(row)
		if m.rowLeadsWithProjectLabel(row) {
			if slugs == nil {
				slugs = make(map[*models.Task]string, len(rows))
			}
			slugs[row.Task] = m.rankedProjectSlug(row, rowWidth)
		}
	}
	noteWidth := 0
	if m.showsRankedProjectLabel() {
		noteWidth = m.noteLabelWidth(rows, rowWidth)
	}
	if len(tasks) == 0 {
		if noteWidth == 0 {
			return nil
		}
		return &chrome.ColumnLayout{ProjectWidth: noteWidth}
	}
	if hidden != nil {
		tasks = append(tasks, hidden.tasks...)
		hasFold = hasFold || hidden.hasFold
	}
	opts := chrome.BadgeWidthOpts{IncludeStatus: true}
	if slugs != nil {
		opts.ProjectNameFor = func(task *models.Task) string { return slugs[task] }
		opts.MaxProjectLen = rankedProjectLabelMaxSlug(rowWidth)
	}
	layout := chrome.ComputeBadgeWidths(tasks, opts)
	if layout != nil {
		layout.HasFold = hasFold
		layout.ProjectWidth = max(layout.ProjectWidth, noteWidth)
	}
	return layout
}

// noteLabelWidth is the widest label the notes tier draws, folded into the
// task rows' project column so a note title starts where a task title does. A
// result set holding only notes still measures it, which is why the layout is
// not nil there.
func (m *OutlinerModel) noteLabelWidth(rows []OutlinerRow, rowWidth int) int {
	width := 0
	for _, row := range rows {
		if !row.IsNoteRow {
			continue
		}
		entry, ok := m.Search.noteMatches[row.NoteRow]
		if !ok {
			continue
		}
		if slug, _, _ := m.noteRowSlug(entry.row, rowWidth); slug != "" {
			width = max(width, lipgloss.Width("["+slug+"]"))
		}
	}
	return width
}

type outlinerViewport struct {
	VisibleRows int
	startIdx    int
	endIdx      int
	rowWidth    int
	// pinned reports that the list's first screen row is the pinned context
	// row rather than a row of the window, and pinnedHeader is the section
	// header it stands in for. VisibleRows still counts that screen row, so
	// startIdx and endIdx then span one row fewer.
	pinned       bool
	pinnedHeader int
	// topRow is the screen row the window's first row is drawn on, below the
	// header rows and any search dropdown, so a cursor row can be named on
	// screen without a second window computation.
	topRow int
}

// drawnRows is the screen rows the window fills: its own rows and the pinned
// context row above them.
func (v outlinerViewport) drawnRows() int {
	drawn := v.endIdx - v.startIdx
	if v.pinned {
		drawn++
	}
	return drawn
}

// outlinerListRows is the height of the block between the header chrome and the
// dock, and it is one definition because both projections have to measure
// against it: the populated path fills it with rows and pads the remainder, and
// the empty path fills it with its header and its explanation and pads the same
// way. That is what puts the dock on one screen row whichever path drew the
// frame. It is pure — the dock's cost is the frame's answer and the rest is the
// frame — so nothing here reads the cursor or the document.
func (m *OutlinerModel) outlinerListRows(extraFooterLines int) int {
	// Clamp to at least one row: at pathologically small heights (e.g. the
	// dimmed project-cycle/pinned preview backgrounds sized to a couple of
	// lines) contentHeight()-extraFooterLines can go non-positive, which would
	// drive a negative window slice.
	//
	// The shell's footer is charged here, once, from what it measured rather than
	// from what this view can guess about it: a footer carrying feedback beside
	// its controls, or controls wrapped at minimum width, is taller than the two
	// rows Height() was sized against, and the difference is the list's to pay.
	// Nothing below re-measures it.
	geometry := m.peekDockGeometry()
	reserved := m.ShellFrame.ExcessRows()
	if geometry.Displayed() {
		// While the dock is open that charge has a floor, and the floor is what
		// keeps the dock on one screen row. The dock's own guidance is the one
		// footer content this view's cursor decides — `Ctrl+D more` is advertised
		// over a document that can move and withheld over one that cannot — so at
		// minimum width the footer wraps for one row and not for the next, and a
		// list following that difference would slide the dock under the cursor.
		// One row is paid whether or not this row's document pages; a footer that
		// genuinely costs more than that is still charged in full. The dock's own
		// availability arithmetic charges the same reservation, so the two cannot
		// disagree about what the footer costs.
		reserved = m.peekDockFooterReservation()
	}
	rows := max(m.ContentHeight()-extraFooterLines-reserved, 1)
	// The dock's height answers to the frame alone, so the list loses the same
	// rows whatever the cursor is resting on: opening Peek moves the list once,
	// and nothing the cursor does afterwards moves it again.
	return max(rows-geometry.TotalRows(), 1)
}

// feedbackBudgetAnchor holds a temporary footer loan only while the user and
// projection remain at the position where it started. A rebuild, key, or
// resize cancels the loan explicitly.
type feedbackBudgetAnchor struct {
	window hostBudgetAnchor
	cursor int
}

// SetShellFrameMetrics repays a feedback row when the message expires. This
// reverses the viewport's automatic clamp, without undoing intervening input.
func (m *OutlinerModel) SetShellFrameMetrics(metrics chrome.ShellFrameMetrics) {
	if m.ShellFrame.FeedbackRows == 0 && metrics.FeedbackRows > 0 && len(m.Rows) > 0 {
		m.feedbackBudget = feedbackBudgetAnchor{window: m.captureHostBudgetAnchor(), cursor: m.Cursor}
	}
	if metrics.FeedbackRows == 0 {
		held := m.feedbackBudget
		if held.window.active && len(m.Rows) > 0 && held.cursor == m.Cursor {
			m.seatWindowAtAnchor(held.window)
		}
		m.feedbackBudget = feedbackBudgetAnchor{}
	}
	m.ShellFrame = metrics
}

func (m *OutlinerModel) ComputeOutlinerViewport(width int, extraFooterLines int) outlinerViewport {
	visibleRows := m.outlinerListRows(extraFooterLines)

	window := m.pinnedContextWindow(visibleRows)
	window.VisibleRows = visibleRows
	window.rowWidth = width
	if len(m.Rows) > visibleRows {
		window.rowWidth = width - 1
	}

	m.LastViewport = window
	return m.LastViewport
}

// pinnedContextWindow decides the row window and whether its first screen row
// is spent on the pinned context row. A window that opens inside a lane whose
// header scrolled above it gives that row up and windows one row fewer, through
// the same PersistentIndexWindow call over the smaller height, so the cursor is
// kept visible beneath the pinned row by the rule that keeps it visible
// anywhere. Both heights are tried against the stored offset rather than one
// against the other's result, so a frame that changes nothing writes the offset
// it found.
//
// The remaining case is the smaller window opening exactly on a header: nothing
// is left to pin, so the row is given back to the window, which then starts on
// that header. At the end of the list that window would come up a row short, and
// there the full window stands instead, unpinned, since a blank row would hide a
// row to name a lane one line above it.
func (m *OutlinerModel) pinnedContextWindow(visibleRows int) outlinerViewport {
	window, offset := m.pinnedContextWindowAt(m.ViewScrollOffset, visibleRows)
	m.ViewScrollOffset = offset
	return window
}

// pinnedContextWindowAt is that decision over a given offset, returning the
// offset the frame settles on instead of writing it. It is what lets the scroll
// chord ask which window an offset would draw before it stores one: whether the
// first screen row is pinned is a property of the destination, not of the frame
// the key was pressed on.
func (m *OutlinerModel) pinnedContextWindowAt(offset, visibleRows int) (outlinerViewport, int) {
	full := offset
	startIdx, endIdx := chrome.PersistentIndexWindow(m.Cursor, len(m.Rows), visibleRows, &full)
	if _, scrolledPast := m.contextHeaderAbove(startIdx); !scrolledPast || visibleRows < 2 {
		return outlinerViewport{startIdx: startIdx, endIdx: endIdx}, full
	}
	fullWindow := outlinerViewport{startIdx: startIdx, endIdx: endIdx}
	reduced := offset
	startIdx, endIdx = chrome.PersistentIndexWindow(m.Cursor, len(m.Rows), visibleRows-1, &reduced)
	header, scrolledPast := m.contextHeaderAbove(startIdx)
	if !scrolledPast && startIdx+visibleRows > len(m.Rows) {
		return fullWindow, full
	}
	if !scrolledPast {
		return outlinerViewport{startIdx: startIdx, endIdx: startIdx + visibleRows}, reduced
	}
	return outlinerViewport{startIdx: startIdx, endIdx: endIdx, pinned: true, pinnedHeader: header}, reduced
}

// contextHeaderAbove also covers modes with project groups but no lane headers.
// Ranked results already label every row; scoped views name their project in
// the title. Neither needs another context row.
func (m *OutlinerModel) contextHeaderAbove(index int) (int, bool) {
	if header, ok := m.sectionHeaderAbove(index); ok {
		return header, true
	}
	if index <= 0 || index >= len(m.Rows) || !m.groupsRowsByProject() || m.ShowsRankedList() {
		return 0, false
	}
	group := m.Rows[index].Group
	if _, hasLane := m.sectionHeaderRows[group.SectionKey]; hasLane {
		return 0, false
	}
	header, ok := m.projectHeaderRows[group.ProjectKey]
	if !ok || header >= index || header < 0 || header >= len(m.Rows) {
		return 0, false
	}
	row := m.Rows[header]
	return header, (row.IsProjectHeader || row.LeadsMergedGroup) && row.Group == group
}

// sectionHeaderAbove answers whether the row at index sits in a lane whose
// header is above it, and where that header is. It reads the row's own group
// and the index the projection recorded, never the rows between them. Either
// single-project form answers no, as does a row carrying no section — the
// ranked list, a separator, a projection that emits no lanes — and an index the
// rows no longer bear out is not trusted.
func (m *OutlinerModel) sectionHeaderAbove(index int) (int, bool) {
	if index <= 0 || index >= len(m.Rows) || !m.groupsRowsByProject() {
		return 0, false
	}
	key := m.Rows[index].Group.SectionKey
	if key == "" {
		return 0, false
	}
	header, ok := m.sectionHeaderRows[key]
	if !ok || header >= index {
		return 0, false
	}
	if row := m.Rows[header]; !row.IsSectionHeader || row.Group.SectionKey != key {
		return 0, false
	}
	return header, true
}

// visibleRowWindow returns the half-open row range the last render drew,
// clamped to the current row count. computeOutlinerViewport is the single
// window authority and mutates ViewScrollOffset, so a key handler consumes its
// cached result here instead of calling it again.
func (m *OutlinerModel) VisibleRowWindow() (int, int) {
	start, end := m.LastViewport.startIdx, m.LastViewport.endIdx
	if m.LastViewport.VisibleRows <= 0 {
		start = m.ViewScrollOffset
		end = start + m.ContentHeight()
	}
	start = min(max(start, 0), len(m.Rows))
	end = min(max(end, start), len(m.Rows))
	return start, end
}

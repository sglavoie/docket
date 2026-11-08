package outliner

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/taskref"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// queryReadoutBody renders the expanded stack, or the empty string when it is
// collapsed. It is pure, because statusStripHeight measures exactly this string
// before statusStripBody paints it: the window is computed from the stored
// offset rather than written back, the way a Peek render reads peekOffsetForRow
// without moving it.
func (m *OutlinerModel) QueryReadoutBody(width int) string {
	if m == nil || !m.QueryReadout.Expanded {
		return ""
	}
	width = max(width, 1)
	heading := theme.TitleStyle.Render(textfit.TruncateToVisualWidth("Narrowing this view", width))
	document, window, overflows := m.queryReadoutWindow(width)
	if !overflows {
		return strings.Join(append([]string{heading}, document...), "\n")
	}
	offset := clampQueryReadoutOffset(m.QueryReadout.Offset, len(document), window)
	lines := append([]string{heading}, document[offset:offset+window]...)
	return strings.Join(append(lines,
		queryReadoutPositionLine(offset, len(document)-offset-window, width)), "\n")
}

// queryReadoutWindow is the one measurement of the readout: the whole document,
// how many of its lines the window draws, and whether anything is off it.
// Render, every scroll write, and the footer all read it, so none of them can
// disagree about where the window ends.
func (m *OutlinerModel) queryReadoutWindow(width int) (document []string, window int, overflows bool) {
	document = m.queryReadoutDocument(width)
	window = m.queryReadoutBodyRows(width)
	if len(document) <= window {
		return document, window, false
	}
	// One row goes to the position line, which is what makes a window that hides
	// something say so instead of ending silently.
	return document, max(window-1, 1), true
}

// queryReadoutBodyRows is the readout's budget: the view's own rows, less what
// the footer overran, less the dock, less the strip's signals above it, less
// this readout's chrome, and less the one outline row the readout may never
// take — it is an expansion of the view, not a replacement for it.
//
// It cannot ask statusStripHeight for the strip's cost, because the height that
// call would answer with is the one this function is still deciding. For the
// same reason it charges the dock's frame-preferred height rather than the
// height the dock settles on: the settled height is constrained by this strip,
// so asking for it here would ask this function for its own answer. Preferred is
// never smaller than settled, so the charge is conservative in the one direction
// that cannot overflow the frame.
func (m *OutlinerModel) queryReadoutBodyRows(width int) int {
	dock := m.peekDockPreferredRows()
	reserved := m.ShellFrame.ExcessRows()
	if dock > 0 {
		reserved = m.peekDockFooterReservation()
	}
	budget := m.ContentHeight() - reserved - dock - m.statusStripSignalRows(width) - queryReadoutChromeRows
	return max(budget, queryReadoutMinBodyRows)
}

// queryReadoutDocument is the whole readout below its heading, wrapped to width
// and styled: the peelable stack, then the axes the readout reports but nothing
// peels. It is complete — a value is wrapped rather than cut — because this is
// where a context row that shortened something says what it shortened.
func (m *OutlinerModel) queryReadoutDocument(width int) []string {
	rows := m.QueryReadoutRows()
	lines := make([]string, 0, len(rows)+6)
	if len(rows) == 0 {
		lines = appendQueryReadoutLines(lines, theme.MutedStyle, width, "nothing: every axis is at its default")
	} else {
		labelWidth := 0
		for _, row := range rows {
			labelWidth = max(labelWidth, lipgloss.Width(row.Label))
		}
		for index, row := range rows {
			// The digits are the peel vocabulary rather than a numbering of what is
			// on screen, so they stay attached to the first nine axes wherever the
			// window sits, and a tenth axis is listed without one: scrolling reaches
			// it, and no new binding is minted to press it.
			marker := "  "
			if index < queryReadoutRowLimit {
				marker = strconv.Itoa(index+1) + " "
			}
			text := fmt.Sprintf("%s%-*s %s", marker, labelWidth, row.Label, row.value)
			if row.note != "" {
				text += "  (" + row.note + ")"
			}
			lines = appendQueryReadoutLines(lines, theme.MutedStyle, width, text)
		}
	}
	return append(lines, m.queryReadoutDetailLines(width)...)
}

// queryReadoutDetailLines lists the axes that are live but peel to nothing,
// read off the same structured description the context row draws from rather
// than out of the row's rendered text. They are last and they carry no digit,
// because a digit is a promise that pressing it takes the axis away.
func (m *OutlinerModel) queryReadoutDetailLines(width int) []string {
	entries := make([]ViewContextEntry, 0, 4)
	labelWidth := 0
	for _, entry := range m.BuildViewContextEntries(width) {
		if !entry.ReadOnly {
			continue
		}
		entries = append(entries, entry)
		labelWidth = max(labelWidth, lipgloss.Width(entry.Label))
	}
	if len(entries) == 0 {
		return nil
	}
	lines := appendQueryReadoutLines(nil, theme.TitleStyle, width, "Also in view")
	for _, entry := range entries {
		lines = appendQueryReadoutLines(lines, theme.MutedStyle, width,
			fmt.Sprintf("  %-*s %s", labelWidth, entry.Label, entry.Value))
	}
	return lines
}

// appendQueryReadoutLines wraps one entry to the readout's width and styles
// every line it became. Wrapping rather than truncating is the whole point of
// the bounded viewport: the readout is where a value the header shortened is
// readable in full.
func appendQueryReadoutLines(lines []string, style lipgloss.Style, width int, text string) []string {
	for _, wrapped := range textfit.WrapText(text, width) {
		lines = append(lines, style.Render(wrapped))
	}
	return lines
}

// queryReadoutPositionLine spends the reserved row on exact counts for the same
// reason Peek's does: what a reader needs is how much is still unread in each
// direction, and a page number only answers that after arithmetic.
func queryReadoutPositionLine(above, below, width int) string {
	return theme.MutedStyle.Render(textfit.TruncateToVisualWidth(
		fmt.Sprintf("%d above · %d below", above, below), width))
}

// clampQueryReadoutOffset is the one bound on the window's position. It is pure
// and it is applied at render as well as at every write, so a peel that
// shortened the document or a resize that shortened the window re-seats the
// window without anything having to notice that either happened.
func clampQueryReadoutOffset(offset, total, window int) int {
	return min(max(offset, 0), max(total-window, 0))
}

// scrollQueryReadout moves the window and nothing else. The outline's cursor,
// its own scroll offset, and the selection are all untouched by every key the
// readout claims, which is what lets it borrow j/k and the paging keys while it
// is open without the list moving underneath it.
//
// A relative move is measured from the line the reader can see, not from the
// stored offset, and those two part company whenever the window or the document
// grew since the last write: the render clamps without writing back, on purpose,
// so a taller terminal leaves an offset past the new maximum standing. Adding
// the delta to that stale number would spend the first k normalizing it and the
// screen would not move, and would shorten a PgUp by however far the offset
// overran. Clamping first is what makes every relative key travel its full
// distance from what is drawn.
func (m *OutlinerModel) scrollQueryReadout(delta int) {
	document, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	visible := clampQueryReadoutOffset(m.QueryReadout.Offset, len(document), window)
	m.QueryReadout.Offset = clampQueryReadoutOffset(visible+delta, len(document), window)
}

// scrollQueryReadoutTo seats the window at an absolute line, clamped. The two
// document bounds are spelled as offsets rather than as their own writes, so
// Home and End answer through the same clamp every other scroll takes.
func (m *OutlinerModel) scrollQueryReadoutTo(offset int) {
	document, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	m.QueryReadout.Offset = clampQueryReadoutOffset(offset, len(document), window)
}

// queryReadoutPageRows is one page of the readout: whatever the window is
// currently showing, so a page is always exactly what is on screen.
func (m *OutlinerModel) queryReadoutPageRows() int {
	_, window, _ := m.queryReadoutWindow(max(m.Width(), 1))
	return max(window, 1)
}

// queryReadoutOverflows is whether the footer should advertise the scroll keys.
// They are claimed whether or not the document moves — the readout owns input
// outright — but naming them over a document that fits would promise a scroll
// there is nothing to do.
func (m *OutlinerModel) queryReadoutOverflows() bool {
	_, _, overflows := m.queryReadoutWindow(max(m.Width(), 1))
	return overflows
}

// queryReadoutGuidance is the footer while the stack is open. It is the gated
// keymap a modal sub-state supplies, so the readout takes no footer rank of its
// own and the ranked projection stays untouched.
func (m *OutlinerModel) queryReadoutGuidance() chrome.GuidanceSpec {
	closeKey := m.queryReadoutKey()
	rows := m.QueryReadoutRows()
	overflows := m.queryReadoutOverflows()

	// The peel keys are named off the stack alone. An empty stack under a
	// document long enough to scroll is still an empty stack: the rows it is
	// showing are the read-only ones, and a digit over those would promise a peel
	// that cannot happen.
	var hints []chrome.GuidanceHint
	if len(rows) == 0 {
		if !overflows {
			// Two keys, one act, one entry — the same shape the populated readout
			// below already uses, so the empty stack does not spend a wider footer
			// than the full one saying less.
			return chrome.GuidanceSpec{
				Hints: chrome.RequiredGuidanceHints(closeKey + "/Esc close readout"),
				Rows:  1,
			}
		}
		hints = chrome.RequiredGuidanceHints(closeKey + "/Esc close readout")
	} else {
		peel := "1 peel one"
		if shown := min(len(rows), queryReadoutRowLimit); shown > 1 {
			peel = fmt.Sprintf("1-%d peel one", shown)
		}
		hints = append(chrome.RequiredGuidanceHints(peel, "x peel all", closeKey+"/Esc close"),
			chrome.OptionalGuidanceHints(10, "s save as lens")...)
	}
	if overflows {
		// Advertised only over a document that can move, for the same reason the
		// dock's paging hints are: the keys are claimed either way, and naming them
		// over a readout that fits would promise a scroll with nowhere to go.
		hints = append(hints, chrome.OptionalGuidanceHints(20,
			"j/k scroll", "PgUp/PgDn page", "Home/End ends")...)
	}
	return chrome.GuidanceSpec{Hints: hints, Rows: 2}
}

// queryReadoutKey is the key that opens and closes the readout, read off the
// registry so the footer, the context row's overflow hint, and the operation
// itself all name the same one.
func (m *OutlinerModel) queryReadoutKey() string {
	if m.OperationRegistry != nil {
		if key := m.OperationRegistry.InputShortcutForOperation(
			operation.InputStrategySimplified, operation.OperationCoreQueryReadout); key != "" {
			return key
		}
	}
	return "Q"
}

// queryReadoutDetailsHint is what the context row says where it had to shorten
// or drop a value. It is one phrase because the row has one place to send a
// reader, and that place is this readout.
func (m *OutlinerModel) queryReadoutDetailsHint() string {
	return m.queryReadoutKey() + " details"
}

// --- The week browse: `gw` over Today's week strip, a gated sub-state on the
// readout's own terms. ---

// The week browse is `gw` over Today's day canvas: a gated sub-state that
// selects one day of the strip and lists what that day holds under it. The
// canvas stays paint — it is still not in m.Rows, takes no cursor, and answers
// no click — so the browse owns no row and moves no window; it changes which
// ink a day name wears and which lines the padding under the strip draws.
//
// It is session-only and frozen nowhere: the day is an offset into the week
// ahead, read against a canvas computed from the live population each frame,
// so a deadline stepped while it is open moves its mark at the next paint.
type weekBrowseState struct {
	Active bool
	// Day indexes the canvas's Ahead days, 0 being tomorrow.
	Day int
}

// weekBrowseDays is the strip's length, which the canvas fixes.
const weekBrowseDays = 7

// OpenWeekBrowse selects the first dated day of the week ahead, or refuses with
// the sentence the status line says. It opens only where the canvas is drawn
// and has something dated, since a browse over a week the screen is not showing
// would be keys with nothing to act on.
func (m *OutlinerModel) OpenWeekBrowse() (string, bool) {
	if m == nil {
		return "", false
	}
	if !m.dayCanvasEligible() {
		return "The week ahead is drawn in Today; press gt, then gw", false
	}
	canvas := m.computeDayCanvas(time.Now())
	if canvas.AheadEmpty() {
		return "Nothing is dated in the week ahead", false
	}
	m.closeMenuBar()
	m.Week = weekBrowseState{Active: true}
	for at, day := range canvas.Ahead {
		if !day.Empty() {
			m.Week.Day = at
			break
		}
	}
	return "", true
}

// closeWeekBrowse puts the selection down; the strip draws as before.
func (m *OutlinerModel) closeWeekBrowse() {
	m.Week = weekBrowseState{}
}

// weekBrowseSelected is the day the strip marks, or -1 while the browse is
// closed or the canvas cannot speak for this view.
func (m *OutlinerModel) weekBrowseSelected() int {
	if !m.Week.Active || !m.dayCanvasEligible() {
		return -1
	}
	return m.Week.Day
}

// HandleWeekBrowseKey is the browse's own gated keymap. Like the readout it
// owns input while open, so an unlisted key closes it rather than running an
// act the footer is not naming.
func (m *OutlinerModel) HandleWeekBrowseKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "h", "left":
		m.Week.Day = max(m.Week.Day-1, 0)
		return m, nil
	case "l", "right":
		m.Week.Day = min(m.Week.Day+1, weekBrowseDays-1)
		return m, nil
	case "home":
		m.Week.Day = 0
		return m, nil
	case "end":
		m.Week.Day = weekBrowseDays - 1
		return m, nil
	case "enter":
		return m.openWeekDayInDue()
	}
	m.closeWeekBrowse()
	return m, nil
}

// openWeekDayInDue leaves for Due, the mode that lists dated work in date
// order, with the cursor on the first of the selected day's tasks the
// projection draws. A day whose work Due does not draw — check-ins alone —
// still lands in Due, where the mode's own clamp seats the cursor.
func (m *OutlinerModel) openWeekDayInDue() (tea.Model, tea.Cmd) {
	canvas := m.computeDayCanvas(time.Now())
	day := m.Week.Day
	m.closeWeekBrowse()
	model, cmd := m.SelectOutlinerMode(chrome.ModeDue)
	if day < 0 || day >= len(canvas.Ahead) {
		return model, cmd
	}
	for _, item := range canvas.Ahead[day].Items {
		if m.MoveCursorToIdentity(taskref.TaskIdentity{ProjectID: item.ProjectID, TaskID: item.TaskID}) {
			break
		}
	}
	return model, cmd
}

// weekBrowseGuidance names the browse's keys and its way out.
func (m *OutlinerModel) weekBrowseGuidance() chrome.GuidanceSpec {
	hints := chrome.RequiredGuidanceHints("←/→ day", "Enter open in Due", "Esc close")
	return chrome.GuidanceSpec{Hints: hints, Rows: 1}
}

// weekBrowseRestingItems bounds the day's list under an empty Today's strip,
// where the card is centered and has no window to measure a budget from.
const weekBrowseRestingItems = 6

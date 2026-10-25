package outliner

import (
	"charm.land/lipgloss/v2"

	"github.com/sglavoie/docket/internal/tui/chrome"
	"github.com/sglavoie/docket/internal/tui/textfit"
	"github.com/sglavoie/docket/internal/tui/theme"
)

// The dock is Peek's place on the frame: a divider, the document window, and a
// height that is decided by the frame and by nothing else. That independence is
// the whole point of docking it. A pane that grew with its content moved the list
// under the cursor every time the cursor landed on a longer task, so reading one
// row cost the position of every other; here the list keeps the same rows and
// the cursor keeps the same screen row whether the detail below is one line or
// fifty. The height follows the terminal instead, which is a different axis: a
// resize is one deliberate act that moves everything once, while crossing a row
// must move nothing.
const (
	peekDockDividerRows = 1
	// peekDockFrameDivisor, peekDockPreferredMinRows and peekDockPreferredMaxRows
	// are the dock's ordinary height: a quarter of the height this view was handed,
	// bounded below so a short terminal still shows a readable page and above so a
	// tall one spends the rest of itself on the list. A quarter is the share that
	// leaves the list the majority of every frame it is drawn in.
	peekDockFrameDivisor     = 4
	peekDockPreferredMinRows = 6
	peekDockPreferredMaxRows = 8
	// peekDockMinRows is the whole dock at its smallest displayed size: one content
	// line, the divider that separates it from the list, and the position line that
	// says how much of the document is off screen. Below this the dock is not drawn
	// at all rather than drawn without one of the three — a window with no content
	// row, or one that cannot say what it is hiding, is worse than the list keeping
	// the rows.
	peekDockMinRows = peekDockDividerRows + inlinePeekPositionRows + 1
	// peekDockListFloorRows is the row the dock may never take: the list is the
	// view and the dock is a detail beside it, so a frame too small for both keeps
	// one task row and shortens the dock.
	peekDockListFloorRows = 1
	// peekDockFooterWrapRows is the floor outlinerListRows puts under the shell's
	// measured footer excess while the dock is open. The dock advertises its own
	// paging keys only over a document that can move, so at minimum width the
	// footer wraps for one row and not for the next, and a budget following that
	// difference would put the dock back under the cursor's control. One row is
	// reserved either way; a taller footer is still charged in full.
	peekDockFooterWrapRows = 1
	// peekDockIndent is the column the dock's content starts in. It is fixed
	// rather than read off the cursor row: the dock hangs off the frame, not off a
	// tree node, so a deeply nested task and a root one read at the same margin.
	peekDockIndent = "  "
	// peekDockPlaceholderText is what the dock says for a row that owns no detail
	// document — a section header or a note. The dock stays open
	// and stays this tall there, so crossing such a row moves nothing on screen.
	peekDockPlaceholderText = "No detail to show for this row."
	// peekDockPlaceholderFingerprint identifies that stand-in content. It is a
	// constant because the placeholder is one document however many rows fall back
	// to it, and it can never be scrolled: a row with no document has no offset to
	// remember.
	peekDockPlaceholderFingerprint = "peek-dock-placeholder"
)

// peekDockGeometry is the dock's one measurement, and every question about
// Peek's size is answered off it: how tall the dock is, how much of the document
// is on screen, how far the two paging distances travel, and whether the dock is
// drawn at all. It carries the body alone because the divider and the position
// line are fixed costs around it, so a geometry cannot describe a window it
// would not draw.
//
// It is a value rather than a method on the model so the render, the offset
// clamp, the paging keys, and the availability hints all read one result instead
// of four recomputations that agree today.
type peekDockGeometry struct {
	bodyRows int
}

// Displayed reports whether this geometry draws a dock. The zero value does not,
// which is both a closed Peek and a frame with no room for the smallest dock,
// and those two are deliberately one case: in each of them there is nothing on
// screen to scroll, to advertise, or to reserve rows for.
func (g peekDockGeometry) Displayed() bool {
	return g.bodyRows > 0
}

// BodyRows is how many of the document's content rows are on screen.
func (g peekDockGeometry) BodyRows() int {
	return max(g.bodyRows, 0)
}

// WindowRows is one document's whole window: its content rows and the position
// line under them.
func (g peekDockGeometry) WindowRows() int {
	if !g.Displayed() {
		return 0
	}
	return g.bodyRows + inlinePeekPositionRows
}

// TotalRows is the whole dock, the divider included, and therefore what the list
// reservation subtracts.
func (g peekDockGeometry) TotalRows() int {
	if !g.Displayed() {
		return 0
	}
	return peekDockDividerRows + g.WindowRows()
}

// HalfPageRows and FullPageRows are the two distances a scroll travels, both
// derived from the window rather than declared beside it. A full page leaves one
// content row on screen across the move, which is what makes the jump readable:
// the row that ended the last window leads the next one, so a wrapped sentence is
// never split across a keystroke with nothing in common between the two halves.
// A half page is the shorter step the same keys mean over every list in the
// product. Both keep a one-row minimum, because a window one or two rows tall
// still has to be able to move.
func (g peekDockGeometry) HalfPageRows() int {
	if !g.Displayed() {
		return 0
	}
	return max(g.bodyRows/2, 1)
}

func (g peekDockGeometry) FullPageRows() int {
	if !g.Displayed() {
		return 0
	}
	return max(g.bodyRows-1, 1)
}

// PeekDockVisible reports whether the dock is on screen. It is Peek's one
// open-or-closed answer, and every reader — the render, the reservation, the
// operation grammar, and the guidance — asks here, so none of them can disagree
// about whether there is a dock. The cursor row is deliberately not part of it:
// the dock survives the cursor moving onto a row that owns no document, which is
// what the placeholder is for.
func (m *OutlinerModel) PeekDockVisible() bool {
	return m.peekDockGeometry().Displayed()
}

// PeekDockRows is what the dock costs the list: the divider, the window, and the
// position row, or zero. It is exported because the shell's own frame contracts
// measure against the dock rather than against a literal height, and it is a
// measurement of the frame rather than of the document, which is what makes the
// list's window, the cursor's screen row, and the page distance independent of
// what Peek is showing.
func (m *OutlinerModel) PeekDockRows() int {
	return m.peekDockGeometry().TotalRows()
}

// peekDockGeometry resolves the dock for the current frame. It is pure and reads
// no cursor, no selection, and no document: the ordinary height is a share of
// what this view was handed, and the only thing that shortens it is a frame with
// nowhere to put it. That is why a short task, a long one, a project header, a
// placeholder row, and an empty projection all dock at the same height in the
// same place.
//
// Two heights meet here. The preferred one is the frame's share; the available
// one is what is left after the chrome this view has already committed to. The
// dock takes the smaller, and gives up entirely below its own three-row minimum
// rather than drawing a window it would have to clip.
func (m *OutlinerModel) peekDockGeometry() peekDockGeometry {
	if m == nil || !m.PeekMode {
		return peekDockGeometry{}
	}
	total := min(m.peekDockPreferredRows(), m.peekDockAvailableRows())
	if total < peekDockMinRows {
		return peekDockGeometry{}
	}
	return peekDockGeometry{bodyRows: total - peekDockDividerRows - inlinePeekPositionRows}
}

// peekDockPreferredRows is the dock's ordinary height: a quarter of the height
// this view was allotted, bounded to the two ends. The height it divides is the
// view's own rather than the terminal's, because the shell's footer is already
// out of it — so a footer that grows does not also shrink the dock, and only the
// availability cap below can.
//
// It is deliberately independent of the strip, the footer excess, and the cursor,
// which is what the readout reads it for: a reservation that moved with a status
// message would move the dock on every message.
func (m *OutlinerModel) peekDockPreferredRows() int {
	if m == nil || !m.PeekMode {
		return 0
	}
	if m.PeekExpanded {
		return max(m.Height()*2/3, peekDockPreferredMinRows)
	}
	return min(max(max(m.Height(), 0)/peekDockFrameDivisor, peekDockPreferredMinRows), peekDockPreferredMaxRows)
}

// peekDockAvailableRows is the room the frame actually has for a dock: this
// view's own rows, less the footer the shell measured, less the strip that holds
// the conflict advisory, the focus notice, the status message, and the expanded
// readout, less the one task row the dock may never take. Everything named here
// is content this view has already committed to drawing, so the dock is what
// yields when they leave nothing.
//
// The strip is measured rather than predicted, and it is measured once: the
// readout inside it budgets against the preferred dock height above rather than
// against this answer, which is what keeps the two out of a circle.
func (m *OutlinerModel) peekDockAvailableRows() int {
	if m == nil {
		return 0
	}
	width := max(m.Width(), 1)
	return m.ContentHeight() - m.peekDockFooterReservation() - m.StatusStripHeight(width) - peekDockListFloorRows
}

// peekDockFooterReservation is what an open dock charges for the shell's footer:
// what the shell measured, with the wrap floor under it. Both the list and the
// dock's own availability read it here, so the rows one of them reserves are the
// rows the other measured against.
func (m *OutlinerModel) peekDockFooterReservation() int {
	if m == nil {
		return 0
	}
	return max(m.ShellFrame.ExcessRows(), peekDockFooterWrapRows)
}

// peekDockWidth is the width the dock draws at. The dock spans the frame rather
// than hanging off a row, so there is no scrollbar column to subtract and no
// need to ask which path drew the last frame: the terminal width is the answer
// on every frame, the empty projection included.
func (m *OutlinerModel) peekDockWidth() int {
	if m == nil {
		return 1
	}
	return max(m.Width(), 1)
}

// PeekDockLines is the dock: the divider that separates it from the list, the
// document's window, and the position row under it. The slice is exactly
// PeekDockRows() long while the dock is open and empty while it is closed, so the
// height that is reserved and the block that is drawn are one measurement rather
// than two that agree.
func (m *OutlinerModel) PeekDockLines() []string {
	geometry := m.peekDockGeometry()
	if !geometry.Displayed() {
		return nil
	}
	width := m.peekDockWidth()
	lines := make([]string, 0, geometry.TotalRows())
	lines = append(lines, chrome.RenderHorizontalDivider(width, chrome.WithDividerColor(theme.ColorBorder)))
	return append(lines, m.peekDockWindow(width, geometry)...)
}

// peekDockWindow is the document's own rows: the cursor row's detail at its
// remembered offset, or the placeholder for a row that owns none. Both come back
// the same height, because Lines pads a short document rather than shrinking the
// window.
func (m *OutlinerModel) peekDockWindow(width int, geometry peekDockGeometry) []string {
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		return peekDockPlaceholderDocument(width, geometry).Lines(0)
	}
	return doc.Lines(m.peekOffsetForRow(row, doc))
}

// peekDockPlaceholderDocument is the stand-in document, built as a document
// rather than as a block of text so it goes through the same Lines the real one
// does: the padding, the position row, and the height are the window's rules,
// not a second copy of them. It takes the same geometry every other document
// does, which is what keeps a row owning no detail exactly as tall as one owning
// forty rows of it.
func peekDockPlaceholderDocument(width int, geometry peekDockGeometry) inlinePeekDocument {
	available := max(width-lipgloss.Width(peekDockIndent), 1)
	text := theme.MutedStyle.Render(textfit.TruncateIntentional(peekDockPlaceholderText, available, "…"))
	return inlinePeekDocument{
		Body:        []string{peekDockIndent + text},
		Fingerprint: peekDockPlaceholderFingerprint,
		indent:      peekDockIndent,
		width:       available,
		window:      geometry,
	}
}

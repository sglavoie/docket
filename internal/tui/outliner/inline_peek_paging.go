package outliner

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// inlinePeekOwnerKind names what kind of thing a remembered offset belongs to.
// Peek draws a task's document today, and a task identity is what tells one
// document from another; the kind travels beside it so a second kind of
// document — a project header's, which would share this window arithmetic and
// these four keys rather than grow its own — can key the same slot without two
// owners that happen to carry equal identities ever reading each other's offset.
type inlinePeekOwnerKind uint8

const (
	inlinePeekOwnerNone inlinePeekOwnerKind = iota
	inlinePeekOwnerTask
)

// inlinePeekOwner is the typed identity half of the key, and the extension point
// a second document kind takes: it adds its kind and its own identity field
// here, teaches peekCursorDocument to build its document, and every reset rule
// below keeps working with nothing else changed.
type inlinePeekOwner struct {
	kind inlinePeekOwnerKind
	task taskref.TaskIdentity
}

func taskPeekOwner(identity taskref.TaskIdentity) inlinePeekOwner {
	return inlinePeekOwner{kind: inlinePeekOwnerTask, task: identity}
}

// sameAs answers whether two owners are the same document's. An owner naming no
// kind matches nothing, itself included: that is the unwritten slot, and a
// terminal that has not been sized yet would otherwise hand it a key of zeroes
// to agree with.
func (owner inlinePeekOwner) sameAs(other inlinePeekOwner) bool {
	if owner.kind == inlinePeekOwnerNone || other.kind == inlinePeekOwnerNone {
		return false
	}
	return owner.kind == other.kind && owner.task == other.task
}

// inlinePeekOffsetKey is what a remembered offset is remembered for. Peek's
// scrolling is session-only and belongs to one owner's one document at one
// layout, so the key carries all three: the typed owner, the width-independent
// fingerprint of the detail it showed, and the terminal size that document was
// laid out against.
type inlinePeekOffsetKey struct {
	owner       inlinePeekOwner
	fingerprint string
	width       int
	height      int
}

// sameAs answers whether a remembered offset still belongs to what is on screen.
// It names each axis rather than comparing the two structs with `==`, because a
// field nobody spells here is a field the reset does not consult: the
// declaration audit reports an unread key field, so forgetting to extend this
// comparison is a red gate rather than an offset that silently outlives its
// document.
func (key inlinePeekOffsetKey) sameAs(other inlinePeekOffsetKey) bool {
	return key.owner.sameAs(other.owner) &&
		key.fingerprint == other.fingerprint &&
		key.width == other.width &&
		key.height == other.height
}

// peekOffsetState is the one offset Peek remembers, counted in content rows from
// the top of the body. There is exactly one slot because Peek shows exactly one
// row's detail; every other owner reads as a key mismatch and therefore as the
// top of its own document.
type peekOffsetState struct {
	key    inlinePeekOffsetKey
	offset int
}

// inlinePeekIndentForRow is the column Peek's lines start in: past the row's
// tree prefix and the fixed lead the row spends on its cursor gutter, checkbox,
// fold indicator, and status icon. Peek carries no tree glyphs of its own, so
// the indent is spaces rather than a copy of the prefix.
func inlinePeekIndentForRow(row OutlinerRow) string {
	return strings.Repeat(" ", lipgloss.Width(row.Prefix)+4)
}

// inlinePeekDocumentForRow builds the complete detail document for one row.
// Every reader — the renderer, the viewport measurement, and the two paging
// keys — comes through here, so none of them can be looking at a different
// document than the others.
func (m *OutlinerModel) inlinePeekDocumentForRow(
	row OutlinerRow,
	indent string,
	rowWidth int,
	colLayout *chrome.ColumnLayout,
) inlinePeekDocument {
	if m == nil || !row.IsTaskRow() {
		return inlinePeekDocument{}
	}
	index := m.dependencyIndex()
	return inlinePeekDocumentFor(inlinePeekOptions{
		Task:             row.Task,
		Indent:           indent,
		Width:            rowWidth,
		TitleOverflow:    m.TaskRowTitleOverflow(row, rowWidth, colLayout),
		Breadcrumb:       row.Breadcrumb,
		Dependencies:     outgoingDependencyDetails(row.Task, index),
		Dependents:       incomingDependencyDetails(m.rowIdentity(row), index),
		DependencyCensus: DependencyCensusFor(row.Task, index),
	})
}

func (m *OutlinerModel) inlinePeekOffsetKey(row OutlinerRow, doc inlinePeekDocument) inlinePeekOffsetKey {
	return inlinePeekOffsetKey{
		owner:       taskPeekOwner(m.rowIdentity(row)),
		fingerprint: doc.Fingerprint,
		width:       m.Width(),
		height:      m.Height(),
	}
}

// peekOffsetForRow resolves the content row a row's Peek starts at. It is pure,
// and that is the whole of how scrolling resets: the stored offset answers only
// for the exact owner, detail, reference population, and layout it was written
// against, so each of those changing falls back to the top without any writer
// having to notice. The model width and height stand in for the layout rather
// than the caller's width, which differs by the scrollbar column between the
// measuring pass and the drawing one.
func (m *OutlinerModel) peekOffsetForRow(row OutlinerRow, doc inlinePeekDocument) int {
	if m == nil || !m.peek.key.sameAs(m.inlinePeekOffsetKey(row, doc)) {
		return 0
	}
	return doc.ClampOffset(m.peek.offset)
}

// peekRenderWidth is the width the last render gave a row. The scrolling keys
// read it rather than recomputing a window: ComputeOutlinerViewport is the single
// window authority, and it has already run for the frame the user is looking at.
func (m *OutlinerModel) peekRenderWidth() int {
	if width := m.LastViewport.rowWidth; width > 0 {
		return width
	}
	return max(m.Width()-1, 1)
}

// peekCursorDocument is the document the scrolling keys act on, or false when
// there is none: Peek closed, a cursor off the rows, or a row that is not a
// task. PeekMode alone is not enough — it survives the cursor moving onto a
// header, where Peek draws nothing to page.
func (m *OutlinerModel) peekCursorDocument() (OutlinerRow, inlinePeekDocument, bool) {
	if m == nil || !m.PeekMode || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	width := m.peekRenderWidth()
	doc := m.inlinePeekDocumentForRow(row, inlinePeekIndentForRow(row), width, chrome.WorstCaseColumnLayout(width))
	return row, doc, true
}

// CanScrollPeek reports whether the window can move at all in a direction, +1
// down and -1 up. One answer covers all four keys, because a half page and a
// full page clamp against the same two ends: where the shorter step cannot
// move, neither can the longer one. It decides what the footer offers and
// nothing about what the keys consume — a Peek on screen consumes them at both
// boundaries rather than letting the press fall through to the list.
func (m *OutlinerModel) CanScrollPeek(direction int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekOffsetForRow(row, doc)
	return doc.ClampOffset(current+direction) != current
}

// scrollPeek moves the window by a number of content rows and reports whether it
// moved. It is the only writer of the offset; every other reset falls out of the
// key. Both distances come through here, so a half page and a full page are one
// movement measured twice rather than two paging models.
func (m *OutlinerModel) scrollPeek(rows int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekOffsetForRow(row, doc)
	next := doc.ClampOffset(current + rows)
	if next == current {
		return false
	}
	m.peek = peekOffsetState{key: m.inlinePeekOffsetKey(row, doc), offset: next}
	return true
}

// HalfPagePeek moves Peek a half page, +1 down and -1 up, and reports whether it
// moved. It is Ctrl+D and Ctrl+U over an open Peek: the same distance those keys
// move a list cursor, spent on the pane that is on screen instead.
func (m *OutlinerModel) HalfPagePeek(direction int) bool {
	return m.scrollPeek(direction * inlinePeekHalfPageRows)
}

// PagePeek moves Peek a full page, +1 down and -1 up, and reports whether it
// moved. Ctrl+F and Ctrl+B keep this distance, which is what every other pageable
// body on the product gives them.
func (m *OutlinerModel) PagePeek(direction int) bool {
	return m.scrollPeek(direction * inlinePeekFullPageRows)
}

// peekScrollHints names the movements that can actually happen, half page first
// because it is the smaller commitment and the one a reader reaches for. The
// Peek footer is a hand-written hint list rather than the strategy's ranks, so
// these read their keys off the strategy instead of spelling them: the hint
// cannot drift from the binding it teaches. The full-page pair carries the later
// priority, so a narrow terminal keeps the shorter step rather than half of each.
func (m *OutlinerModel) peekScrollHints() []chrome.GuidanceHint {
	peek := m.PeekCommandContext()
	if !peek.Visible {
		return nil
	}
	hints := make([]chrome.GuidanceHint, 0, 4)
	if peek.CanPageDown {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageDown, "Ctrl+D")+" more", peekHalfPageHintPriority, false))
	}
	if peek.CanPageUp {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageUp, "Ctrl+U")+" back", peekHalfPageHintPriority, false))
	}
	if peek.CanPageDown {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekPageDown, "Ctrl+F")+" page down", peekFullPageHintPriority, false))
	}
	if peek.CanPageUp {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekPageUp, "Ctrl+B")+" page up", peekFullPageHintPriority, false))
	}
	return hints
}

// The two priorities the Peek guidance spends on movement. Both sit under the
// row of ordinary optional hints, and the full page sits under the half page,
// because a reader who only has room for one offer is better served by the step
// that cannot overshoot.
const (
	peekHalfPageHintPriority = 9
	peekFullPageHintPriority = 11
)

// peekDisplayKey is the key Peek answers to when the registry cannot be asked.
// The guidance and the header projection both spell it through here rather than
// inline, so the day it moves again there is one literal to change.
const peekDisplayKey = "b"

func (m *OutlinerModel) peekOperationKey(id operation.OperationID, fallback string) string {
	if m == nil || m.OperationRegistry == nil {
		return fallback
	}
	if key := m.OperationRegistry.InputShortcutForOperation(operation.InputStrategySimplified, id); key != "" {
		return key
	}
	return fallback
}

// PeekCommandContext is Peek's contribution to the operation grammar's view of
// the world, and it answers two different questions with two different fields.
// Visible is consumption: the four scrolling descriptors are applicable under it
// alone, so a Peek on screen swallows every one of those keys — at either
// boundary and over a document too short to move — rather than letting the press
// reach the list cursor underneath. CanPageDown and CanPageUp are availability:
// they say whether the window has anywhere to go, and only the footer hints read
// them, because an offer that cannot move is worse than no offer while a key that
// silently moves the wrong thing is worse than both.
func (m *OutlinerModel) PeekCommandContext() operation.PeekContext {
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		return operation.PeekContext{}
	}
	if !doc.Paged() {
		return operation.PeekContext{Visible: true}
	}
	current := m.peekOffsetForRow(row, doc)
	return operation.PeekContext{
		Visible:     true,
		CanPageDown: current < doc.MaxOffset(),
		CanPageUp:   current > 0,
	}
}

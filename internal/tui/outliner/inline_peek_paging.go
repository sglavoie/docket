package outliner

import (
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// inlinePeekPageKey is what a remembered page is remembered for. Peek's paging
// is session-only and belongs to one task's one document at one layout, so the
// key carries all three: the task's identity, the width-independent fingerprint
// of the detail it showed, and the terminal size that document was laid out
// against.
type inlinePeekPageKey struct {
	identity    taskref.TaskIdentity
	fingerprint string
	width       int
	height      int
}

// sameAs answers whether a remembered page still belongs to what is on screen.
// It names each axis rather than comparing the two structs with `==`, because a
// field nobody spells here is a field the reset does not consult: the
// declaration audit reports an unread key field, so forgetting to extend this
// comparison is a red gate rather than a page that silently outlives its
// document.
func (key inlinePeekPageKey) sameAs(other inlinePeekPageKey) bool {
	return key.identity == other.identity &&
		key.fingerprint == other.fingerprint &&
		key.width == other.width &&
		key.height == other.height
}

// peekPageState is the one page Peek remembers. There is exactly one slot
// because Peek shows exactly one row's detail; every other task reads as a key
// mismatch and therefore as the first page.
type peekPageState struct {
	key  inlinePeekPageKey
	page int
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

func (m *OutlinerModel) inlinePeekPageKey(row OutlinerRow, doc inlinePeekDocument) inlinePeekPageKey {
	return inlinePeekPageKey{
		identity:    m.rowIdentity(row),
		fingerprint: doc.Fingerprint,
		width:       m.Width(),
		height:      m.Height(),
	}
}

// peekPageForRow resolves the page a row's Peek is showing. It is pure, and
// that is the whole of how paging resets: the stored page answers only for the
// exact task, detail, reference population, and layout it was written against,
// so each of those changing falls back to the first page without any writer
// having to notice. The model width and height stand in for the layout rather
// than the caller's width, which differs by the scrollbar column between the
// measuring pass and the drawing one.
func (m *OutlinerModel) peekPageForRow(row OutlinerRow, doc inlinePeekDocument) int {
	if m == nil || !m.peek.key.sameAs(m.inlinePeekPageKey(row, doc)) {
		return 0
	}
	return doc.ClampPage(m.peek.page)
}

// peekRenderWidth is the width the last render gave a row. The paging keys read
// it rather than recomputing a window: ComputeOutlinerViewport is the single
// window authority, and it has already run for the frame the user is looking at.
func (m *OutlinerModel) peekRenderWidth() int {
	if width := m.LastViewport.rowWidth; width > 0 {
		return width
	}
	return max(m.Width()-1, 1)
}

// peekCursorDocument is the document the paging keys act on, or false when
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

// CanPagePeek reports whether Peek can move the requested number of pages. It
// is what keeps the two keys off the footer and out of the grammar wherever
// there is nothing to page: an offer that cannot move is worse than no offer.
func (m *OutlinerModel) CanPagePeek(delta int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekPageForRow(row, doc)
	return doc.ClampPage(current+delta) != current
}

// PagePeek moves Peek by whole pages and reports whether it moved. It is the
// only writer of the page state; every other reset falls out of the key.
func (m *OutlinerModel) PagePeek(delta int) bool {
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return false
	}
	current := m.peekPageForRow(row, doc)
	next := doc.ClampPage(current + delta)
	if next == current {
		return false
	}
	m.peek = peekPageState{key: m.inlinePeekPageKey(row, doc), page: next}
	return true
}

// peekPagingHintLabels names the page turns that can actually move, in the
// order they are pressed. The Peek footer is a hand-written hint list rather
// than the strategy's ranks, so these read their keys off the strategy instead
// of spelling them: the hint cannot drift from the binding it teaches.
func (m *OutlinerModel) peekPagingHintLabels() []string {
	peek := m.PeekCommandContext()
	if !peek.Visible {
		return nil
	}
	labels := make([]string, 0, 2)
	if peek.CanPageDown {
		labels = append(labels, m.peekOperationKey(operation.OperationCorePeekPageDown, "Ctrl+F")+" more")
	}
	if peek.CanPageUp {
		labels = append(labels, m.peekOperationKey(operation.OperationCorePeekPageUp, "Ctrl+B")+" back")
	}
	return labels
}

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
// the world: whether a page is on screen and whether it can move in either
// direction. The two paging descriptors read it, so dispatch, `?` help, and the
// footer all answer from the same fact.
func (m *OutlinerModel) PeekCommandContext() operation.PeekContext {
	row, doc, ok := m.peekCursorDocument()
	if !ok {
		return operation.PeekContext{}
	}
	if !doc.Paged() {
		return operation.PeekContext{Visible: true}
	}
	current := m.peekPageForRow(row, doc)
	return operation.PeekContext{
		Visible:     true,
		CanPageDown: doc.ClampPage(current+1) != current,
		CanPageUp:   doc.ClampPage(current-1) != current,
	}
}

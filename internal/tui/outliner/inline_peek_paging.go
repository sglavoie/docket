package outliner

import (
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"

	"charm.land/lipgloss/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/fuzzy"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/tui/textfit"
	"github.com/sglavoie/tt/internal/tui/theme"
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

// inlinePeekOwnerKind names what kind of thing a remembered offset belongs to.
// Peek draws two documents — a task's and a project header's — and each kind's
// own identity is what tells one document of that kind from another. The kind
// travels beside the identity so the two populations cannot collide: a task
// owner and a project owner are never the same owner however their identity
// fields happen to compare.
type inlinePeekOwnerKind uint8

const (
	inlinePeekOwnerNone inlinePeekOwnerKind = iota
	inlinePeekOwnerTask
	inlinePeekOwnerProject
	inlinePeekOwnerChecklist
)

// inlinePeekOwner is the typed identity half of the key, and the extension point
// a further document kind takes: it adds its kind and its own identity field
// here, teaches peekCursorDocument to build its document, and every reset rule
// below keeps working with nothing else changed.
type inlinePeekOwner struct {
	kind inlinePeekOwnerKind
	task taskref.TaskIdentity
	// project is the catalog identity rather than the row's collapse key, which
	// is what makes the several headers one project can hold at once — All emits
	// a group per state section — one owner: a page turn under one of them is the
	// same page under the others, because they are the same project's detail.
	project string
	// Checklist items have no durable ID. Index and text prevent an unrelated
	// item at a reused position from inheriting this item's reading position.
	checklistIndex int
	checklistText  string
}

func taskPeekOwner(identity taskref.TaskIdentity) inlinePeekOwner {
	return inlinePeekOwner{kind: inlinePeekOwnerTask, task: identity}
}

// projectPeekOwner names the project a header row stands for. A header with no
// identity resolves no project and therefore draws no document, so it takes the
// unwritten owner rather than an owner every other anonymous header would agree
// with.
func projectPeekOwner(projectID string) inlinePeekOwner {
	if projectID == "" {
		return inlinePeekOwner{}
	}
	return inlinePeekOwner{kind: inlinePeekOwnerProject, project: projectID}
}

// sameAs answers whether two owners are the same document's. An owner naming no
// kind matches nothing, itself included: that is the unwritten slot, and a
// terminal that has not been sized yet would otherwise hand it a key of zeroes
// to agree with.
func (owner inlinePeekOwner) sameAs(other inlinePeekOwner) bool {
	if owner.kind == inlinePeekOwnerNone || other.kind == inlinePeekOwnerNone {
		return false
	}
	return owner.kind == other.kind && owner.task == other.task && owner.project == other.project &&
		owner.checklistIndex == other.checklistIndex && owner.checklistText == other.checklistText
}

// rowSupportsPeek names the rows that carry a detail document. Every reader of a
// Peek — the render, the measurement, the offset key, and the four scrolling
// keys — asks here, so none of them can admit a row another one refuses.
func rowSupportsPeek(row OutlinerRow) bool {
	return row.IsTaskRow() || row.IsProjectHeader || checklistPeekItemValid(row)
}

// inlinePeekOffsetKey is what a remembered offset is remembered for. Peek's
// scrolling is session-only. Matching content and geometry reuse a row offset;
// changed content or geometry resolves the saved reading anchor instead.
type inlinePeekOffsetKey struct {
	owner       inlinePeekOwner
	fingerprint string
	query       string
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
		key.query == other.query &&
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
	anchor peekReadingAnchor
}

// inlinePeekDocumentForRow builds the complete detail document for one row.
// Every reader — the renderer, the viewport measurement, and the two paging
// keys — comes through here, so none of them can be looking at a different
// document than the others. The row's kind picks the builder, and the project
// header takes the branch first: it is not a task row, and a task row is never a
// header.
func (m *OutlinerModel) inlinePeekDocumentForRow(
	row OutlinerRow,
	indent string,
	rowWidth int,
	geometry peekDockGeometry,
) inlinePeekDocument {
	if m == nil {
		return inlinePeekDocument{}
	}
	if row.IsChecklistItem {
		return inlinePeekChecklistDocumentFor(row, indent, rowWidth, geometry)
	}
	if row.IsProjectHeader {
		// The row's own project identity, not the view's: a cross-project group
		// header names a project the scoped model may not be, and resolving through
		// the scope would draw the wrong project's comments under it.
		return inlinePeekProjectDocumentFor(inlinePeekProjectOptions{
			Project: m.projectByGroupID(row.ProjectID),
			Indent:  indent,
			Width:   rowWidth,
			Window:  geometry,
		})
	}
	if !row.IsTaskRow() {
		return inlinePeekDocument{}
	}
	index := m.dependencyIndex()
	excerpt := m.peekSearchExcerpt(row)
	projectSlug, _ := m.projectSlugForRow(row)
	return inlinePeekDocumentFor(inlinePeekOptions{
		Task:             row.Task,
		Indent:           indent,
		Width:            rowWidth,
		Window:           geometry,
		Compact:          !m.PeekExpanded,
		Breadcrumb:       row.Breadcrumb,
		ProjectSlug:      projectSlug,
		SearchExcerpt:    excerpt,
		SearchExcerptKey: m.searchTextQuery() + "\x1e" + excerpt,
		Planning:         m.peekPlanningMembership(row),
		Dependencies:     outgoingDependencyDetails(row.Task, index),
		Dependents:       incomingDependencyDetails(m.rowIdentity(row), index),
		DependencyCensus: DependencyCensusFor(row.Task, index),
	})
}

// peekPlanningMembership names the planning collection a task belongs to, or
// nothing. Membership is neither a lifecycle state nor a field on the task: it is
// an explicit project-qualified entry in the planning state, and Today and Next
// are mutually exclusive, so one name is the whole answer. This is the impure
// half of the document — the renderer below it reads only what it is handed — and
// it is resolved here for the same reason the reference lists are.
func (m *OutlinerModel) peekPlanningMembership(row OutlinerRow) string {
	if m == nil || m.Runtime == nil || row.Task == nil {
		return ""
	}
	identity := m.rowIdentity(row)
	if identity.ProjectID == "" || identity.TaskID == "" {
		return ""
	}
	locator := config.ProjectTaskLocator{ProjectID: identity.ProjectID, TaskID: identity.TaskID}
	planning := m.Runtime.Planning()
	switch {
	case slices.Contains(planning.GetToday(), locator):
		return chrome.PlanStatusValue("today")
	case slices.Contains(planning.GetNext(), locator):
		return chrome.PlanStatusValue("next")
	}
	return ""
}

func (m *OutlinerModel) inlinePeekOffsetKey(row OutlinerRow, doc inlinePeekDocument) inlinePeekOffsetKey {
	return inlinePeekOffsetKey{
		owner:       m.peekOwnerForRow(row),
		fingerprint: doc.Fingerprint,
		query:       m.searchTextQuery(),
		width:       m.Width(),
		height:      m.Height(),
	}
}

// peekOwnerForRow types the row the offset belongs to, on the same split
// inlinePeekDocumentForRow builds it on.
func (m *OutlinerModel) peekOwnerForRow(row OutlinerRow) inlinePeekOwner {
	if row.IsProjectHeader {
		return projectPeekOwner(row.ProjectID)
	}
	if checklistPeekItemValid(row) {
		return inlinePeekOwner{
			kind: inlinePeekOwnerChecklist, task: m.rowIdentity(row),
			checklistIndex: row.ChecklistIndex, checklistText: row.Task.Checklist[row.ChecklistIndex].Text,
		}
	}
	return taskPeekOwner(m.rowIdentity(row))
}

// peekOffsetForRow is pure: unchanged frames use the stored row, while reflow
// and edits resolve the saved content anchor in the current document. Only a
// scroll writes the anchor, so temporary shrinking cannot erase reading position.
func (m *OutlinerModel) peekOffsetForRow(row OutlinerRow, doc inlinePeekDocument) int {
	if m == nil || !m.peek.key.owner.sameAs(m.peekOwnerForRow(row)) || m.peek.key.query != m.searchTextQuery() {
		return 0
	}
	if m.peek.key.sameAs(m.inlinePeekOffsetKey(row, doc)) {
		return doc.ClampOffset(m.peek.offset)
	}
	return doc.offsetForAnchor(m.peek.anchor)
}

// peekCursorDocument is the document the dock draws and the scrolling keys act
// on, or false when there is none: no dock on the frame, a cursor off the rows,
// or a row that owns no document. An open Peek alone is not enough — it survives
// the cursor moving onto a section header or a note row, where the dock draws its
// placeholder and there is nothing to page.
//
// The frame's geometry is stamped onto the document here, which is the one place
// content and window meet: every reader below takes the window off the document
// it was handed rather than measuring the frame again.
func (m *OutlinerModel) peekCursorDocument() (OutlinerRow, inlinePeekDocument, bool) {
	geometry := m.peekDockGeometry()
	if m == nil || !geometry.Displayed() || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	row := m.Rows[m.Cursor]
	if !rowSupportsPeek(row) {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
	width := m.peekDockWidth()
	doc := m.inlinePeekDocumentForRow(row, peekDockIndent, width, geometry)
	// A header naming a project the population no longer holds draws nothing, and
	// nothing on screen is nothing to page: the four keys belong to the list again
	// rather than being swallowed by a pane that is not there.
	if doc.Empty() {
		return OutlinerRow{}, inlinePeekDocument{}, false
	}
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
// moved. It is the only writer of the offset and content anchor. Both distances
// come through here, so a half page and a full page are one
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
	m.peek = peekOffsetState{key: m.inlinePeekOffsetKey(row, doc), offset: next, anchor: doc.readingAnchor(next)}
	return true
}

// HalfPagePeek moves Peek a half page, +1 down and -1 up, and reports whether it
// moved. It is Ctrl+D and Ctrl+U over an open Peek: the same distance those keys
// move a list cursor, spent on the pane that is on screen instead. The distance
// is the window's half rather than a constant, so the keys keep meaning "half of
// what I can see" at every dock height.
func (m *OutlinerModel) HalfPagePeek(direction int) bool {
	return m.scrollPeek(direction * m.peekDockGeometry().HalfPageRows())
}

// PagePeek moves Peek a full page, +1 down and -1 up, and reports whether it
// moved. Ctrl+F and Ctrl+B keep the one-row overlap every other pageable body on
// the product gives them, measured against the window that is actually drawn.
func (m *OutlinerModel) PagePeek(direction int) bool {
	return m.scrollPeek(direction * m.peekDockGeometry().FullPageRows())
}

// peekScrollHints names the movements that can actually happen. Only the half
// page is offered: it is the smaller commitment and the one a reader reaches for,
// and advertising Ctrl+F and Ctrl+B beside it spent a footer entry restating the
// same direction. The full-page pair still answers and `?` help still names it.
// The Peek footer is a hand-written hint list rather than the strategy's ranks,
// so these read their keys off the strategy instead of spelling them: the hint
// cannot drift from the binding it teaches.
func (m *OutlinerModel) peekScrollHints() []chrome.GuidanceHint {
	peek := m.PeekCommandContext()
	if !peek.Visible {
		return nil
	}
	hints := make([]chrome.GuidanceHint, 0, 2)
	if peek.CanPageDown {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageDown, "Ctrl+D")+" more", peekHalfPageHintPriority, false))
	}
	if peek.CanPageUp {
		hints = append(hints, chrome.NewGuidanceHint(
			m.peekOperationKey(operation.OperationCorePeekHalfPageUp, "Ctrl+U")+" back", peekHalfPageHintPriority, false))
	}
	return hints
}

// The priority the Peek guidance spends on movement: under the row of ordinary
// optional hints.
const peekHalfPageHintPriority = 9

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
// Visible is consumption, and it is the dock's own open-or-closed state rather
// than the cursor document's: the four scrolling descriptors are applicable
// under it alone, so a dock on screen swallows every one of those keys — at
// either boundary, over a document too short to move, and over a row showing the
// placeholder — rather than letting the press reach the list cursor underneath.
// The delegated handler is where a row that names no document refuses by name.
// CanPageDown and CanPageUp are availability: they say whether the window has
// anywhere to go, and only the footer hints read them, because an offer that
// cannot move is worse than no offer while a key that silently moves the wrong
// thing is worse than both.
func (m *OutlinerModel) PeekCommandContext() operation.PeekContext {
	if !m.PeekDockVisible() {
		return operation.PeekContext{}
	}
	row, doc, ok := m.peekCursorDocument()
	if !ok || !doc.Paged() {
		return operation.PeekContext{Visible: true}
	}
	current := m.peekOffsetForRow(row, doc)
	return operation.PeekContext{
		Visible:     true,
		CanPageDown: current < doc.MaxOffset(),
		CanPageUp:   current > 0,
	}
}

// peekSearchExcerpt reads the source attributed by the live projection and its
// actual case mode. A predicate-only query and a title hit need no explanation.
func (m *OutlinerModel) peekSearchExcerpt(row OutlinerRow) string {
	if !m.HasSearchProjection() || row.Task == nil || m.searchTextQuery() == "" {
		return ""
	}
	label := matchSourceLabels[row.MatchSource]
	if label == "" {
		return ""
	}
	text := searchMatchText(row.Task, m.projectByGroupID(row.ProjectID), row.MatchSource)
	_, positions := fuzzy.NewMatcher(m.searchCaseSensitive()).ScoreTermsWithPositions(text, m.searchTextQuery())
	if len(positions) == 0 {
		return ""
	}
	return label + " · " + searchMatchExcerpt(text, positions)
}

// A reading position belongs to content within a named section, independent
// of terminal cells, wrapping, palette, and the sections above it.
type peekReadingAnchor struct {
	section   string
	text      []rune
	character int
}

func peekReadingText(line string) []rune {
	return []rune(strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, ansi.Strip(line)))
}

func (doc inlinePeekDocument) readingAnchor(offset int) peekReadingAnchor {
	for _, section := range doc.sections {
		if offset >= len(section.lines) {
			offset -= len(section.lines)
			continue
		}
		name, _, _ := strings.Cut(section.key, ":")
		anchor := peekReadingAnchor{section: name}
		for i, line := range section.lines {
			text := peekReadingText(line)
			if i < offset {
				anchor.character += len(text)
			}
			anchor.text = append(anchor.text, text...)
		}
		return anchor
	}
	return peekReadingAnchor{}
}

func (doc inlinePeekDocument) offsetForAnchor(anchor peekReadingAnchor) int {
	start := 0
	for _, section := range doc.sections {
		name, _, _ := strings.Cut(section.key, ":")
		if name != anchor.section {
			start += len(section.lines)
			continue
		}
		var text []rune
		ends := make([]int, len(section.lines))
		for i, line := range section.lines {
			text = append(text, peekReadingText(line)...)
			ends[i] = len(text)
		}
		character := relocatedPeekCharacter(anchor.text, text, anchor.character)
		for i, end := range ends {
			if character < end {
				return doc.ClampOffset(start + i)
			}
		}
		return doc.ClampOffset(start + max(len(section.lines)-1, 0))
	}
	// If the section itself was removed, there is no honest place to resume.
	return 0
}

// Preserve a point in the unchanged prefix or suffix of an edit. This covers
// appending text, inserting before the passage, and changes in other sections.
// If the passage itself was replaced, resume at the start of that section.
func relocatedPeekCharacter(before, after []rune, character int) int {
	prefix := 0
	for prefix < min(len(before), len(after)) && before[prefix] == after[prefix] {
		prefix++
	}
	if character < prefix || prefix == len(before) {
		return character
	}
	suffix := 0
	for suffix < min(len(before), len(after))-prefix && before[len(before)-1-suffix] == after[len(after)-1-suffix] {
		suffix++
	}
	if character >= len(before)-suffix {
		return max(character+len(after)-len(before), 0)
	}
	return 0
}

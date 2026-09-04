package outliner

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/query"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

const outlinerJumpHistoryLimit = 100

// outlinerJumpPosition is a position in one stable Outliner projection. Task
// identifies an ordinary live cursor position. Relocation origins deliberately
// leave Task empty so jumping backward prefers the first unaffected task that
// followed the moved target, then the preceding task, before using the old row.
type OutlinerJumpPosition struct {
	Task         taskref.TaskIdentity
	Following    taskref.TaskIdentity
	Preceding    taskref.TaskIdentity
	RowIndex     int
	ScrollOffset int
}

type outlinerRelocationHistory struct {
	Back       []OutlinerJumpPosition
	forward    []OutlinerJumpPosition
	Pending    *OutlinerRelocationCapture
	Projection string
}

func (m *OutlinerModel) CurrentTaskIdentity() taskref.TaskIdentity {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return taskref.TaskIdentity{}
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return taskref.TaskIdentity{}
	}
	return m.TaskIdentityForRow(row)
}

func (m *OutlinerModel) currentJumpPosition() (OutlinerJumpPosition, bool) {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) || !m.Rows[m.Cursor].IsNavigable() {
		return OutlinerJumpPosition{}, false
	}
	return OutlinerJumpPosition{
		Task:         m.CurrentTaskIdentity(),
		RowIndex:     m.Cursor,
		ScrollOffset: m.ViewScrollOffset,
	}, true
}

func (m *OutlinerModel) RowIndexForTask(identity taskref.TaskIdentity) (int, bool) {
	if !identity.Valid() {
		return 0, false
	}
	for i, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && m.TaskIdentityForRow(row) == identity {
			return i, true
		}
	}
	return 0, false
}

func (m *OutlinerModel) resolveJumpPosition(position OutlinerJumpPosition) (int, bool) {
	if index, ok := m.RowIndexForTask(position.Task); ok {
		return index, true
	}
	if index, ok := m.RowIndexForTask(position.Following); ok {
		return index, true
	}
	if index, ok := m.RowIndexForTask(position.Preceding); ok {
		return index, true
	}
	if len(m.Rows) == 0 {
		return 0, false
	}
	origin := position.RowIndex
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.Rows) {
		origin = len(m.Rows) - 1
	}
	for distance := 0; distance < len(m.Rows); distance++ {
		next := origin + distance
		if next < len(m.Rows) && m.Rows[next].IsNavigable() {
			return next, true
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.Rows[previous].IsNavigable() {
			return previous, true
		}
	}
	return 0, false
}

func appendBoundedJump(stack []OutlinerJumpPosition, position OutlinerJumpPosition) []OutlinerJumpPosition {
	stack = append(stack, position)
	if len(stack) > outlinerJumpHistoryLimit {
		stack = append([]OutlinerJumpPosition(nil), stack[len(stack)-outlinerJumpHistoryLimit:]...)
	}
	return stack
}

func (m *OutlinerModel) PushJumpBack(position OutlinerJumpPosition) {
	m.RelocationHistory.Back = appendBoundedJump(m.RelocationHistory.Back, position)
}

func (m *OutlinerModel) JumpBackward() tea.Cmd {
	if m == nil {
		return nil
	}
	m.SyncJumpProjection()
	current, currentOK := m.currentJumpPosition()
	for len(m.RelocationHistory.Back) > 0 {
		last := len(m.RelocationHistory.Back) - 1
		target := m.RelocationHistory.Back[last]
		m.RelocationHistory.Back = m.RelocationHistory.Back[:last]
		index, ok := m.resolveJumpPosition(target)
		if !ok {
			continue
		}
		if currentOK {
			m.RelocationHistory.forward = appendBoundedJump(m.RelocationHistory.forward, current)
		}
		return m.moveToJumpPosition(target, index)
	}
	return nil
}

func (m *OutlinerModel) JumpForward() tea.Cmd {
	if m == nil {
		return nil
	}
	m.SyncJumpProjection()
	current, currentOK := m.currentJumpPosition()
	for len(m.RelocationHistory.forward) > 0 {
		last := len(m.RelocationHistory.forward) - 1
		target := m.RelocationHistory.forward[last]
		m.RelocationHistory.forward = m.RelocationHistory.forward[:last]
		index, ok := m.resolveJumpPosition(target)
		if !ok {
			continue
		}
		if currentOK {
			m.PushJumpBack(current)
		}
		return m.moveToJumpPosition(target, index)
	}
	return nil
}

func (m *OutlinerModel) moveToJumpPosition(position OutlinerJumpPosition, index int) tea.Cmd {
	oldCursor := m.Cursor
	m.Cursor = index
	m.ViewScrollOffset = position.ScrollOffset
	m.VoidRelocationCaptureAfterNavigation(oldCursor)
	return listsurface.FlashIfJumped(oldCursor, index, &m.CursorFlash)
}

// syncJumpProjection invalidates history as soon as presentation state changes,
// including when the user later returns to a projection with the same shape.
func (m *OutlinerModel) SyncJumpProjection() {
	if m == nil {
		return
	}
	projection := m.JumpProjectionFingerprint()
	history := &m.RelocationHistory
	if history.Projection != "" && history.Projection != projection {
		history.Back = nil
		history.forward = nil
		history.Pending = nil
	}
	history.Projection = projection
}

func (m *OutlinerModel) JumpProjectionFingerprint() string {
	var b strings.Builder
	fmt.Fprintf(&b, "project=%q|mode=%s|scope=%q|search=%t:%t:%q|sort=%d:%t|tree=%d:%t|",
		projectIDForJumpProjection(m), m.PrimaryMode, m.Scope.ProjectID,
		m.HasSearchProjection(), m.ShowsRankedList(), m.SearchQuery(),
		m.Sort.Field, m.Sort.Ascending, m.TreeSort.Field, m.TreeSort.Ascending)
	fmt.Fprintf(&b, "filters=%q|", strings.Join(query.CommonFilterParts(&m.Filter.CommonFilterState), "\x1f"))
	fmt.Fprintf(&b, "zoom=%q|", m.Focus.Zoom.key)

	collapsed := make([]string, 0, len(m.Folds.CollapsedSections))
	for key, value := range m.Folds.CollapsedSections {
		if value {
			collapsed = append(collapsed, key)
		}
	}
	sort.Strings(collapsed)
	fmt.Fprintf(&b, "collapsed=%q|", strings.Join(collapsed, "\x1f"))

	folded := make([]string, 0, len(m.Folds.ExpandedNodes))
	for identity, expanded := range m.Folds.ExpandedNodes {
		if !expanded {
			folded = append(folded, identity.ProjectID+"\x1e"+identity.TaskID)
		}
	}
	sort.Strings(folded)
	fmt.Fprintf(&b, "folded=%q|", strings.Join(folded, "\x1f"))

	checklists := make([]string, 0, len(m.Folds.ExpandedChecklists))
	for identity, expanded := range m.Folds.ExpandedChecklists {
		if expanded {
			checklists = append(checklists, identity.ProjectID+"\x1e"+identity.TaskID)
		}
	}
	sort.Strings(checklists)
	fmt.Fprintf(&b, "checklists=%q|", strings.Join(checklists, "\x1f"))

	for _, entry := range m.Focus.Stack {
		fmt.Fprintf(&b, "focus=%s:%d:%d|", entry.TaskID, entry.CursorIndex, entry.ScrollOffset)
	}
	for _, entry := range m.Focus.CrossStack {
		fmt.Fprintf(&b, "cross-focus=%s:%s:%d:%d|", entry.ProjectID, entry.TaskID, entry.CursorIndex, entry.ScrollOffset)
	}
	return b.String()
}

func projectIDForJumpProjection(m *OutlinerModel) string {
	if m != nil && m.Project != nil {
		return m.Project.ID
	}
	return ""
}

// outlinerRelocationCapture travels with a mutation until its committed rows
// are visible. Failed commands never surface the capture to the refresh path.
//
// Settled and SettledCursor exist to tell "the rebuild displaced me" apart from
// "the user pressed j". The committed mutation's own in-memory refresh rebuilds
// rows before the disk reload lands, so a departing task is already gone from
// the projection and the cursor no longer identifies the capture's target. That
// displacement is the mutation's own doing and must not void the capture, while
// a genuine keypress in the same window must. SettledCursor records the cursor
// exactly as that refresh left it, so the reload guard can accept an untouched
// cursor and still discard a moved one.
type OutlinerRelocationCapture struct {
	Task          taskref.TaskIdentity
	Origin        OutlinerJumpPosition
	Projection    string
	TransactionID string
	Settled       bool
	SettledCursor int
}

// StampTransaction satisfies coordinator.TransactionRelocation, which is how a
// capture rides a mutation the coordinator layer commits without that layer
// naming this struct. It returns a copy so the caller's capture is never
// mutated by a command it handed off.
func (c *OutlinerRelocationCapture) StampTransaction(id string) coordinator.TransactionRelocation {
	if c == nil {
		return nil
	}
	stamped := *c
	stamped.TransactionID = id
	return &stamped
}

// outlinerRelocationFrom recovers the shell's own capture from the opaque value
// a coordinator carried across the commit. A nil interface and a value some
// other surface attached both yield nil, which every consumer already treats as
// "nothing to relocate".
func OutlinerRelocationFrom(value coordinator.TransactionRelocation) *OutlinerRelocationCapture {
	capture, _ := value.(*OutlinerRelocationCapture)
	return capture
}

// outlinerOriginRecordedMsg reports that a committed relocation actually added
// an origin to jump history. The shell uses the transaction ID to decorate only
// the still-active matching recovery toast.
type OutlinerOriginRecordedMsg struct{ TransactionID string }

// captureOutlinerRelocation snapshots the cursor's old list neighborhood only
// when the followed task is part of the mutation target set.
func (m *OutlinerModel) CaptureOutlinerRelocation(targets []taskref.TaskIdentity) *OutlinerRelocationCapture {
	if m == nil || len(targets) == 0 || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return nil
	}
	m.SyncJumpProjection()
	cursorTask := m.CurrentTaskIdentity()
	if !cursorTask.Valid() {
		return nil
	}
	targetSet := make(map[taskref.TaskIdentity]bool, len(targets))
	for _, target := range targets {
		if target.Valid() {
			targetSet[target] = true
		}
	}
	if !targetSet[cursorTask] {
		return nil
	}

	origin := OutlinerJumpPosition{RowIndex: m.Cursor, ScrollOffset: m.ViewScrollOffset}
	for i := m.Cursor + 1; i < len(m.Rows); i++ {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if identity.Valid() && !targetSet[identity] {
			origin.Following = identity
			break
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if identity.Valid() && !targetSet[identity] {
			origin.Preceding = identity
			break
		}
	}
	return &OutlinerRelocationCapture{
		Task:       cursorTask,
		Origin:     origin,
		Projection: m.RelocationHistory.Projection,
	}
}

// armOutlinerRelocation retains a committed asynchronous mutation for the next
// accepted Outliner snapshot. If the user left the target or changed projection
// while the command was running, the result must not steal focus later.
func (m *OutlinerModel) ArmOutlinerRelocation(capture *OutlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	m.SyncJumpProjection()
	if capture.Projection != m.RelocationHistory.Projection || m.CurrentTaskIdentity() != capture.Task {
		return
	}
	copy := *capture
	m.RelocationHistory.Pending = &copy
}

// settleRelocationCursor stamps a pending capture with the cursor the committed
// mutation's own refresh just produced. It is called once, immediately after
// that refresh, so any later difference is a cursor move the user made.
func (m *OutlinerModel) SettleRelocationCursor() {
	if m == nil {
		return
	}
	pending := m.RelocationHistory.Pending
	if pending == nil {
		return
	}
	pending.Settled = true
	pending.SettledCursor = m.Cursor
}

// voidRelocationCaptureAfterNavigation drops a pending capture the moment the
// user moves the cursor themselves. discardDisplacedRelocationCapture can only
// compare numbers when the reload lands, and a cursor that leaves SettledCursor
// and returns to it is numerically indistinguishable from one that never moved,
// so the accepted reload would re-arm an advance the user already voided.
// Recording the navigation as it happens is what tells the two apart. Callers
// pass the cursor from before their own move: a refused or boundary-flashed
// navigation moved nothing and voids nothing.
func (m *OutlinerModel) VoidRelocationCaptureAfterNavigation(previous int) {
	if m == nil || m.RelocationHistory.Pending == nil {
		return
	}
	if m.Cursor == previous {
		return
	}
	m.RelocationHistory.Pending = nil
}

// discardDisplacedRelocationCapture drops a pending capture the user has walked
// away from. A user may keep navigating while the committed mutation's disk
// refresh is in flight, and that later snapshot must not steal focus. Two
// cursors are acceptable: the capture's own task, still followed, and the cursor
// exactly as settleRelocationCursor recorded it, which is where the mutation's
// own rebuild left it when the target departed the projection. Anything else is
// a keypress. Callers must run this before rebuilding rows, so the comparison
// reads the cursor that refresh produced rather than a reload artifact. A user
// keypress that lands back on SettledCursor is not visible here at all; that
// case is voided as it happens by voidRelocationCaptureAfterNavigation.
func (m *OutlinerModel) DiscardDisplacedRelocationCapture() {
	if m == nil {
		return
	}
	pending := m.RelocationHistory.Pending
	if pending == nil {
		return
	}
	if m.CurrentTaskIdentity() == pending.Task {
		return
	}
	if pending.Settled && m.Cursor == pending.SettledCursor {
		return
	}
	m.RelocationHistory.Pending = nil
}

func (m *OutlinerModel) FinishPendingOutlinerRelocation() tea.Cmd {
	if m == nil {
		return nil
	}
	capture := m.RelocationHistory.Pending
	m.RelocationHistory.Pending = nil
	jumpCmd, recorded := m.FinishOutlinerRelocation(capture, false)
	if !recorded {
		return jumpCmd
	}
	originCmd := func() tea.Msg { return OutlinerOriginRecordedMsg{TransactionID: capture.TransactionID} }
	return tea.Batch(jumpCmd, originCmd)
}

// finishOutlinerRelocation follows the committed task and records its vacated
// origin only when it remains visible at a different row in the same projection.
// Synchronous planning operations set forceFollow because no user input can have
// occurred between capture and commit.
func (m *OutlinerModel) FinishOutlinerRelocation(capture *OutlinerRelocationCapture, forceFollow bool) (tea.Cmd, bool) {
	if m == nil || capture == nil {
		return nil, false
	}
	m.SyncJumpProjection()
	if capture.Projection != m.RelocationHistory.Projection {
		return nil, false
	}
	// A departure is checked before the forceFollow guard so the synchronous and
	// asynchronous callers reach the same advance. The task is gone from this
	// projection, so there is nothing to follow and no origin worth recording.
	destination, visible := m.RowIndexForTask(capture.Task)
	if !visible {
		m.advanceAfterDeparture(capture)
		return nil, false
	}
	if !forceFollow && m.CurrentTaskIdentity() != capture.Task {
		return nil, false
	}
	if destination == capture.Origin.RowIndex {
		return nil, false
	}
	oldCursor := m.Cursor
	m.Cursor = destination
	m.PushJumpBack(capture.Origin)
	m.RelocationHistory.forward = nil
	return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), true
}

// advanceAfterDeparture carries the cursor onto the successor the relocation
// capture already recorded, for the modes whose catalog entry asks for it. It
// writes m.cursor and nothing else: computeOutlinerViewport remains the single
// window authority, so no ViewScrollOffset write belongs here. No projection
// check is needed either — every caller sits behind finishOutlinerRelocation's
// projection guard, which already covers mode, scope, search, filters, sort,
// zoom, collapsed sections, folds, and focus. No flash is emitted: the user's own
// keypress caused the move and the next row to triage is under the cursor.
func (m *OutlinerModel) advanceAfterDeparture(capture *OutlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	if !m.ActiveOutlinerModeEntry().SupportsCursorAdvance() {
		return
	}
	successor, ok := m.RowIndexForTask(capture.Origin.Following)
	if !ok {
		return
	}
	m.Cursor = successor
}

// finishTaskLineEditRelocation resolves the final project-qualified identity
// after an atomic row edit. A visible result is followed and records the vacated
// origin; a result hidden by the unchanged search/mode/filter/scope projection
// leaves history intact and lands on the nearest surviving origin neighbor.
func (m *OutlinerModel) FinishTaskLineEditRelocation(capture *OutlinerRelocationCapture, final taskref.TaskIdentity) (tea.Cmd, bool) {
	if m == nil || capture == nil || !final.Valid() {
		return nil, false
	}
	m.SyncJumpProjection()
	if capture.Projection != m.RelocationHistory.Projection {
		return nil, false
	}
	if destination, ok := m.RowIndexForTask(final); ok {
		oldCursor := m.Cursor
		m.Cursor = destination
		if destination == capture.Origin.RowIndex {
			return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), false
		}
		m.PushJumpBack(capture.Origin)
		m.RelocationHistory.forward = nil
		return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), true
	}
	nearest, ok := m.resolveJumpPosition(capture.Origin)
	if !ok {
		return nil, false
	}
	oldCursor := m.Cursor
	m.Cursor = nearest
	m.ViewScrollOffset = capture.Origin.ScrollOffset
	return listsurface.FlashIfJumped(oldCursor, nearest, &m.CursorFlash), false
}

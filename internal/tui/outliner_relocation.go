package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

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
type outlinerRelocationCapture struct {
	Task          taskref.TaskIdentity
	Origin        outlinerJumpPosition
	Projection    string
	TransactionID string
	Settled       bool
	SettledCursor int
}

// StampTransaction satisfies coordinator.TransactionRelocation, which is how a
// capture rides a mutation the coordinator layer commits without that layer
// naming this struct. It returns a copy so the caller's capture is never
// mutated by a command it handed off.
func (c *outlinerRelocationCapture) StampTransaction(id string) coordinator.TransactionRelocation {
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
func outlinerRelocationFrom(value coordinator.TransactionRelocation) *outlinerRelocationCapture {
	capture, _ := value.(*outlinerRelocationCapture)
	return capture
}

// outlinerOriginRecordedMsg reports that a committed relocation actually added
// an origin to jump history. The shell uses the transaction ID to decorate only
// the still-active matching recovery toast.
type outlinerOriginRecordedMsg struct{ TransactionID string }

// captureOutlinerRelocation snapshots the cursor's old list neighborhood only
// when the followed task is part of the mutation target set.
func (m *OutlinerModel) captureOutlinerRelocation(targets []taskref.TaskIdentity) *outlinerRelocationCapture {
	if m == nil || len(targets) == 0 || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return nil
	}
	m.syncJumpProjection()
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

	origin := outlinerJumpPosition{RowIndex: m.Cursor, ScrollOffset: m.ViewScrollOffset}
	for i := m.Cursor + 1; i < len(m.rows); i++ {
		row := m.rows[i]
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		identity := m.taskIdentityForRow(row)
		if identity.Valid() && !targetSet[identity] {
			origin.Following = identity
			break
		}
	}
	for i := m.Cursor - 1; i >= 0; i-- {
		row := m.rows[i]
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		identity := m.taskIdentityForRow(row)
		if identity.Valid() && !targetSet[identity] {
			origin.Preceding = identity
			break
		}
	}
	return &outlinerRelocationCapture{
		Task:       cursorTask,
		Origin:     origin,
		Projection: m.relocationHistory.projection,
	}
}

// armOutlinerRelocation retains a committed asynchronous mutation for the next
// accepted Outliner snapshot. If the user left the target or changed projection
// while the command was running, the result must not steal focus later.
func (m *OutlinerModel) armOutlinerRelocation(capture *outlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	m.syncJumpProjection()
	if capture.Projection != m.relocationHistory.projection || m.CurrentTaskIdentity() != capture.Task {
		return
	}
	copy := *capture
	m.relocationHistory.pending = &copy
}

// settleRelocationCursor stamps a pending capture with the cursor the committed
// mutation's own refresh just produced. It is called once, immediately after
// that refresh, so any later difference is a cursor move the user made.
func (m *OutlinerModel) settleRelocationCursor() {
	if m == nil {
		return
	}
	pending := m.relocationHistory.pending
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
func (m *OutlinerModel) voidRelocationCaptureAfterNavigation(previous int) {
	if m == nil || m.relocationHistory.pending == nil {
		return
	}
	if m.Cursor == previous {
		return
	}
	m.relocationHistory.pending = nil
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
func (m *OutlinerModel) discardDisplacedRelocationCapture() {
	if m == nil {
		return
	}
	pending := m.relocationHistory.pending
	if pending == nil {
		return
	}
	if m.CurrentTaskIdentity() == pending.Task {
		return
	}
	if pending.Settled && m.Cursor == pending.SettledCursor {
		return
	}
	m.relocationHistory.pending = nil
}

func (m *OutlinerModel) finishPendingOutlinerRelocation() tea.Cmd {
	if m == nil {
		return nil
	}
	capture := m.relocationHistory.pending
	m.relocationHistory.pending = nil
	jumpCmd, recorded := m.finishOutlinerRelocation(capture, false)
	if !recorded {
		return jumpCmd
	}
	originCmd := func() tea.Msg { return outlinerOriginRecordedMsg{TransactionID: capture.TransactionID} }
	return tea.Batch(jumpCmd, originCmd)
}

// finishOutlinerRelocation follows the committed task and records its vacated
// origin only when it remains visible at a different row in the same projection.
// Synchronous planning operations set forceFollow because no user input can have
// occurred between capture and commit.
func (m *OutlinerModel) finishOutlinerRelocation(capture *outlinerRelocationCapture, forceFollow bool) (tea.Cmd, bool) {
	if m == nil || capture == nil {
		return nil, false
	}
	m.syncJumpProjection()
	if capture.Projection != m.relocationHistory.projection {
		return nil, false
	}
	// A departure is checked before the forceFollow guard so the synchronous and
	// asynchronous callers reach the same advance. The task is gone from this
	// projection, so there is nothing to follow and no origin worth recording.
	destination, visible := m.rowIndexForTask(capture.Task)
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
	m.pushJumpBack(capture.Origin)
	m.relocationHistory.forward = nil
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
func (m *OutlinerModel) advanceAfterDeparture(capture *outlinerRelocationCapture) {
	if m == nil || capture == nil {
		return
	}
	if !m.activeOutlinerModeEntry().SupportsCursorAdvance() {
		return
	}
	successor, ok := m.rowIndexForTask(capture.Origin.Following)
	if !ok {
		return
	}
	m.Cursor = successor
}

// finishTaskLineEditRelocation resolves the final project-qualified identity
// after an atomic row edit. A visible result is followed and records the vacated
// origin; a result hidden by the unchanged search/mode/filter/scope projection
// leaves history intact and lands on the nearest surviving origin neighbor.
func (m *OutlinerModel) finishTaskLineEditRelocation(capture *outlinerRelocationCapture, final taskref.TaskIdentity) (tea.Cmd, bool) {
	if m == nil || capture == nil || !final.Valid() {
		return nil, false
	}
	m.syncJumpProjection()
	if capture.Projection != m.relocationHistory.projection {
		return nil, false
	}
	if destination, ok := m.rowIndexForTask(final); ok {
		oldCursor := m.Cursor
		m.Cursor = destination
		if destination == capture.Origin.RowIndex {
			return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), false
		}
		m.pushJumpBack(capture.Origin)
		m.relocationHistory.forward = nil
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

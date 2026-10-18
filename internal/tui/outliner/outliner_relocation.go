package outliner

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
type OutlinerRelocationCapture struct {
	Task taskref.TaskIdentity
	// Group is the block the cursor row belonged to when the mutation was issued,
	// and it is what decides retention: a committed task found again inside it is
	// the row the user is still pointing at, while one found only outside it left
	// the block and vacated a position the sweep continues from. The row index is
	// deliberately not that test — a rebuild moves every index below an edit — and
	// neither is bare visibility, which is what used to carry the cursor into
	// another lane or another project behind a single keypress.
	Group OutlinerRowGroup
	// PromotedChildren are the cursor task's direct children as the projection
	// held them, minus any that are themselves mutation targets. A parent leaving
	// its group promotes its surviving children into the position it vacated, and
	// that child is the first thing the sweep should land on; it is recorded here
	// because a folded parent renders none of them, so the rebuilt rows cannot be
	// asked afterwards which child took its place.
	PromotedChildren []taskref.TaskIdentity
	// FollowingNeighbors and PrecedingNeighbors are every unselected task the
	// projection drew after and before the cursor row, whole and in the order the
	// sweep reads them: outward from the cursor. One identity on each side is not
	// enough, because a mutation moves more than the rows it names — a moved parent
	// carries its descendants into another project, and the nearest neighbor is
	// exactly the one most likely to be such a descendant. Recording the ordered
	// neighborhood rather than its first entry is what lets the recovery walk past
	// the tasks that departed with the target and land on the first one that
	// survived. They are captured whole and never sampled: a truncated list reads
	// as "nothing survived ahead" and sends the sweep backward, which is the
	// failure a cap would reintroduce.
	FollowingNeighbors []taskref.TaskIdentity
	PrecedingNeighbors []taskref.TaskIdentity
	Origin             OutlinerJumpPosition
	Projection         string
	TransactionID      string
	Settled            bool
	SettledCursor      int

	// ReviewAdvance marks a capture that carries a review acknowledgement rather
	// than a relocation. The two are different questions about the same commit:
	// a relocation asks where the task went, and this asks which candidate the
	// pass moves to next. The flag is separate from ReviewSuccessors because a
	// pass with no successor left is exactly the case that has something to say.
	ReviewAdvance bool
	// ReviewSuccessors is the ordered deduplicated sequence of eligible task
	// identities that followed the cursor when the acknowledgement was issued.
	ReviewSuccessors []taskref.TaskIdentity
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

	cursorRow := m.Rows[m.Cursor]
	origin := OutlinerJumpPosition{RowIndex: m.Cursor, ScrollOffset: m.ViewScrollOffset}
	following := m.unselectedNeighborIdentities(m.Cursor+1, 1, targetSet)
	preceding := m.unselectedNeighborIdentities(m.Cursor-1, -1, targetSet)
	// A jump frame keeps one identity per side: it answers where the user was
	// standing rather than where a sweep continues, and resolveJumpPosition already
	// falls back to the recorded row when neither resolves. The nearest neighbor on
	// each side is that answer, and it is read off the same scan so the frame and
	// the recovery can never disagree about order.
	if len(following) > 0 {
		origin.Following = following[0]
	}
	if len(preceding) > 0 {
		origin.Preceding = preceding[0]
	}
	return &OutlinerRelocationCapture{
		Task:               cursorTask,
		Group:              cursorRow.Group,
		PromotedChildren:   promotableChildIdentities(cursorRow, cursorTask.ProjectID, targetSet),
		FollowingNeighbors: following,
		PrecedingNeighbors: preceding,
		Origin:             origin,
		Projection:         m.RelocationHistory.Projection,
	}
}

// unselectedNeighborIdentities walks the drawn rows outward from start in the
// given direction and collects every task the mutation did not name, in the
// order it meets them. Non-task rows are skipped exactly as the recovery skips
// them, and a duplicate identity — All draws one task in several lanes — is kept
// only once, so the sweep never spends two attempts on one task.
func (m *OutlinerModel) unselectedNeighborIdentities(start, step int, targets map[taskref.TaskIdentity]bool) []taskref.TaskIdentity {
	var identities []taskref.TaskIdentity
	seen := make(map[taskref.TaskIdentity]bool)
	for i := start; i >= 0 && i < len(m.Rows); i += step {
		row := m.Rows[i]
		if !row.IsTaskRow() || row.Task == nil {
			continue
		}
		identity := m.TaskIdentityForRow(row)
		if !identity.Valid() || targets[identity] || seen[identity] {
			continue
		}
		seen[identity] = true
		identities = append(identities, identity)
	}
	return identities
}

// promotableChildIdentities reads the cursor row's own direct children, in the
// order the projection holds them, and drops the ones the mutation also names.
// The row's task carries the projection's tree rather than the stored one, so a
// child already promoted past a filtered ancestor is recorded here exactly as the
// rebuild will re-root it, and a child that is itself a target is left out on the
// same terms as the surrounding tasks: a sweep continues onto work the keypress
// did not touch.
func promotableChildIdentities(row OutlinerRow, projectID string, targets map[taskref.TaskIdentity]bool) []taskref.TaskIdentity {
	if row.Task == nil || len(row.Task.Children) == 0 || projectID == "" {
		return nil
	}
	identities := make([]taskref.TaskIdentity, 0, len(row.Task.Children))
	for _, child := range row.Task.Children {
		if child == nil || child.ID == "" {
			continue
		}
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: child.ID}
		if targets[identity] {
			continue
		}
		identities = append(identities, identity)
	}
	if len(identities) == 0 {
		return nil
	}
	return identities
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

// AdoptNavigatedCursor takes the cursor the user just moved as the one the
// in-flight work has to respect. Callers pass the cursor from before their own
// move: a refused or boundary-flashed navigation moved nothing and is adopted as
// nothing.
//
// It settles two things at once, because one keypress decides both. A pending
// relocation capture is dropped: discardDisplacedRelocationCapture can only
// compare numbers when the reload lands, and a cursor that leaves SettledCursor
// and returns to it is numerically indistinguishable from one that never moved,
// so the accepted reload would re-arm an advance the user already voided, and
// recording the navigation as it happens is what tells the two apart. And a read
// already in flight is re-aimed at the new row, on exactly the terms
// rebaseReloadTargetAfterRelocation states for a relation hop or a scope push:
// its projects are still the newest reading there is and still wanted, only
// where to stand in them has changed. Leaving the read aimed where the
// mutation's own rebuild settled would drag the cursor back off the row the user
// chose, which is the advance the dropped capture was canceling.
func (m *OutlinerModel) AdoptNavigatedCursor(previous int) {
	if m == nil || m.Cursor == previous {
		return
	}
	m.RelocationHistory.Pending = nil
	m.rebaseReloadTargetAfterRelocation()
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
	// A review acknowledgement branches before every follow rule below. Its task
	// usually does not move at all — a Waiting one keeps its row — so the ordinary
	// "did it go somewhere" question has no bearing on where the pass goes next,
	// and it records no origin because nothing was vacated.
	if capture.ReviewAdvance {
		return m.finishReviewAdvance(capture), false
	}
	// A departure is checked before the forceFollow guard so the synchronous and
	// asynchronous callers reach the same recovery. The task left the block the
	// keypress was aimed from — into another lane, another project's group, or out
	// of the projection entirely — so there is nothing here to follow and no origin
	// worth recording: the cursor never went anywhere for `Ctrl+O` to come back
	// from.
	destination, retained := m.rowIndexForTaskInGroup(capture.Task, capture.Group)
	if !retained {
		m.recoverCursorAfterDeparture(capture)
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

// relocationRecoveryTarget names the row that continues the sweep the departing
// task was part of, in the one order every surface uses: the first surviving
// child promoted into the position the parent vacated, then the first surviving
// unselected task that followed it, then the first surviving one that preceded
// it. Every candidate is a task row by construction — CaptureOutlinerRelocation
// records none other and rowIndexForTaskInGroup and RowIndexForTask match none
// other — so a header, a separator, a checklist item, and a note row are all
// skipped without a second predicate.
//
// Each side is walked rather than probed once, because a mutation takes rows it
// never named: the descendants of a moved parent leave with it, and they are the
// rows nearest the cursor. Stopping at the nearest neighbor would read a
// departed child as "nothing survived ahead" and send the sweep backward past
// the successor the user was heading for, so the preceding side is only reached
// once every following neighbor has been tried.
//
// The child is looked for inside the captured group alone, because that is what
// "promoted into the vacated position" means: a child of another lane was
// already rendering somewhere else and the departure did not move it. The two
// neighbors are looked for anywhere, because the sweep crosses a group boundary
// the moment the vacated group runs out of rows.
//
// No projection check is needed: every caller sits behind a projection guard that
// already covers mode, scope, search, filters, sort, zoom, collapsed sections,
// folds, and focus.
func (m *OutlinerModel) relocationRecoveryTarget(capture *OutlinerRelocationCapture) (int, bool) {
	if m == nil || capture == nil {
		return 0, false
	}
	for _, child := range capture.PromotedChildren {
		if index, ok := m.rowIndexForTaskInGroup(child, capture.Group); ok {
			return index, true
		}
	}
	if index, ok := m.firstDrawnNeighbor(capture.FollowingNeighbors); ok {
		return index, true
	}
	if index, ok := m.firstDrawnNeighbor(capture.PrecedingNeighbors); ok {
		return index, true
	}
	return 0, false
}

// firstDrawnNeighbor answers with the row of the first captured neighbor the
// rebuilt projection still draws. Bare visibility is the right test here, unlike
// the retention test above it: the sweep crosses a group boundary the moment the
// vacated group runs out of rows, so a survivor found in the next group is still
// the successor.
func (m *OutlinerModel) firstDrawnNeighbor(neighbors []taskref.TaskIdentity) (int, bool) {
	for _, neighbor := range neighbors {
		if index, ok := m.RowIndexForTask(neighbor); ok {
			return index, true
		}
	}
	return 0, false
}

// recoverCursorAfterDeparture seats the cursor where the sweep continues. It
// writes m.Cursor and nothing else: ComputeOutlinerViewport remains the single
// window authority, so no ViewScrollOffset write belongs here. No flash is
// emitted: the user's own keypress caused the move and the next row to act on is
// under the cursor. With nothing left to recover onto, the rebuild's own clamp
// stands, which is what keeps the existing empty-projection anchoring.
func (m *OutlinerModel) recoverCursorAfterDeparture(capture *OutlinerRelocationCapture) {
	if index, ok := m.relocationRecoveryTarget(capture); ok {
		m.Cursor = index
	}
}

// finishTaskLineEditRelocation resolves the final project-qualified identity
// after an atomic row edit, on the same retention rule every other mutation
// takes. An edit that renamed, reprioritized, or reparented the task inside its
// own block is followed to wherever that block now holds it and records the
// vacated origin, so `Ctrl+O` returns to the row the edit was typed from. An edit
// that spelled a destination — another state, another project, a value the
// narrowing no longer admits — is a departure like any other: the cursor is not
// carried after it, and the sweep continues from the former location instead.
//
// Only this surface keeps the numeric fallback. A row edit is the one mutation
// whose result can be hidden by the projection with no surviving neighbor at all
// — the edit may be the thing that emptied the block — and the origin's own row
// and scroll offset are the last thing left that names where the user was.
func (m *OutlinerModel) FinishTaskLineEditRelocation(capture *OutlinerRelocationCapture, final taskref.TaskIdentity) (tea.Cmd, bool) {
	if m == nil || capture == nil || !final.Valid() {
		return nil, false
	}
	m.SyncJumpProjection()
	if capture.Projection != m.RelocationHistory.Projection {
		return nil, false
	}
	if destination, ok := m.rowIndexForTaskInGroup(final, capture.Group); ok {
		oldCursor := m.Cursor
		m.Cursor = destination
		if destination == capture.Origin.RowIndex {
			return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), false
		}
		m.PushJumpBack(capture.Origin)
		m.RelocationHistory.forward = nil
		return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), true
	}
	nearest, ok := m.relocationRecoveryTarget(capture)
	if !ok {
		if nearest, ok = m.resolveJumpPosition(capture.Origin); !ok {
			return nil, false
		}
	}
	oldCursor := m.Cursor
	m.Cursor = nearest
	m.ViewScrollOffset = capture.Origin.ScrollOffset
	return listsurface.FlashIfJumped(oldCursor, nearest, &m.CursorFlash), false
}

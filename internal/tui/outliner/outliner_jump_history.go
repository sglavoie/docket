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

// rowIndexForTaskInGroup finds the task inside one block of the current
// projection. It is the retention test and the promoted-child test both: All
// draws one task in several lanes, so asking whether an identity is visible
// somewhere cannot tell a task that held its place apart from one that left for
// the Pinned lane, the Done lane, or another project's group.
func (m *OutlinerModel) rowIndexForTaskInGroup(identity taskref.TaskIdentity, group OutlinerRowGroup) (int, bool) {
	if !identity.Valid() {
		return 0, false
	}
	for i, row := range m.Rows {
		if row.Group != group || !row.IsTaskRow() || row.Task == nil {
			continue
		}
		if m.TaskIdentityForRow(row) == identity {
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
	m.AdoptNavigatedCursor(oldCursor)
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
	Origin           OutlinerJumpPosition
	Projection       string
	TransactionID    string
	Settled          bool
	SettledCursor    int

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
		Task:             cursorTask,
		Group:            cursorRow.Group,
		PromotedChildren: promotableChildIdentities(cursorRow, cursorTask.ProjectID, targetSet),
		Origin:           origin,
		Projection:       m.RelocationHistory.Projection,
	}
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
// child promoted into the position the parent vacated, then the first unselected
// task that followed it, then the one that preceded it. Every candidate is a task
// row by construction — CaptureOutlinerRelocation records none other and
// rowIndexForTaskInGroup and RowIndexForTask match none other — so a header, a
// separator, a checklist item, and a note row are all skipped without a second
// predicate.
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
	if index, ok := m.RowIndexForTask(capture.Origin.Following); ok {
		return index, true
	}
	if index, ok := m.RowIndexForTask(capture.Origin.Preceding); ok {
		return index, true
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

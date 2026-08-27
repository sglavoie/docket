package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

const outlinerJumpHistoryLimit = 100

// outlinerJumpPosition is a position in one stable Outliner projection. Task
// identifies an ordinary live cursor position. Relocation origins deliberately
// leave Task empty so jumping backward prefers the first unaffected task that
// followed the moved target, then the preceding task, before using the old row.
type outlinerJumpPosition struct {
	Task         TaskIdentity
	Following    TaskIdentity
	Preceding    TaskIdentity
	RowIndex     int
	ScrollOffset int
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
type outlinerRelocationCapture struct {
	Task          TaskIdentity
	Origin        outlinerJumpPosition
	Projection    string
	TransactionID string
	Settled       bool
	SettledCursor int
}

// outlinerOriginRecordedMsg reports that a committed relocation actually added
// an origin to jump history. The shell uses the transaction ID to decorate only
// the still-active matching recovery toast.
type outlinerOriginRecordedMsg struct{ TransactionID string }

type outlinerRelocationHistory struct {
	back       []outlinerJumpPosition
	forward    []outlinerJumpPosition
	pending    *outlinerRelocationCapture
	projection string
}

func (m *OutlinerModel) currentTaskIdentity() TaskIdentity {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return TaskIdentity{}
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return TaskIdentity{}
	}
	return m.taskIdentityForRow(row)
}

func (m *OutlinerModel) currentJumpPosition() (outlinerJumpPosition, bool) {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) || !m.rows[m.cursor].isNavigable() {
		return outlinerJumpPosition{}, false
	}
	return outlinerJumpPosition{
		Task:         m.currentTaskIdentity(),
		RowIndex:     m.cursor,
		ScrollOffset: m.viewScrollOffset,
	}, true
}

// captureOutlinerRelocation snapshots the cursor's old list neighborhood only
// when the followed task is part of the mutation target set.
func (m *OutlinerModel) captureOutlinerRelocation(targets []TaskIdentity) *outlinerRelocationCapture {
	if m == nil || len(targets) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	m.syncJumpProjection()
	cursorTask := m.currentTaskIdentity()
	if !cursorTask.Valid() {
		return nil
	}
	targetSet := make(map[TaskIdentity]bool, len(targets))
	for _, target := range targets {
		if target.Valid() {
			targetSet[target] = true
		}
	}
	if !targetSet[cursorTask] {
		return nil
	}

	origin := outlinerJumpPosition{RowIndex: m.cursor, ScrollOffset: m.viewScrollOffset}
	for i := m.cursor + 1; i < len(m.rows); i++ {
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
	for i := m.cursor - 1; i >= 0; i-- {
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
	if capture.Projection != m.relocationHistory.projection || m.currentTaskIdentity() != capture.Task {
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
	pending.SettledCursor = m.cursor
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
	if m.cursor == previous {
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
	if m.currentTaskIdentity() == pending.Task {
		return
	}
	if pending.Settled && m.cursor == pending.SettledCursor {
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
	if !forceFollow && m.currentTaskIdentity() != capture.Task {
		return nil, false
	}
	if destination == capture.Origin.RowIndex {
		return nil, false
	}
	oldCursor := m.cursor
	m.cursor = destination
	m.pushJumpBack(capture.Origin)
	m.relocationHistory.forward = nil
	return flashIfJumped(oldCursor, destination, &m.cursorFlash), true
}

// advanceAfterDeparture carries the cursor onto the successor the relocation
// capture already recorded, for the modes whose catalog entry asks for it. It
// writes m.cursor and nothing else: computeOutlinerViewport remains the single
// window authority, so no viewScrollOffset write belongs here. No projection
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
	m.cursor = successor
}

// finishTaskLineEditRelocation resolves the final project-qualified identity
// after an atomic row edit. A visible result is followed and records the vacated
// origin; a result hidden by the unchanged search/mode/filter/scope projection
// leaves history intact and lands on the nearest surviving origin neighbor.
func (m *OutlinerModel) finishTaskLineEditRelocation(capture *outlinerRelocationCapture, final TaskIdentity) (tea.Cmd, bool) {
	if m == nil || capture == nil || !final.Valid() {
		return nil, false
	}
	m.syncJumpProjection()
	if capture.Projection != m.relocationHistory.projection {
		return nil, false
	}
	if destination, ok := m.rowIndexForTask(final); ok {
		oldCursor := m.cursor
		m.cursor = destination
		if destination == capture.Origin.RowIndex {
			return flashIfJumped(oldCursor, destination, &m.cursorFlash), false
		}
		m.pushJumpBack(capture.Origin)
		m.relocationHistory.forward = nil
		return flashIfJumped(oldCursor, destination, &m.cursorFlash), true
	}
	nearest, ok := m.resolveJumpPosition(capture.Origin)
	if !ok {
		return nil, false
	}
	oldCursor := m.cursor
	m.cursor = nearest
	m.viewScrollOffset = capture.Origin.ScrollOffset
	return flashIfJumped(oldCursor, nearest, &m.cursorFlash), false
}

func (m *OutlinerModel) rowIndexForTask(identity TaskIdentity) (int, bool) {
	if !identity.Valid() {
		return 0, false
	}
	for i, row := range m.rows {
		if row.isTaskRow() && row.task != nil && m.taskIdentityForRow(row) == identity {
			return i, true
		}
	}
	return 0, false
}

func (m *OutlinerModel) resolveJumpPosition(position outlinerJumpPosition) (int, bool) {
	if index, ok := m.rowIndexForTask(position.Task); ok {
		return index, true
	}
	if index, ok := m.rowIndexForTask(position.Following); ok {
		return index, true
	}
	if index, ok := m.rowIndexForTask(position.Preceding); ok {
		return index, true
	}
	if len(m.rows) == 0 {
		return 0, false
	}
	origin := position.RowIndex
	if origin < 0 {
		origin = 0
	}
	if origin >= len(m.rows) {
		origin = len(m.rows) - 1
	}
	for distance := 0; distance < len(m.rows); distance++ {
		next := origin + distance
		if next < len(m.rows) && m.rows[next].isNavigable() {
			return next, true
		}
		previous := origin - distance
		if distance > 0 && previous >= 0 && m.rows[previous].isNavigable() {
			return previous, true
		}
	}
	return 0, false
}

func appendBoundedJump(stack []outlinerJumpPosition, position outlinerJumpPosition) []outlinerJumpPosition {
	stack = append(stack, position)
	if len(stack) > outlinerJumpHistoryLimit {
		stack = append([]outlinerJumpPosition(nil), stack[len(stack)-outlinerJumpHistoryLimit:]...)
	}
	return stack
}

func (m *OutlinerModel) pushJumpBack(position outlinerJumpPosition) {
	m.relocationHistory.back = appendBoundedJump(m.relocationHistory.back, position)
}

func (m *OutlinerModel) jumpBackward() tea.Cmd {
	if m == nil {
		return nil
	}
	m.syncJumpProjection()
	current, currentOK := m.currentJumpPosition()
	for len(m.relocationHistory.back) > 0 {
		last := len(m.relocationHistory.back) - 1
		target := m.relocationHistory.back[last]
		m.relocationHistory.back = m.relocationHistory.back[:last]
		index, ok := m.resolveJumpPosition(target)
		if !ok {
			continue
		}
		if currentOK {
			m.relocationHistory.forward = appendBoundedJump(m.relocationHistory.forward, current)
		}
		return m.moveToJumpPosition(target, index)
	}
	return nil
}

func (m *OutlinerModel) jumpForward() tea.Cmd {
	if m == nil {
		return nil
	}
	m.syncJumpProjection()
	current, currentOK := m.currentJumpPosition()
	for len(m.relocationHistory.forward) > 0 {
		last := len(m.relocationHistory.forward) - 1
		target := m.relocationHistory.forward[last]
		m.relocationHistory.forward = m.relocationHistory.forward[:last]
		index, ok := m.resolveJumpPosition(target)
		if !ok {
			continue
		}
		if currentOK {
			m.pushJumpBack(current)
		}
		return m.moveToJumpPosition(target, index)
	}
	return nil
}

func (m *OutlinerModel) moveToJumpPosition(position outlinerJumpPosition, index int) tea.Cmd {
	oldCursor := m.cursor
	m.cursor = index
	m.viewScrollOffset = position.ScrollOffset
	m.voidRelocationCaptureAfterNavigation(oldCursor)
	return flashIfJumped(oldCursor, index, &m.cursorFlash)
}

// syncJumpProjection invalidates history as soon as presentation state changes,
// including when the user later returns to a projection with the same shape.
func (m *OutlinerModel) syncJumpProjection() {
	if m == nil {
		return
	}
	projection := m.jumpProjectionFingerprint()
	history := &m.relocationHistory
	if history.projection != "" && history.projection != projection {
		history.back = nil
		history.forward = nil
		history.pending = nil
	}
	history.projection = projection
}

func (m *OutlinerModel) jumpProjectionFingerprint() string {
	var b strings.Builder
	fmt.Fprintf(&b, "project=%q|mode=%s|scope=%q|search=%t:%q|sort=%d:%t|tree=%d:%t|",
		projectIDForJumpProjection(m), m.primaryMode, m.scopedProjectID,
		m.hasSearchProjection(), m.SearchQuery(),
		m.sort.Field, m.sort.Ascending, m.treeSort.Field, m.treeSort.Ascending)
	fmt.Fprintf(&b, "filters=%q|", strings.Join(commonFilterParts(&m.filter.CommonFilterState), "\x1f"))
	fmt.Fprintf(&b, "zoom=%q|", m.zoom.key)

	collapsed := make([]string, 0, len(m.collapsedSections))
	for key, value := range m.collapsedSections {
		if value {
			collapsed = append(collapsed, key)
		}
	}
	sort.Strings(collapsed)
	fmt.Fprintf(&b, "collapsed=%q|", strings.Join(collapsed, "\x1f"))

	folded := make([]string, 0, len(m.expandedNodes))
	for identity, expanded := range m.expandedNodes {
		if !expanded {
			folded = append(folded, identity.ProjectID+"\x1e"+identity.TaskID)
		}
	}
	sort.Strings(folded)
	fmt.Fprintf(&b, "folded=%q|", strings.Join(folded, "\x1f"))

	checklists := make([]string, 0, len(m.expandedChecklists))
	for identity, expanded := range m.expandedChecklists {
		if expanded {
			checklists = append(checklists, identity.ProjectID+"\x1e"+identity.TaskID)
		}
	}
	sort.Strings(checklists)
	fmt.Fprintf(&b, "checklists=%q|", strings.Join(checklists, "\x1f"))

	for _, entry := range m.focusStack {
		fmt.Fprintf(&b, "focus=%s:%d:%d|", entry.TaskID, entry.CursorIndex, entry.ScrollOffset)
	}
	for _, entry := range m.crossFocusStack {
		fmt.Fprintf(&b, "cross-focus=%s:%s:%d:%d|", entry.ProjectID, entry.TaskID, entry.CursorIndex, entry.ScrollOffset)
	}
	return b.String()
}

func projectIDForJumpProjection(m *OutlinerModel) string {
	if m != nil && m.project != nil {
		return m.project.ID
	}
	return ""
}

func (m *AppModel) captureOutlinerRelocationForTargets(targets ResolvedOperationTargets) *outlinerRelocationCapture {
	out, ok := m.outlinerPrimary()
	if !ok || out == nil {
		return nil
	}
	identities := make([]TaskIdentity, 0, len(targets.Targets))
	for _, target := range targets.Targets {
		if target.Task.Valid() {
			identities = append(identities, target.Task)
		}
	}
	return out.captureOutlinerRelocation(identities)
}

// withOutlinerRelocationCapture decorates only successful batch results. Error
// messages pass through unchanged, so failed mutations cannot pollute history.
func withOutlinerRelocationCapture(cmd tea.Cmd, capture *outlinerRelocationCapture) tea.Cmd {
	if cmd == nil || capture == nil {
		return cmd
	}
	return func() tea.Msg {
		msg := cmd()
		switch result := msg.(type) {
		case BatchTransactionResultMsg:
			copy := *capture
			copy.TransactionID = result.TransactionID
			result.outlinerRelocation = &copy
			return result
		case TransactionResultMsg:
			copy := *capture
			copy.TransactionID = result.Result.TransactionID
			result.outlinerRelocation = &copy
			return result
		default:
			return msg
		}
	}
}

func (m *AppModel) finishSynchronousOutlinerRelocation(cmd tea.Cmd, capture *outlinerRelocationCapture, successMessage, transactionID string) tea.Cmd {
	recorded := false
	var jumpCmd tea.Cmd
	if m != nil && m.crossOutliner != nil && capture != nil {
		jumpCmd, recorded = m.crossOutliner.finishOutlinerRelocation(capture, true)
	}
	feedbackCmd := m.setRecoveryFeedback(successMessage, transactionID, recorded)
	return tea.Batch(cmd, jumpCmd, feedbackCmd)
}

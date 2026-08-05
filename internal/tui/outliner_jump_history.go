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
type outlinerRelocationCapture struct {
	Task       TaskIdentity
	Origin     outlinerJumpPosition
	Projection string
}

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

func (m *OutlinerModel) finishPendingOutlinerRelocation() tea.Cmd {
	if m == nil {
		return nil
	}
	capture := m.relocationHistory.pending
	m.relocationHistory.pending = nil
	return m.finishOutlinerRelocation(capture, false)
}

// finishOutlinerRelocation follows the committed task and records its vacated
// origin only when it remains visible at a different row in the same projection.
// Synchronous planning operations set forceFollow because no user input can have
// occurred between capture and commit.
func (m *OutlinerModel) finishOutlinerRelocation(capture *outlinerRelocationCapture, forceFollow bool) tea.Cmd {
	if m == nil || capture == nil {
		return nil
	}
	m.syncJumpProjection()
	if capture.Projection != m.relocationHistory.projection {
		return nil
	}
	if !forceFollow && m.currentTaskIdentity() != capture.Task {
		return nil
	}
	destination, ok := m.rowIndexForTask(capture.Task)
	if !ok || destination == capture.Origin.RowIndex {
		return nil
	}
	oldCursor := m.cursor
	m.cursor = destination
	m.pushJumpBack(capture.Origin)
	m.relocationHistory.forward = nil
	return flashIfJumped(oldCursor, destination, &m.cursorFlash)
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

	for _, entry := range m.zoomStack {
		fmt.Fprintf(&b, "zoom=%s:%d|", entry.TaskID, entry.CursorIndex)
	}
	for _, entry := range m.crossZoomStack {
		fmt.Fprintf(&b, "cross-zoom=%s:%s:%d|", entry.ProjectID, entry.TaskID, entry.CursorIndex)
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
		batch, ok := msg.(BatchTransactionResultMsg)
		if !ok {
			return msg
		}
		copy := *capture
		batch.outlinerRelocation = &copy
		return batch
	}
}

func (m *AppModel) finishSynchronousOutlinerRelocation(cmd tea.Cmd, capture *outlinerRelocationCapture) tea.Cmd {
	if m == nil || m.crossOutliner == nil || capture == nil {
		return cmd
	}
	jumpCmd := m.crossOutliner.finishOutlinerRelocation(capture, true)
	if jumpCmd == nil {
		return cmd
	}
	return tea.Batch(cmd, jumpCmd)
}

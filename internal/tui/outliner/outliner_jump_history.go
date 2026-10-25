package outliner

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/query"
	"github.com/sglavoie/docket/internal/tui/taskref"
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

// RefocusRestoredTask seats the cursor on a task a ledger restore put back, and
// only on one this projection already draws. It expands nothing, loads nothing,
// and changes no mode, scope, query, or fold: a restored task the current view
// does not admit is the view's answer, not an obstacle, and the caller names
// where the task went instead of rewriting the projection to show it.
//
// A visual selection is left strictly alone, because the selection and the rows
// it counts are the user's standing state and an undo is not a selection
// gesture.
//
// seated reports that the cursor is on the restored task — whether it had to
// move or was already there. originRecorded reports that the move actually
// vacated a position worth returning to, so `Ctrl+O` is advertised only where
// that frame exists: an undo that moved nothing pushes nothing.
func (m *OutlinerModel) RefocusRestoredTask(identity taskref.TaskIdentity) (cmd tea.Cmd, seated, originRecorded bool) {
	if m == nil || !identity.Valid() || m.IsMultiSelectActive() {
		return nil, false, false
	}
	destination, visible := m.RowIndexForTask(identity)
	if !visible {
		return nil, false, false
	}
	if destination == m.Cursor {
		return nil, true, false
	}
	m.SyncJumpProjection()
	origin, hasOrigin := m.currentJumpPosition()
	oldCursor := m.Cursor
	m.Cursor = destination
	if hasOrigin {
		m.PushJumpBack(origin)
		m.RelocationHistory.forward = nil
	}
	m.AdoptNavigatedCursor(oldCursor)
	return listsurface.FlashIfJumped(oldCursor, destination, &m.CursorFlash), true, hasOrigin
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

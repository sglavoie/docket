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
	fmt.Fprintf(&b, "project=%q|mode=%s|scope=%q|search=%t:%t:%q|sort=%d:%t|tree=%d:%t|",
		projectIDForJumpProjection(m), m.primaryMode, m.scopedProjectID,
		m.hasSearchProjection(), m.showsRankedList(), m.SearchQuery(),
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

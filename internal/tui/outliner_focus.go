package tui

import "github.com/sglavoie/tt/internal/tui/taskref"

// focusSubtreeAtCursor enters a descendants-only projection rooted at the
// selected task. Each frame retains the exact parent cursor and scroll offset,
// so nested Focus levels unwind without moving the viewport.
func (m *OutlinerModel) focusSubtreeAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.showsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.Cursor]
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	identity := m.taskIdentityForRow(row)
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil || len(task.Children) == 0 {
		return false
	}

	m.pushFocusFrame(identity, m.Cursor, m.ViewScrollOffset)
	m.peekMode = false
	m.Cursor = 0
	m.ViewScrollOffset = 0
	m.rebuildRows()
	return true
}

// focusParentAtCursor widens Focus by exactly one level, keeping the cursor on
// the task it was pressed from so its siblings come into view. It reads the
// cursor only to open Focus; once focused it climbs from the focus root, which
// is the sole rule that terminates. Widening toward the cursor's parent instead
// would push a nested frame back down whenever the cursor sat deeper than a
// direct child, and the next press would climb out of it again forever.
func (m *OutlinerModel) focusParentAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.showsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.Cursor]
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	origin := m.taskIdentityForRow(row)
	if _, focused := m.focusRootIdentity(); focused {
		m.climbFocusLevel()
	} else {
		task, _ := m.taskAndProjectForIdentity(origin)
		parentID := parentIDOf(task)
		if parentID == "" {
			return false
		}
		m.pushFocusFrame(
			taskref.TaskIdentity{ProjectID: origin.ProjectID, TaskID: parentID},
			m.Cursor,
			m.ViewScrollOffset,
		)
	}
	m.peekMode = false
	m.rebuildRows()
	m.moveCursorToIdentity(origin)
	return true
}

// pushFocusFrame records one Focus frame in whichever stack the current scope
// owns, retaining the cursor and scroll offset Esc restores.
func (m *OutlinerModel) pushFocusFrame(identity taskref.TaskIdentity, cursorIndex, scrollOffset int) {
	if m.project != nil {
		m.focus.push(FocusEntry{
			TaskID:       identity.TaskID,
			CursorIndex:  cursorIndex,
			ScrollOffset: scrollOffset,
		})
		return
	}
	m.focus.pushCross(CrossProjectFocusEntry{
		TaskID:       identity.TaskID,
		ProjectID:    identity.ProjectID,
		CursorIndex:  cursorIndex,
		ScrollOffset: scrollOffset,
	})
	m.population.invalidate()
}

// climbFocusLevel replaces the top frame with one rooted at the focus root's own
// parent. Popping alone already lands there when the frame beneath is that
// parent; otherwise the popped frame spanned more than one level and the parent
// is pushed back, which keeps descending from the frame beneath it.
func (m *OutlinerModel) climbFocusLevel() {
	root, focused := m.focusRootIdentity()
	if !focused {
		return
	}
	rootTask, _ := m.taskAndProjectForIdentity(root)
	cursorIndex, scrollOffset := m.popFocusFrame()
	m.Cursor, m.ViewScrollOffset = cursorIndex, scrollOffset

	parent := taskref.TaskIdentity{ProjectID: root.ProjectID, TaskID: parentIDOf(rootTask)}
	if parent.TaskID == "" {
		return
	}
	if beneath, stillFocused := m.focusRootIdentity(); stillFocused && beneath == parent {
		return
	}
	m.pushFocusFrame(parent, cursorIndex, scrollOffset)
}

// popFocusFrame removes the top Focus frame without rebuilding, returning the
// viewport it retained. popFocusLevel is Esc's inverse of Enter and rebuilds;
// the climb needs the frame gone before it decides what to push in its place.
func (m *OutlinerModel) popFocusFrame() (cursorIndex, scrollOffset int) {
	if top, popped := m.focus.pop(); popped {
		return top.CursorIndex, top.ScrollOffset
	}
	if top, popped := m.focus.popCross(); popped {
		m.population.invalidate()
		return top.CursorIndex, top.ScrollOffset
	}
	return m.Cursor, m.ViewScrollOffset
}

// focusRootIdentity names the task the current Focus projection is rooted at.
// Rebuild-time focus-stack validation keeps the frame live, so a caller still
// resolves the identity against loaded state before acting on it.
func (m *OutlinerModel) focusRootIdentity() (taskref.TaskIdentity, bool) {
	if m == nil {
		return taskref.TaskIdentity{}, false
	}
	var identity taskref.TaskIdentity
	switch {
	case len(m.focus.crossStack) > 0:
		top := m.focus.crossStack[len(m.focus.crossStack)-1]
		identity = taskref.TaskIdentity{ProjectID: top.ProjectID, TaskID: top.TaskID}
	case m.project != nil && len(m.focus.stack) > 0:
		top := m.focus.stack[len(m.focus.stack)-1]
		identity = taskref.TaskIdentity{ProjectID: m.project.ID, TaskID: top.TaskID}
	default:
		return taskref.TaskIdentity{}, false
	}
	return identity, identity.Valid()
}

// popFocusLevel restores one Focus frame. It is deliberately one-level-at-a-
// time so Esc remains a predictable inverse of nested Enter operations.
func (m *OutlinerModel) popFocusLevel() bool {
	if m == nil {
		return false
	}
	if top, popped := m.focus.pop(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.rebuildRows()
		return true
	}
	if top, popped := m.focus.popCross(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.population.invalidate()
		m.rebuildRows()
		return true
	}
	return false
}

// enterSectionZoom narrows the projection to the one section the header names,
// retaining the viewport Esc restores. Unlike Focus, the world narrows around a
// stationary cursor: the zoomed header is the surviving projection's first row,
// so the cursor keeps naming the row it was pressed on. Re-entering the section
// already zoomed to changes nothing, which keeps the retained viewport the
// pre-zoom one.
func (m *OutlinerModel) enterSectionZoom(row outlinerRow) bool {
	if m == nil || m.MultiSelectMode || m.showsRankedList() || !row.IsSectionHeader {
		return false
	}
	key := row.effectiveCollapseKey()
	if m.focus.zoom.key == key {
		return true
	}
	m.focus.enterZoom(key, row.SectionName, m.Cursor, m.ViewScrollOffset)
	m.peekMode = false
	m.population.invalidate()
	m.rebuildRows()
	m.moveCursorToHeaderKey(key)
	return true
}

// exitSectionZoom widens back to the whole mode, restoring the viewport the zoom
// was entered from. Work completed inside the zoom can have shortened the list
// that cursor was taken against, so the restore clamps.
func (m *OutlinerModel) exitSectionZoom() bool {
	if m == nil || !m.focus.zoom.active() {
		return false
	}
	cursor, scroll := m.focus.exitZoom()
	m.Cursor = cursor
	m.ViewScrollOffset = scroll
	m.population.invalidate()
	m.rebuildRows()
	m.clampCursor()
	return true
}

// clearSectionZoom drops the zoom for a transition that reshapes the projection
// on its own terms — a scope push, a mode change, a reveal — so the pre-zoom
// viewport exitSectionZoom would restore is discarded with it. The force flag is
// set here rather than left to the fingerprint so every clear invalidates the
// cross-project memoization guard the same way a scope push does.
func (m *OutlinerModel) clearSectionZoom() {
	if m == nil || !m.focus.zoom.active() {
		return
	}
	m.focus.clearZoom()
	m.population.invalidate()
}

// zoomSectionAtCursor narrows the Outliner to the section the cursor header
// names.
func (m *OutlinerModel) zoomSectionAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	return m.enterSectionZoom(m.rows[m.Cursor])
}

// activateGroupAtCursor scopes the Outliner to the project the cursor sub-header
// names. Enter on a section header is the separate zoom act.
func (m *OutlinerModel) activateGroupAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.Cursor]
	if !row.IsProjectHeader || row.ProjectID == "" {
		return false
	}
	m.pushProjectScope(row.ProjectID)
	return true
}

package outliner

import (
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// focusSubtreeAtCursor enters a descendants-only projection rooted at the
// selected task. Each frame retains the exact parent cursor and scroll offset,
// so nested Focus levels unwind without moving the viewport.
func (m *OutlinerModel) FocusSubtreeAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	identity := m.TaskIdentityForRow(row)
	task, _ := m.TaskAndProjectForIdentity(identity)
	if task == nil || len(task.Children) == 0 {
		return false
	}

	m.pushFocusFrame(identity, m.Cursor, m.ViewScrollOffset)
	m.PeekMode = false
	m.Cursor = 0
	m.ViewScrollOffset = 0
	m.RebuildRows()
	m.seatCursorAtProjectionTop()
	return true
}

// seatCursorAtProjectionTop puts the cursor on the first row of a projection
// that was just rebuilt from scratch, skipping the focused-project header when
// content follows it. It is called only where the cursor was deliberately reset
// to the top — a scope push and a subtree focus — and never on an ordinary
// rebuild, where the header is present on both sides of the pass and every
// captured index already counts it. The header is still where the cursor lands
// when the projection holds nothing else, which is what keeps an empty project
// reachable.
func (m *OutlinerModel) seatCursorAtProjectionTop() {
	m.Cursor = 0
	if len(m.Rows) > 1 && m.Rows[0].IsFocusedProjectHeader {
		m.Cursor = 1
	}
	m.ClampCursor()
}

// focusParentAtCursor widens Focus by exactly one level, keeping the cursor on
// the task it was pressed from so its siblings come into view. It reads the
// cursor only to open Focus; once focused it climbs from the focus root, which
// is the sole rule that terminates. Widening toward the cursor's parent instead
// would push a nested frame back down whenever the cursor sat deeper than a
// direct child, and the next press would climb out of it again forever.
func (m *OutlinerModel) FocusParentAtCursor() bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() ||
		m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	if !row.IsTaskRow() || row.Task == nil {
		return false
	}
	origin := m.TaskIdentityForRow(row)
	if _, focused := m.FocusRootIdentity(); focused {
		m.climbFocusLevel()
	} else {
		task, _ := m.TaskAndProjectForIdentity(origin)
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
	m.PeekMode = false
	m.RebuildRows()
	m.MoveCursorToIdentity(origin)
	return true
}

// pushFocusFrame records one Focus frame in whichever stack the current scope
// owns, retaining the cursor and scroll offset Esc restores.
func (m *OutlinerModel) pushFocusFrame(identity taskref.TaskIdentity, cursorIndex, scrollOffset int) {
	if m.Project != nil {
		m.Focus.push(FocusEntry{
			TaskID:       identity.TaskID,
			CursorIndex:  cursorIndex,
			ScrollOffset: scrollOffset,
		})
		return
	}
	m.Focus.pushCross(CrossProjectFocusEntry{
		TaskID:       identity.TaskID,
		ProjectID:    identity.ProjectID,
		CursorIndex:  cursorIndex,
		ScrollOffset: scrollOffset,
	})
	m.Population.Invalidate()
}

// climbFocusLevel replaces the top frame with one rooted at the focus root's own
// parent. Popping alone already lands there when the frame beneath is that
// parent; otherwise the popped frame spanned more than one level and the parent
// is pushed back, which keeps descending from the frame beneath it.
func (m *OutlinerModel) climbFocusLevel() {
	root, focused := m.FocusRootIdentity()
	if !focused {
		return
	}
	rootTask, _ := m.TaskAndProjectForIdentity(root)
	cursorIndex, scrollOffset := m.popFocusFrame()
	m.Cursor, m.ViewScrollOffset = cursorIndex, scrollOffset

	parent := taskref.TaskIdentity{ProjectID: root.ProjectID, TaskID: parentIDOf(rootTask)}
	if parent.TaskID == "" {
		return
	}
	if beneath, stillFocused := m.FocusRootIdentity(); stillFocused && beneath == parent {
		return
	}
	m.pushFocusFrame(parent, cursorIndex, scrollOffset)
}

// popFocusFrame removes the top Focus frame without rebuilding, returning the
// viewport it retained. popFocusLevel is Esc's inverse of Enter and rebuilds;
// the climb needs the frame gone before it decides what to push in its place.
func (m *OutlinerModel) popFocusFrame() (cursorIndex, scrollOffset int) {
	if top, popped := m.Focus.pop(); popped {
		return top.CursorIndex, top.ScrollOffset
	}
	if top, popped := m.Focus.popCross(); popped {
		m.Population.Invalidate()
		return top.CursorIndex, top.ScrollOffset
	}
	return m.Cursor, m.ViewScrollOffset
}

// focusRootIdentity names the task the current Focus projection is rooted at.
// Rebuild-time focus-stack validation keeps the frame live, so a caller still
// resolves the identity against loaded state before acting on it.
func (m *OutlinerModel) FocusRootIdentity() (taskref.TaskIdentity, bool) {
	if m == nil {
		return taskref.TaskIdentity{}, false
	}
	var identity taskref.TaskIdentity
	switch {
	case len(m.Focus.CrossStack) > 0:
		top := m.Focus.CrossStack[len(m.Focus.CrossStack)-1]
		identity = taskref.TaskIdentity{ProjectID: top.ProjectID, TaskID: top.TaskID}
	case m.Project != nil && len(m.Focus.Stack) > 0:
		top := m.Focus.Stack[len(m.Focus.Stack)-1]
		identity = taskref.TaskIdentity{ProjectID: m.Project.ID, TaskID: top.TaskID}
	default:
		return taskref.TaskIdentity{}, false
	}
	return identity, identity.Valid()
}

// popFocusLevel restores one Focus frame. It is deliberately one-level-at-a-
// time so Esc remains a predictable inverse of nested Enter operations.
func (m *OutlinerModel) PopFocusLevel() bool {
	if m == nil {
		return false
	}
	if top, popped := m.Focus.pop(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.RebuildRows()
		return true
	}
	if top, popped := m.Focus.popCross(); popped {
		m.Cursor = top.CursorIndex
		m.ViewScrollOffset = top.ScrollOffset
		m.Population.Invalidate()
		m.RebuildRows()
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
func (m *OutlinerModel) EnterSectionZoom(row OutlinerRow) bool {
	if m == nil || m.MultiSelectMode || m.ShowsRankedList() || !row.IsSectionHeader {
		return false
	}
	key := row.EffectiveCollapseKey()
	if m.Focus.Zoom.key == key {
		return true
	}
	m.Focus.enterZoom(key, row.SectionName, m.Cursor, m.ViewScrollOffset)
	m.PeekMode = false
	m.Population.Invalidate()
	m.RebuildRows()
	if !m.moveCursorToGroupKey(key) {
		m.ClampCursor()
	}
	return true
}

// exitSectionZoom widens back to the whole mode, restoring the viewport the zoom
// was entered from. Work completed inside the zoom can have shortened the list
// that cursor was taken against, so the restore clamps.
func (m *OutlinerModel) ExitSectionZoom() bool {
	if m == nil || !m.Focus.Zoom.Active() {
		return false
	}
	cursor, scroll := m.Focus.exitZoom()
	m.Cursor = cursor
	m.ViewScrollOffset = scroll
	m.Population.Invalidate()
	m.RebuildRows()
	m.ClampCursor()
	return true
}

// clearSectionZoom drops the zoom for a transition that reshapes the projection
// on its own terms — a scope push, a mode change, a reveal — so the pre-zoom
// viewport exitSectionZoom would restore is discarded with it. The force flag is
// set here rather than left to the fingerprint so every clear invalidates the
// cross-project memoization guard the same way a scope push does.
func (m *OutlinerModel) clearSectionZoom() {
	if m == nil || !m.Focus.Zoom.Active() {
		return
	}
	m.Focus.clearZoom()
	m.Population.Invalidate()
}

// zoomSectionAtCursor narrows the Outliner to the section the cursor header
// names.
func (m *OutlinerModel) ZoomSectionAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	return m.EnterSectionZoom(m.Rows[m.Cursor])
}

// activateGroupAtCursor scopes the Outliner to the project the cursor sub-header
// names. Enter on a section header is the separate zoom act.
func (m *OutlinerModel) ActivateGroupAtCursor() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.Rows) {
		return false
	}
	row := m.Rows[m.Cursor]
	// The focused-project header names the scope the Outliner is already in, so
	// Enter there has nowhere to go. It refuses here as well as in the
	// descriptor's applicability, so the key is silent rather than reporting a
	// scope push that pushed nothing.
	if !row.IsProjectHeader || row.IsFocusedProjectHeader || row.ProjectID == "" {
		return false
	}
	m.PushProjectScope(row.ProjectID)
	return true
}

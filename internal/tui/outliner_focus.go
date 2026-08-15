package tui

// focusSubtreeAtCursor enters a descendants-only projection rooted at the
// selected task. Each frame retains the exact parent cursor and scroll offset,
// so nested Focus levels unwind without moving the viewport.
func (m *OutlinerModel) focusSubtreeAtCursor() bool {
	if m == nil || m.multiSelectMode || m.hasSearchProjection() ||
		m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return false
	}
	identity := m.taskIdentityForRow(row)
	task, _ := m.taskAndProjectForIdentity(identity)
	if task == nil || len(task.Children) == 0 {
		return false
	}

	if m.project != nil {
		m.focusStack = append(m.focusStack, FocusEntry{
			TaskID:       identity.TaskID,
			CursorIndex:  m.cursor,
			ScrollOffset: m.viewScrollOffset,
		})
	} else {
		m.crossFocusStack = append(m.crossFocusStack, CrossProjectFocusEntry{
			TaskID:       identity.TaskID,
			ProjectID:    identity.ProjectID,
			CursorIndex:  m.cursor,
			ScrollOffset: m.viewScrollOffset,
		})
		m.crossForceRebuild = true
	}
	m.peekMode = false
	m.cursor = 0
	m.viewScrollOffset = 0
	m.rebuildRows()
	return true
}

// focusRootIdentity names the task the current Focus projection is rooted at.
// Rebuild-time focus-stack validation keeps the frame live, so a caller still
// resolves the identity against loaded state before acting on it.
func (m *OutlinerModel) focusRootIdentity() (TaskIdentity, bool) {
	if m == nil {
		return TaskIdentity{}, false
	}
	var identity TaskIdentity
	switch {
	case len(m.crossFocusStack) > 0:
		top := m.crossFocusStack[len(m.crossFocusStack)-1]
		identity = TaskIdentity{ProjectID: top.ProjectID, TaskID: top.TaskID}
	case m.project != nil && len(m.focusStack) > 0:
		top := m.focusStack[len(m.focusStack)-1]
		identity = TaskIdentity{ProjectID: m.project.ID, TaskID: top.TaskID}
	default:
		return TaskIdentity{}, false
	}
	return identity, identity.Valid()
}

// popFocusLevel restores one Focus frame. It is deliberately one-level-at-a-
// time so Esc remains a predictable inverse of nested Enter operations.
func (m *OutlinerModel) popFocusLevel() bool {
	if m == nil {
		return false
	}
	if len(m.focusStack) > 0 {
		top := m.focusStack[len(m.focusStack)-1]
		m.focusStack = m.focusStack[:len(m.focusStack)-1]
		m.cursor = top.CursorIndex
		m.viewScrollOffset = top.ScrollOffset
		m.rebuildRows()
		return true
	}
	if len(m.crossFocusStack) > 0 {
		top := m.crossFocusStack[len(m.crossFocusStack)-1]
		m.crossFocusStack = m.crossFocusStack[:len(m.crossFocusStack)-1]
		m.cursor = top.CursorIndex
		m.viewScrollOffset = top.ScrollOffset
		m.crossForceRebuild = true
		m.rebuildRows()
		return true
	}
	return false
}

// activateGroupAtCursor preserves Enter's non-task behavior: section headers
// toggle their rows and project headers focus the Outliner on that project.
func (m *OutlinerModel) activateGroupAtCursor() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	row := m.rows[m.cursor]
	switch {
	case row.IsProjectHeader && row.ProjectID != "":
		m.pushProjectScope(row.ProjectID)
		return true
	case row.IsSectionHeader:
		m.setSectionRowCollapsed(row, !m.isSectionRowCollapsed(row))
		m.rebuildRows()
		return true
	default:
		return false
	}
}

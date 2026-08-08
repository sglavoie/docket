package tui

import (
	tea "charm.land/bubbletea/v2"
)

// isPinnedReorderRow reports whether J/K reorders the pin order rather than
// task hierarchy or a durable planning list. A pinned row answers before the
// active mode's own ordering, so Today's durable plan order does not claim it.
func (m *OutlinerModel) isPinnedReorderRow() bool {
	if m == nil || m.cursor < 0 || m.cursor >= len(m.rows) {
		return false
	}
	if m.multiSelectMode {
		return false
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil || !row.IsPinnedSection {
		return false
	}
	return m.resolveProjectForRow(row) != nil
}

// pinnedSectionParentAt returns the section-local parent of the pinned row at
// index, or the zero identity when the row sits at the section's root level.
// It reproduces the edge rule buildSectionRowForest applies: a parent link is
// retained only when the parent is itself projected into the same group. The
// backward scan is sufficient because the flattener always emits a parent above
// its children and only descends into expanded parents.
func (m *OutlinerModel) pinnedSectionParentAt(index int) TaskIdentity {
	if index < 0 || index >= len(m.rows) {
		return TaskIdentity{}
	}
	row := m.rows[index]
	if row.task == nil || row.task.ParentID == nil {
		return TaskIdentity{}
	}
	parent := TaskIdentity{
		ProjectID: m.taskIdentityForRow(row).ProjectID,
		TaskID:    *row.task.ParentID,
	}
	for i := index - 1; i >= 0; i-- {
		candidate := m.rows[i]
		if !candidate.IsPinnedSection || !candidate.isTaskRow() ||
			candidate.task == nil || candidate.ProjectID != row.ProjectID {
			return TaskIdentity{}
		}
		if m.taskIdentityForRow(candidate) == parent {
			return parent
		}
	}
	return TaskIdentity{}
}

// adjacentVisiblePinnedSibling returns the index of the nearest visible pinned
// row in the delta direction that shares the cursor's section-local parent.
// Rows belonging to a sibling's expanded subtree are stepped over; a section
// header, project header, separator, non-pinned row, or foreign project bounds
// the search, so a move can never leave the cursor's project group.
func (m *OutlinerModel) adjacentVisiblePinnedSibling(delta int) (int, bool) {
	step := 1
	if delta < 0 {
		step = -1
	}
	cursorRow := m.rows[m.cursor]
	cursorParent := m.pinnedSectionParentAt(m.cursor)
	for i := m.cursor + step; i >= 0 && i < len(m.rows); i += step {
		row := m.rows[i]
		if !row.IsPinnedSection || !row.isTaskRow() ||
			row.task == nil || row.ProjectID != cursorRow.ProjectID {
			return 0, false
		}
		if m.pinnedSectionParentAt(i) == cursorParent {
			return i, true
		}
	}
	return 0, false
}

// pinnedReorderMove swaps the cursor's pin order with the adjacent visible
// pinned sibling. The identity that must end up displayed first is passed
// first: pinned rows sort PinnedAt-descending and the application layer
// resolves an exact tie in favor of its first argument.
func (m *OutlinerModel) pinnedReorderMove(delta int) tea.Cmd {
	if !m.isPinnedReorderRow() || m.taskItem.task == nil {
		return nil
	}
	targetIndex, ok := m.adjacentVisiblePinnedSibling(delta)
	if !ok {
		return nil
	}
	cursor := m.taskIdentityForRow(m.rows[m.cursor])
	target := m.taskIdentityForRow(m.rows[targetIndex])
	if !cursor.Valid() || !target.Valid() {
		return nil
	}
	if delta < 0 {
		return m.taskItem.ReorderPinned(cursor, target)
	}
	return m.taskItem.ReorderPinned(target, cursor)
}

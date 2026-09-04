package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// isPinnedReorderRow reports whether J/K reorders the pin order rather than
// task hierarchy or a durable planning list. A pinned row answers before the
// active mode's own ordering, so Today's durable plan order does not claim it.
func (m *OutlinerModel) isPinnedReorderRow() bool {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return false
	}
	if m.MultiSelectMode {
		return false
	}
	row := m.rows[m.Cursor]
	if !row.isTaskRow() || row.task == nil || !row.IsPinnedSection {
		return false
	}
	return m.resolveProjectForRow(row) != nil
}

// pinnedSectionParentAt returns the section-local parent of the pinned row at
// index, or the zero identity when the row sits at the section's root level.
// It reproduces the edge rule buildSectionRowForestWith applies: a parent link is
// retained only when the parent is itself projected into the same group. The
// backward scan is sufficient because the flattener always emits a parent above
// its children and only descends into expanded parents.
func (m *OutlinerModel) pinnedSectionParentAt(index int) taskref.TaskIdentity {
	if index < 0 || index >= len(m.rows) {
		return taskref.TaskIdentity{}
	}
	row := m.rows[index]
	if row.task == nil || row.task.ParentID == nil {
		return taskref.TaskIdentity{}
	}
	parent := taskref.TaskIdentity{
		ProjectID: m.taskIdentityForRow(row).ProjectID,
		TaskID:    *row.task.ParentID,
	}
	for i := index - 1; i >= 0; i-- {
		candidate := m.rows[i]
		if !candidate.IsPinnedSection || !candidate.isTaskRow() ||
			candidate.task == nil || candidate.ProjectID != row.ProjectID {
			return taskref.TaskIdentity{}
		}
		if m.taskIdentityForRow(candidate) == parent {
			return parent
		}
	}
	return taskref.TaskIdentity{}
}

// visiblePinnedSiblings returns every visible pinned row index in the delta
// direction that shares the cursor's section-local parent, nearest first. Rows
// belonging to a sibling's expanded subtree are stepped over; a section header,
// project header, separator, non-pinned row, or foreign project bounds the
// search, so a move can never leave the cursor's project group.
func (m *OutlinerModel) visiblePinnedSiblings(delta int) []int {
	if m == nil || m.Cursor < 0 || m.Cursor >= len(m.rows) {
		return nil
	}
	step := 1
	if delta < 0 {
		step = -1
	}
	cursorRow := m.rows[m.Cursor]
	cursorParent := m.pinnedSectionParentAt(m.Cursor)
	var siblings []int
	for i := m.Cursor + step; i >= 0 && i < len(m.rows); i += step {
		row := m.rows[i]
		if !row.IsPinnedSection || !row.isTaskRow() ||
			row.task == nil || row.ProjectID != cursorRow.ProjectID {
			break
		}
		if m.pinnedSectionParentAt(i) == cursorParent {
			siblings = append(siblings, i)
		}
	}
	return siblings
}

// adjacentVisiblePinnedSibling returns the index of the nearest visible pinned
// sibling in the delta direction.
func (m *OutlinerModel) adjacentVisiblePinnedSibling(delta int) (int, bool) {
	siblings := m.visiblePinnedSiblings(delta)
	if len(siblings) == 0 {
		return 0, false
	}
	return siblings[0], true
}

// pinnedReorderMove swaps the cursor's pin order with the adjacent visible
// pinned sibling. The identity that must end up displayed first is passed
// first: pinned rows sort PinnedAt-descending and the application layer
// resolves an exact tie in favor of its first argument.
func (m *OutlinerModel) pinnedReorderMove(delta int) tea.Cmd {
	if !m.isPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	targetIndex, ok := m.adjacentVisiblePinnedSibling(delta)
	if !ok {
		return nil
	}
	cursor := m.taskIdentityForRow(m.rows[m.Cursor])
	target := m.taskIdentityForRow(m.rows[targetIndex])
	if !cursor.Valid() || !target.Valid() {
		return nil
	}
	if delta < 0 {
		return m.taskItem.ReorderPinned(cursor, target)
	}
	return m.taskItem.ReorderPinned(target, cursor)
}

// pinnedReorderEdge sends the cursor's pin to the far end of its visible pinned
// siblings in one transaction. The order handed down is display order, and it
// names only the peers the cursor would have swapped past one at a time, so the
// redistributed timestamps stay inside that run.
func (m *OutlinerModel) pinnedReorderEdge(delta int) tea.Cmd {
	if !m.isPinnedReorderRow() || !m.taskItem.Wired() {
		return nil
	}
	rows := m.visiblePinnedSiblings(delta)
	if len(rows) == 0 {
		return nil
	}
	peers := make([]taskref.TaskIdentity, 0, len(rows))
	for _, index := range rows {
		peers = append(peers, m.taskIdentityForRow(m.rows[index]))
	}
	moved := m.taskIdentityForRow(m.rows[m.Cursor])
	order := edgeMoveOrder(moved, peers, delta)
	for _, identity := range order {
		if !identity.Valid() {
			return nil
		}
	}
	return m.taskItem.ReorderPinnedOrder(moved, order, reorderEdgeLabel(delta))
}

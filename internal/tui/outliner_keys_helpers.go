package tui

import "github.com/sglavoie/tt/internal/models"

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.expandedNodes) == 0 {
		m.expandedNodes = make(map[TaskIdentity]bool)
		if m.project != nil {
			m.markAllExpanded(m.project.ID, m.project.GetRootTasks())
		} else {
			for _, p := range m.loadedProjects {
				m.markAllExpanded(p.ID, p.GetRootTasks())
			}
		}
	}
}

// markAllExpanded marks all parent tasks as expanded recursively.
func (m *OutlinerModel) markAllExpanded(projectID string, tasks []*models.Task) {
	for _, task := range tasks {
		if len(task.Children) > 0 {
			m.expandedNodes[TaskIdentity{ProjectID: projectID, TaskID: task.ID}] = true
			m.markAllExpanded(projectID, task.Children)
		}
	}
}

// parentIDOf returns the parent task ID for t, or "" if t is a root.
func parentIDOf(t *models.Task) string {
	if t == nil || t.ParentID == nil {
		return ""
	}
	return *t.ParentID
}

// visibleSiblings returns the visible sibling rows in m.rows around cursorIdx —
// tasks sharing the same parent ID and ProjectID as the cursor row, gathered
// across a window bounded by section headers, project headers, and separators.
// Non-sibling task rows inside the window (e.g. expanded descendants of a
// sibling) are skipped rather than treated as hard stops, so J/K reorders and
// move-to-top/bottom can target siblings on either side of an expanded parent.
// Returns nil when cursorIdx is out of range or points at a header/separator/nil
// task row. The window boundaries confine reorders to a single section bucket
// (and to a single project in cross-project mode) without dereferencing
// potentially stale cloned-parent Children slices.
func (m *OutlinerModel) visibleSiblings(cursorIdx int) []*models.Task {
	if cursorIdx < 0 || cursorIdx >= len(m.rows) {
		return nil
	}
	cursorRow := m.rows[cursorIdx]
	if !cursorRow.isTaskRow() || cursorRow.task == nil {
		return nil
	}
	cursorParentID := parentIDOf(cursorRow.task)
	cursorProjectID := cursorRow.ProjectID

	// isBoundary reports whether a row terminates the reorder window. Section
	// headers, project sub-headers, and separators delimit buckets; a task row
	// in a different project also bounds the window in cross-project mode.
	isBoundary := func(row outlinerRow) bool {
		if !row.isTaskRow() {
			return true
		}
		if row.task != nil && row.ProjectID != cursorProjectID {
			return true
		}
		return false
	}

	// isSibling reports whether a task row shares the cursor's parent (and
	// project). Non-matching task rows inside the window are skipped, not stops.
	isSibling := func(row outlinerRow) bool {
		if !row.isTaskRow() || row.task == nil {
			return false
		}
		if parentIDOf(row.task) != cursorParentID {
			return false
		}
		if row.ProjectID != cursorProjectID {
			return false
		}
		return true
	}

	var leftSide []*models.Task
	for i := cursorIdx - 1; i >= 0; i-- {
		if isBoundary(m.rows[i]) {
			break
		}
		if isSibling(m.rows[i]) {
			leftSide = append(leftSide, m.rows[i].task)
		}
	}
	// Reverse leftSide so siblings appear in row order.
	for i, j := 0, len(leftSide)-1; i < j; i, j = i+1, j-1 {
		leftSide[i], leftSide[j] = leftSide[j], leftSide[i]
	}

	siblings := append(leftSide, cursorRow.task)
	for i := cursorIdx + 1; i < len(m.rows); i++ {
		if isBoundary(m.rows[i]) {
			break
		}
		if isSibling(m.rows[i]) {
			siblings = append(siblings, m.rows[i].task)
		}
	}
	return siblings
}

// moveCursorToTask positions the cursor on the row matching the given task ID.
// Falls back to clamping if the task is not found in the current rows.
func (m *OutlinerModel) moveCursorToTask(taskID string) {
	for i, row := range m.rows {
		if row.isTaskRow() && row.task != nil && row.task.ID == taskID {
			m.cursor = i
			return
		}
	}
	m.clampCursor()
}

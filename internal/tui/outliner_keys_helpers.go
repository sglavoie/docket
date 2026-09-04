package tui

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.expandedNodes) == 0 {
		m.expandedNodes = make(map[taskref.TaskIdentity]bool)
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
	m.markAllFolds(projectID, tasks, true)
}

// markAllFolds writes one fold state onto every parent task in the subtree and
// reports whether any of them moved. Childless tasks stay absent: they have
// nothing to fold, and an entry for one would only make isExpanded answer a
// question nobody asks.
func (m *OutlinerModel) markAllFolds(projectID string, tasks []*models.Task, expanded bool) bool {
	changed := false
	for _, task := range tasks {
		if len(task.Children) == 0 {
			continue
		}
		identity := taskref.TaskIdentity{ProjectID: projectID, TaskID: task.ID}
		if previous, known := m.expandedNodes[identity]; !known || previous != expanded {
			changed = true
		}
		m.expandedNodes[identity] = expanded
		if m.markAllFolds(projectID, task.Children, expanded) {
			changed = true
		}
	}
	return changed
}

// anyTaskFoldCollapsed reports whether any task subtree is currently folded. An
// absent key reads as expanded, so only a false entry counts. It is the one
// definition of that question, shared by the direction rule t decides with and
// by the mutation below that has to know whether unfolding changes anything.
func (m *OutlinerModel) anyTaskFoldCollapsed() bool {
	if m == nil {
		return false
	}
	return slices.Contains(slices.Collect(maps.Values(m.expandedNodes)), false)
}

// setAllTaskFolds drives every task subtree to one fold state and reports
// whether that changed anything, so the caller can refuse instead of rebuilding
// rows nothing moved. It owns expandedNodes alone: sections answer to s through
// collapsedSections, and checklists default collapsed under their own map.
//
// The walk covers every loaded project rather than the active projection,
// matching markAllExpanded. Folds are project-qualified and outlive a mode
// change, so folding only what a filter happens to show would leave the modes
// beside it disagreeing about the same task.
func (m *OutlinerModel) setAllTaskFolds(expanded bool) bool {
	if m == nil {
		return false
	}
	if expanded {
		// An empty map is the canonical all-expanded state, so clearing beats
		// writing true over every node. Only a false entry hides anything, so
		// only a false entry makes this a change.
		if !m.anyTaskFoldCollapsed() {
			return false
		}
		m.expandedNodes = make(map[taskref.TaskIdentity]bool)
		return true
	}

	if m.expandedNodes == nil {
		m.expandedNodes = make(map[taskref.TaskIdentity]bool)
	}
	if m.project != nil {
		return m.markAllFolds(m.project.ID, m.project.GetRootTasks(), false)
	}
	changed := false
	for _, project := range m.loadedProjects {
		if m.markAllFolds(project.ID, project.GetRootTasks(), false) {
			changed = true
		}
	}
	return changed
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
			m.Cursor = i
			return
		}
	}
	m.clampCursor()
}

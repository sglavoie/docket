package outliner

import (
	"maps"
	"slices"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/taskref"
)

// ensureExpandedNodes initializes the expanded nodes map if it's empty,
// marking all parent tasks as expanded.
func (m *OutlinerModel) ensureExpandedNodes() {
	if len(m.Folds.ExpandedNodes) == 0 {
		m.Folds.resetNodes()
		if m.Project != nil {
			m.markAllExpanded(m.Project.ID, m.Project.GetRootTasks())
		} else {
			for _, p := range m.Population.Projects {
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
		if previous, known := m.Folds.ExpandedNodes[identity]; !known || previous != expanded {
			changed = true
		}
		m.Folds.setNode(identity, expanded)
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
func (m *OutlinerModel) AnyTaskFoldCollapsed() bool {
	if m == nil {
		return false
	}
	return slices.Contains(slices.Collect(maps.Values(m.Folds.ExpandedNodes)), false)
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
		if !m.AnyTaskFoldCollapsed() {
			return false
		}
		m.Folds.resetNodes()
		return true
	}

	if m.Project != nil {
		return m.markAllFolds(m.Project.ID, m.Project.GetRootTasks(), false)
	}
	changed := false
	for _, project := range m.Population.Projects {
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
// across a window bounded by the cursor row's own group. Non-sibling task rows
// inside the window (e.g. expanded descendants of a sibling) are skipped rather
// than treated as hard stops, so J/K reorders and move-to-top/bottom can target
// siblings on either side of an expanded parent. Returns nil when cursorIdx is
// out of range or points at a header/separator/nil task row. The group boundary
// confines reorders to a single section bucket (and to a single project in
// cross-project mode) without dereferencing potentially stale cloned-parent
// Children slices, and without depending on a header row being drawn between
// two groups. Under the default priority order the window is also one priority
// band, since that is the only run manual order still arranges.
func (m *OutlinerModel) visibleSiblings(cursorIdx int) []*models.Task {
	if cursorIdx < 0 || cursorIdx >= len(m.Rows) {
		return nil
	}
	cursorRow := m.Rows[cursorIdx]
	if !cursorRow.IsTaskRow() || cursorRow.Task == nil {
		return nil
	}
	cursorParentID := parentIDOf(cursorRow.Task)
	cursorProjectID := cursorRow.ProjectID
	priorityBanded := m.sortsByPriorityBand()

	// isBoundary reports whether a row terminates the reorder window. Anything
	// that is not a task row ends it — headers, separators, checklist items — and
	// so does a task row belonging to another group, which is what keeps a
	// reorder inside one section bucket and one project.
	isBoundary := func(row OutlinerRow) bool {
		if !row.IsTaskRow() {
			return true
		}
		return row.Group != cursorRow.Group
	}

	// isSibling reports whether a task row shares the cursor's parent (and
	// project). Non-matching task rows inside the window are skipped, not stops.
	isSibling := func(row OutlinerRow) bool {
		if !row.IsTaskRow() || row.Task == nil {
			return false
		}
		if parentIDOf(row.Task) != cursorParentID {
			return false
		}
		if row.ProjectID != cursorProjectID {
			return false
		}
		if priorityBanded && comparePriorityBand(row.Task, cursorRow.Task) != 0 {
			return false
		}
		return true
	}

	var leftSide []*models.Task
	for i := cursorIdx - 1; i >= 0; i-- {
		if isBoundary(m.Rows[i]) {
			break
		}
		if isSibling(m.Rows[i]) {
			leftSide = append(leftSide, m.Rows[i].Task)
		}
	}
	// Reverse leftSide so siblings appear in row order.
	for i, j := 0, len(leftSide)-1; i < j; i, j = i+1, j-1 {
		leftSide[i], leftSide[j] = leftSide[j], leftSide[i]
	}

	siblings := append(leftSide, cursorRow.Task)
	for i := cursorIdx + 1; i < len(m.Rows); i++ {
		if isBoundary(m.Rows[i]) {
			break
		}
		if isSibling(m.Rows[i]) {
			siblings = append(siblings, m.Rows[i].Task)
		}
	}
	return siblings
}

// moveCursorToTask positions the cursor on the row matching the given task ID.
// Falls back to clamping if the task is not found in the current rows.
func (m *OutlinerModel) MoveCursorToTask(taskID string) {
	for i, row := range m.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			m.Cursor = i
			return
		}
	}
	m.ClampCursor()
}

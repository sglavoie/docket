package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) handleTaskMutationKey(key string, onHeader bool) (tea.Model, tea.Cmd, bool) { //nolint:gocyclo
	switch key {
	case "J":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		task := m.rows[m.cursor].task
		if m.project == nil {
			// Cross-project: the completion-boundary and edge guards use VISIBLE
			// siblings (preserving the silent no-op at the edge and the
			// MoveBlockedMsg toast), but the swap itself is delegated to the app-layer
			// MoveTaskDown seam, which reorders MODEL-order siblings and commits one
			// ledger transaction; the shell reloads the cross outliner.
			row, _, ok := m.crossGetMutationTarget()
			if !ok {
				return m, nil, true
			}
			task = row.task
			siblings := m.visibleSiblings(m.cursor)
			idx := m.siblingIndex(siblings, task.ID)
			if idx >= 0 && idx < len(siblings)-1 {
				siblingBelow := siblings[idx+1]
				if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
					return m, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot move uncompleted task below completed task"}
					}, true
				}
				return m, m.taskItem.MoveDown(task.ID), true
			}
			return m, nil, true
		}
		siblings := m.visibleSiblings(m.cursor)
		idx := m.siblingIndex(siblings, task.ID)
		if idx >= 0 && idx < len(siblings)-1 {
			siblingBelow := siblings[idx+1]
			if task.IsRoot() && !task.IsComplete() && siblingBelow.IsComplete() {
				return m, func() tea.Msg {
					return MoveBlockedMsg{Reason: "cannot move uncompleted task below completed task"}
				}, true
			}
			taskID := task.ID
			m.project.SwapTasks(task, siblingBelow)
			m.setUndoContext("Move Down", task.Title)
			m.undo.TaskIDs = []string{task.ID}
			m.rebuildRows()
			m.moveCursorToTask(taskID)
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "K":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		task := m.rows[m.cursor].task
		if m.project == nil {
			// Cross-project: VISIBLE-sibling guards, MODEL-order swap via the
			// app-layer MoveTaskUp seam (see the "J" branch).
			row, _, ok := m.crossGetMutationTarget()
			if !ok {
				return m, nil, true
			}
			task = row.task
			siblings := m.visibleSiblings(m.cursor)
			idx := m.siblingIndex(siblings, task.ID)
			if idx > 0 {
				siblingAbove := siblings[idx-1]
				if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
					return m, func() tea.Msg {
						return MoveBlockedMsg{Reason: "cannot move completed task above uncompleted task"}
					}, true
				}
				return m, m.taskItem.MoveUp(task.ID), true
			}
			return m, nil, true
		}
		siblings := m.visibleSiblings(m.cursor)
		idx := m.siblingIndex(siblings, task.ID)
		if idx > 0 {
			siblingAbove := siblings[idx-1]
			if task.IsRoot() && task.IsComplete() && !siblingAbove.IsComplete() {
				return m, func() tea.Msg {
					return MoveBlockedMsg{Reason: "cannot move completed task above uncompleted task"}
				}, true
			}
			taskID := task.ID
			m.project.SwapTasks(task, siblingAbove)
			m.setUndoContext("Move Up", task.Title)
			m.undo.TaskIDs = []string{task.ID}
			m.rebuildRows()
			m.moveCursorToTask(taskID)
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "right":
		if len(m.rows) <= 1 || m.cursor <= 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		if m.rows[m.cursor-1].task == nil {
			return m, nil, true
		}
		if m.project == nil {
			// Cross-project: require the visually-adjacent row above to be a task in
			// the same project (matching the legacy same-project indent gate), then
			// delegate the nest to the app-layer IndentTask seam, which resolves the
			// model-above parent within the task's own project and commits one ledger
			// transaction; the shell reloads the cross outliner.
			curRow := m.rows[m.cursor]
			aboveRow := m.rows[m.cursor-1]
			if curRow.ProjectID != aboveRow.ProjectID {
				return m, nil, true
			}
			return m, m.taskItem.Indent(curRow.task.ID), true
		}
		task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
		candidate := m.project.GetTaskByID(m.rows[m.cursor-1].task.ID)
		if task == nil || candidate == nil {
			return m, nil, true
		}
		if IndentTask(m.project, task, candidate) {
			m.setUndoContext("Indent", task.Title)
			m.undo.TaskIDs = []string{task.ID}
			m.validateZoomStack()
			m.rebuildRows()
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true

	case "left":
		if len(m.rows) == 0 || m.cursor >= len(m.rows) || onHeader {
			return m, nil, true
		}
		if m.project == nil {
			// Cross-project: delegate the outdent to the app-layer OutdentTask seam,
			// which validates the task has a parent and commits one ledger
			// transaction; the shell reloads the cross outliner.
			curRow := m.rows[m.cursor]
			return m, m.taskItem.Outdent(curRow.task.ID), true
		}
		task := m.project.GetTaskByID(m.rows[m.cursor].task.ID)
		if task == nil {
			return m, nil, true
		}
		if OutdentTask(m.project, task) {
			m.setUndoContext("Outdent", task.Title)
			m.undo.TaskIDs = []string{task.ID}
			m.validateZoomStack()
			m.rebuildRows()
			return m, m.saveWithWorkflow(), true
		}
		return m, nil, true
	}

	return nil, nil, false
}

func (m *OutlinerModel) siblingIndex(siblings []*models.Task, taskID string) int {
	for i, sibling := range siblings {
		if sibling.ID == taskID {
			return i
		}
	}
	return -1
}

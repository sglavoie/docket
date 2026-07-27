package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *OutlinerModel) reorderTask(delta int) tea.Cmd {
	if len(m.rows) == 0 || m.cursor < 0 || m.cursor >= len(m.rows) {
		return nil
	}
	row := m.rows[m.cursor]
	if !row.isTaskRow() || row.task == nil {
		return nil
	}

	task := row.task
	siblings := m.visibleSiblings(m.cursor)
	index := siblingIndex(siblings, task.ID)
	target := index + delta
	if index < 0 || target < 0 || target >= len(siblings) {
		return nil
	}
	other := siblings[target]
	if task.IsRoot() {
		if delta > 0 && !task.IsComplete() && other.IsComplete() {
			return func() tea.Msg {
				return MoveBlockedMsg{Reason: "cannot move uncompleted task below completed task"}
			}
		}
		if delta < 0 && task.IsComplete() && !other.IsComplete() {
			return func() tea.Msg {
				return MoveBlockedMsg{Reason: "cannot move completed task above uncompleted task"}
			}
		}
	}

	if m.project == nil {
		_, project, ok := m.crossGetMutationTarget()
		if !ok {
			return nil
		}
		identity := taskIdentityFor(task, project)
		if delta > 0 {
			return m.taskItem.MoveDown(identity)
		}
		return m.taskItem.MoveUp(identity)
	}

	task = m.project.GetTaskByID(task.ID)
	other = m.project.GetTaskByID(other.ID)
	if task == nil || other == nil {
		return nil
	}
	m.project.SwapTasks(task, other)
	action := "Move Up"
	if delta > 0 {
		action = "Move Down"
	}
	m.setUndoContext(action, task.Title)
	m.undo.TaskIDs = []string{task.ID}
	m.rebuildRows()
	m.moveCursorToTask(task.ID)
	return m.saveWithWorkflow()
}

func siblingIndex(siblings []*models.Task, taskID string) int {
	for index, sibling := range siblings {
		if sibling != nil && sibling.ID == taskID {
			return index
		}
	}
	return -1
}

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
	project := m.resolveProjectForRow(row)
	if project == nil {
		return nil
	}
	taskIdentity := taskIdentityFor(task, project)
	targetIdentity := taskIdentityFor(other, project)
	if m.taskItem.task != nil {
		return m.taskItem.ReorderSiblings(taskIdentity, targetIdentity)
	}

	// Isolated Outliner models have no transaction coordinator. Preserve their
	// in-memory structural behavior for focused rendering/navigation tests while
	// production always takes the checked application transaction above.
	task = project.GetTaskByID(task.ID)
	other = project.GetTaskByID(other.ID)
	if task == nil || other == nil {
		return nil
	}
	project.SwapTasks(task, other)
	rebuildOutlinerPreservingCursor(m)
	return nil
}

func siblingIndex(siblings []*models.Task, taskID string) int {
	for index, sibling := range siblings {
		if sibling != nil && sibling.ID == taskID {
			return index
		}
	}
	return -1
}

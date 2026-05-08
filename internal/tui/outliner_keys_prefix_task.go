package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleTaskPrefix handles the t prefix key for task-related operations.
func (m *OutlinerModel) handleTaskPrefix(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	// Cross-project: resolve project from cursor, delegate to shared handlers.
	if m.project == nil {
		// Block task creation (same as a/i/I)
		if key == "n" || key == "N" {
			m.setStatus("use go to open project outliner for task creation")
			return m, nil
		}
		row, proj, valid := m.crossGetMutationTarget()
		if !valid {
			return m, nil
		}
		task := row.task
		// to, tc, ta, tl, te, tm → message-only handlers
		if handled, cmd := handleTaskPrefixMsgKey(key, task, proj); handled {
			return m, cmd
		}
		// ts, tt → action handlers (no focusedTask in cross-project)
		if handled, cmd := handleTaskActionPrefixKey(key, task, proj, nil, func() []*models.Task {
			return m.getMultiSelectedTasks()
		}); handled {
			return m, cmd
		}
		// tr → rename via quickEdit
		if key == "r" {
			m.crossQuickEditProject = proj
			return m, m.quickEdit.Enter(task, true)
		}
		// tp → pin/unpin
		if key == "p" {
			return m.crossTogglePinned(row, proj)
		}
		// th → dim/undim
		if key == "h" {
			return m.crossToggleDimmed(row, proj)
		}
		// tD → delete
		if key == "D" {
			if task.IsRecurring() {
				m.confirm.Show("delete-recurring",
					fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
						task.ShortID, task.Title))
			} else {
				m.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
			}
			m.deleteTarget = task
			return m, nil
		}
		return m, nil
	}
	var focusedTask *models.Task
	if len(m.zoomStack) > 0 {
		topID := m.zoomStack[len(m.zoomStack)-1].TaskID
		focusedTask = m.project.GetTaskByID(topID)
	}
	if cmd, handled := handleSharedTaskPrefix(key, m, focusedTask, func(task *models.Task) tea.Cmd {
		return m.quickEdit.Enter(task, true)
	}); handled {
		return m, cmd
	}

	return m, nil
}

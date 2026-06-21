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
		// Task creation: route into the cursor task's owning project.
		if key == "n" || key == "N" {
			task, project := m.getTaskRef()
			if task == nil || project == nil {
				m.setNoTaskStatus()
				return m, nil
			}
			skipPicker := key == "n"
			return m, func() tea.Msg {
				return CreateTaskMsg{ParentTask: task, Project: project, SkipTemplatePicker: skipPicker}
			}
		}
		if refs := m.getSelectedTaskProjectRefs(); len(refs) > 0 {
			switch key {
			case "s":
				m.setStatus("bulk section move not supported in cross-project mode")
				return m, nil
			case "t":
				return m, func() tea.Msg { return ShowBulkTagSelectorMsg{TaskRefs: refs} }
			}
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
		if handled, cmd := handleTaskActionPrefixKey(key, task, proj, nil, nil); handled {
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

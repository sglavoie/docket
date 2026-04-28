package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleConfirmation handles confirmation prompts using the shared ConfirmDialog.
func (m *ProjectTreeModel) handleConfirmation(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	action := m.confirm.Action()

	// Handle recurring task delete confirmation (d/s/n instead of y/n)
	if action == "delete-recurring" && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			m.confirm.Cancel()
			return m, m.deleteTask()
		case "s", "S":
			// Stop recurrence but keep the task (don't delete)
			task := m.deleteTarget
			m.deleteTarget = nil
			m.confirm.Cancel()
			task.ClearRecurrence()
			m.setUndoContext("Stop Recurrence", task.Title)
			m.undo.TaskIDs = []string{task.ID}
			m.setStatus(fmt.Sprintf("Stopped recurrence on #%d", task.ShortID))
			return m, m.saveAndRefresh()
		case "n", "N", "esc":
			m.confirm.Cancel()
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Standard y/n handling via ConfirmDialog
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case "archive":
				return m, m.archiveProject()
			case "unarchive":
				return m, m.unarchiveProject()
			case "delete":
				return m, m.deleteTask()
			case "bulk-delete":
				return m, m.bulkDeleteTasks()
			case "bulk-status":
				return m, m.executePendingBulkStatus()
			case "archive-tasks":
				return m, m.archiveCompletedTasks()
			}
		}
		m.deleteTarget = nil
		m.clearPendingBulkStatus()
		return m, nil
	}
	return m, nil
}

// handleSearchMode handles input while in search mode.
func (m *ProjectTreeModel) handleSearchMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	model, cmd, handled := m.HandleSearchMode(msg, m, func() {
		m.updateVisibility()
		visible := m.visibleNodes()
		m.cursor = ClampCursor(m.cursor, len(visible))
		DetectSearchAutocomplete(m.SearchAC(), m.SearchInputValue(), m.getAllTags(), m.project.GetSections())
	})
	if handled {
		return model, cmd
	}
	return m, cmd
}

// handleQuickEditMode handles input while in quick edit mode.
func (m *ProjectTreeModel) handleQuickEditMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	_, cmd := handleQuickEditKey(msg, m.quickEdit, m.project, m.saveAndRefresh, m.setUndoContext)
	return m, cmd
}

// enterQuickEditMode starts inline editing for the current task.
func (m *ProjectTreeModel) enterQuickEditMode(task *models.Task, renameOnly bool) {
	m.quickEdit.Enter(task, renameOnly)
}

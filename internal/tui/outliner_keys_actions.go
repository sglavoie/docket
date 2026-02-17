package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleConfirmDialog processes key input while a confirmation dialog is active.
func (m *OutlinerModel) handleConfirmDialog(key string) (tea.Model, tea.Cmd) {
	// Capture action before HandleKey clears it on confirm/cancel.
	action := m.confirm.Action()

	// Handle recurring delete separately (d/s/n instead of y/n)
	if action == "delete-recurring" && m.deleteTarget != nil {
		switch key {
		case "d", "D":
			m.confirm.Cancel()
			m.statusMessage = ""
			return m.executeDelete()
		case "s", "S":
			m.deleteTarget.ClearRecurrence()
			m.confirm.Cancel()
			m.statusMessage = ""
			return m.executeDelete()
		case "n", "N", "esc":
			m.confirm.Cancel()
			m.statusMessage = ""
			m.deleteTarget = nil
			return m, nil
		}
		return m, nil
	}

	// Standard y/n handling
	confirmed, handled := m.confirm.HandleKey(key)
	if handled {
		if confirmed {
			switch action {
			case "delete":
				return m.executeDelete()
			case "bulk-delete":
				return m.executeBulkDelete()
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.statusMessage = ""
		m.deleteTarget = nil
		return m, nil
	}
	return m, nil
}

// applyBulkStatusChange applies a status transition to all multi-selected tasks.
func (m *OutlinerModel) applyBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return m, nil
	}

	valid, _ := analyzeBulkStatusTransition(m.project, tasks, newStatus)
	if len(valid) == 0 {
		m.statusMessage = "No selected tasks can transition to " + string(newStatus)
		m.clearMultiSelect()
		return m, nil
	}

	pf := m.workflowPersistFn()
	for _, task := range valid {
		pf(m.project, task.ID, func(t *models.Task) {
			t.ApplyTransition(newStatus)
		})
	}

	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", len(valid), newStatus)
	m.clearMultiSelect()
	m.rebuildRows()
	return m, nil
}

// executeDelete removes the current deleteTarget task using shared helpers.
func (m *OutlinerModel) executeDelete() (tea.Model, tea.Cmd) {
	if m.deleteTarget == nil {
		return m, nil
	}
	task := m.deleteTarget
	m.deleteTarget = nil
	m.statusMessage = ""

	pf := m.workflowPersistFn()
	result := applyDeleteTask(pf, m.project, task.ID, m.config.DataDir)
	if result.Err != nil {
		m.err = result.Err
		return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	m.rebuildRows()
	return m, result.Cmd
}

// executeBulkDelete removes all multi-selected tasks using shared helpers.
func (m *OutlinerModel) executeBulkDelete() (tea.Model, tea.Cmd) {
	if len(m.selectedTasks) == 0 {
		return m, nil
	}

	count := len(m.selectedTasks)
	pf := m.workflowPersistFn()
	var lastCmd tea.Cmd
	for taskID := range m.selectedTasks {
		result := applyDeleteTask(pf, m.project, taskID, m.config.DataDir)
		if result.Err != nil {
			m.err = result.Err
			return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
		lastCmd = result.Cmd
	}

	m.clearMultiSelect()
	m.rebuildRows()
	m.statusMessage = fmt.Sprintf("Deleted %d tasks", count)
	return m, lastCmd
}

// archiveCompletedTasks moves all completed tasks to the archive file.
func (m *OutlinerModel) archiveCompletedTasks() (tea.Model, tea.Cmd) {
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := m.project.CountCompletedTasks()

	svc := NewProjectService()
	cmd := func() tea.Msg {
		if completedCount == 0 {
			return nil
		}
		if undoDepth > 0 {
			_ = svc.RecordUndoSnapshot(m.project, "Archive Tasks",
				fmt.Sprintf("%d tasks", completedCount), undoDepth)
		}
		if err := svc.Backup(m.project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to create backup: %w", err)}
		}
		count, err := svc.ArchiveTasks(m.project)
		if err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to archive tasks: %w", err)}
		}
		_ = svc.Load(m.project)
		m.rebuildRows()
		return TasksArchivedMsg{Count: count}
	}
	m.statusMessage = ""
	return m, cmd
}

// handleCutKey copies the selected task(s) title to clipboard and initiates
// delete confirmation (cut = copy + delete).
func (m *OutlinerModel) handleCutKey() (tea.Model, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		tasks := m.getSelectedTasks()
		copyCmd := func() tea.Msg { return CopyTasksTitleMsg{Tasks: tasks} }
		m.confirm.Show("bulk-delete", fmt.Sprintf("Cut %d selected tasks? (y/n)", len(tasks)))
		m.statusMessage = m.confirm.Prompt()
		return m, copyCmd
	}
	if len(m.rows) > 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
		task := m.rows[m.cursor].task
		copyCmd := func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		m.confirm.Show("delete", fmt.Sprintf("Cut task #%d? (y/n)", task.ShortID))
		m.statusMessage = m.confirm.Prompt()
		m.deleteTarget = task
		return m, copyCmd
	}
	return m, nil
}

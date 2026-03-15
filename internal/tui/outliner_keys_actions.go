package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	if confirmed, handled := m.HandleConfirmKey(key); handled {
		if confirmed {
			switch action {
			case "delete":
				return m.executeDelete()
			case "bulk-delete":
				return m.executeBulkDelete()
			case "bulk-status":
				return m.executePendingBulkStatus()
			case "archive-tasks":
				return m.archiveCompletedTasks()
			}
		}
		m.deleteTarget = nil
		m.bulkOp.Clear()
		return m, nil
	}
	return m, nil
}

// applyBulkStatusChange applies a status transition to all multi-selected tasks.
// If some tasks are blocked by transition rules or incomplete dependencies,
// shows a confirmation prompt before proceeding with the valid ones.
func (m *OutlinerModel) applyBulkStatusChange(newStatus models.TaskStatus) (tea.Model, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return m, nil
	}

	valid, blockedCount := analyzeBulkStatusTransition(m.project, tasks, newStatus)
	if len(valid) == 0 {
		m.statusMessage = "No selected tasks can transition to " + string(newStatus)
		m.clearMultiSelect()
		return m, nil
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// If some tasks are blocked, show confirmation before proceeding
	if blockedCount > 0 {
		m.bulkOp.SetOperation(newStatus, valid, blockedCount, prevStates)
		m.confirm.Show("bulk-status",
			fmt.Sprintf("%d tasks blocked by dependencies. Apply to %d valid tasks?", blockedCount, len(valid)))
		m.clearMultiSelect()
		return m, nil
	}

	// No blocked tasks: mutate all, then save once
	for _, task := range valid {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyTransition(newStatus)
		})
	}
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(valid), newStatus))
	if err := m.saveAndRefreshWithPrevStates(prevStates); err != nil {
		m.err = err
		return m, func() tea.Msg { return ErrorMsg{Err: err} }
	}

	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", len(valid), newStatus)
	m.clearMultiSelect()
	return m, nil
}

// executePendingBulkStatus applies the pending bulk status change after confirmation.
func (m *OutlinerModel) executePendingBulkStatus() (tea.Model, tea.Cmd) {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.bulkOp.Clear()
		return m, nil
	}

	prevStates := m.bulkOp.GetPrevStates()
	status := m.bulkOp.GetStatus()
	tasks := m.bulkOp.GetTasks()

	for _, task := range tasks {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyTransition(status)
		})
	}
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(tasks), status))
	if err := m.saveAndRefreshWithPrevStates(prevStates); err != nil {
		m.err = err
		m.bulkOp.Clear()
		return m, func() tea.Msg { return ErrorMsg{Err: err} }
	}

	m.statusMessage = fmt.Sprintf("Changed %d tasks to %s", len(tasks), status)
	m.bulkOp.Clear()
	return m, nil
}

// showDeleteConfirm shows a confirmation dialog for deleting a single task.
func (m *OutlinerModel) showDeleteConfirm(task *models.Task) {
	m.deleteTarget = task
	if task.IsRecurring() {
		m.confirm.Show("delete-recurring",
			fmt.Sprintf("Delete recurring task #%d '%s'? (d)elete instance / (s)top recurrence / (n)o",
				task.ShortID, task.Title))
	} else {
		m.confirm.Show("delete", fmt.Sprintf("Delete task #%d?", task.ShortID))
	}
}

// showBulkDeleteConfirm shows a confirmation dialog for deleting multiple selected tasks.
func (m *OutlinerModel) showBulkDeleteConfirm() {
	totalChildren := 0
	for _, task := range m.getSelectedTasks() {
		totalChildren += task.CountChildren()
	}
	if totalChildren > 0 {
		m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks and %d children?", len(m.selectedTasks), totalChildren))
	} else {
		m.confirm.Show("bulk-delete", fmt.Sprintf("Delete %d selected tasks?", len(m.selectedTasks)))
	}
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
// Collects all tasks and descendants, cleans up attachments, filters in one
// pass, then persists once instead of per-task.
func (m *OutlinerModel) executeBulkDelete() (tea.Model, tea.Cmd) {
	if len(m.selectedTasks) == 0 {
		return m, nil
	}

	count := len(m.selectedTasks)
	prevStates := captureTaskStates(m.project.Tasks)

	// Collect all selected task IDs and their descendants
	rootIDs := make(map[string]bool, len(m.selectedTasks))
	for taskID := range m.selectedTasks {
		rootIDs[taskID] = true
	}
	idsToDelete := collectDescendantIDs(m.project.Tasks, rootIDs)

	// Clean up attachments for all tasks being deleted
	for _, t := range m.project.Tasks {
		if idsToDelete[t.ID] {
			if err := storage.CleanupTaskAttachments(m.project, t); err != nil {
				logging.Warn("failed to clean up attachments for task %s: %v", t.ID, err)
			}
		}
	}

	// Filter out deleted tasks and clear dangling parent references
	var remaining []*models.Task
	for _, t := range m.project.Tasks {
		if !idsToDelete[t.ID] {
			if t.ParentID != nil && idsToDelete[*t.ParentID] {
				t.ParentID = nil
			}
			remaining = append(remaining, t)
		}
	}
	m.project.Tasks = remaining

	// Persist once
	m.setUndoContext("Bulk Delete", fmt.Sprintf("%d tasks", count))
	if err := m.saveAndRefreshWithPrevStates(prevStates); err != nil {
		m.err = err
		return m, func() tea.Msg { return ErrorMsg{Err: err} }
	}

	m.clearMultiSelect()
	m.statusMessage = fmt.Sprintf("Deleted %d tasks", count)
	return m, nil
}

// archiveCompletedTasks moves all completed tasks to the archive file.
func (m *OutlinerModel) archiveCompletedTasks() (tea.Model, tea.Cmd) {
	undoDepth := 0
	if m.config.UndoHistoryDepth != nil {
		undoDepth = *m.config.UndoHistoryDepth
	}
	completedCount := m.project.CountCompletedTasks()

	svc := m.projectService
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
		m.confirm.Show("bulk-delete", fmt.Sprintf("Cut %d selected tasks?", len(tasks)))
		return m, copyCmd
	}
	if len(m.rows) > 0 && m.cursor >= 0 && m.cursor < len(m.rows) && !m.rows[m.cursor].IsSectionHeader {
		task := m.rows[m.cursor].task
		copyCmd := func() tea.Msg { return CopyTaskTitleMsg{Task: task} }
		m.confirm.Show("delete", fmt.Sprintf("Cut task #%d?", task.ShortID))
		m.deleteTarget = task
		return m, copyCmd
	}
	return m, nil
}

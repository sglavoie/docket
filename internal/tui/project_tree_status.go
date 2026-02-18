package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// handleStatusTransition applies a status change to either the selected tasks
// (in multi-select mode) or the task at the cursor.
// For single-task mode, delegates to the shared applyStatusChange().
// For multi-select mode, uses analyzeBulkStatusTransition with ConfirmDialog
// when blocked tasks are present.
func (m *ProjectTreeModel) handleStatusTransition(status models.TaskStatus, visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		return m.handleBulkStatusTransition(status)
	}
	return m.handleSingleStatusTransition(status, visible)
}

// handleSingleStatusTransition applies a status change to the task at cursor
// using the shared applyStatusChange() function.
func (m *ProjectTreeModel) handleSingleStatusTransition(status models.TaskStatus, visible []*TreeNode) (bool, tea.Cmd) {
	if !CursorInBounds(m.cursor, len(visible)) {
		return true, nil
	}

	task := visible[m.cursor].Task
	m.setUndoContext(StatusVerb(status)+" Task", fmt.Sprintf("#%d %s", task.ShortID, task.Title))

	pf := m.workflowPersistFn()
	result := applyStatusChange(pf, m.project, task.ID, status)
	if result.Err != nil {
		return true, func() tea.Msg { return ErrorMsg{Err: result.Err} }
	}
	m.buildTree()
	return true, result.Cmd
}

// handleBulkStatusTransition applies a status change to all multi-selected tasks.
// If some tasks are blocked by transition rules or incomplete dependencies,
// shows a confirmation prompt before proceeding with the valid ones.
func (m *ProjectTreeModel) handleBulkStatusTransition(status models.TaskStatus) (bool, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return true, nil
	}

	valid, blockedCount := analyzeBulkStatusTransition(m.project, tasks, status)

	if len(valid) == 0 {
		m.clearSelection()
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("no selected tasks can transition to %s", status)}
		}
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// If there are blocked tasks, show confirmation before proceeding
	if blockedCount > 0 {
		m.bulkOp.SetOperation(status, valid, blockedCount, prevStates)
		m.confirm.Show("bulk-status",
			fmt.Sprintf("%d tasks have incomplete deps, complete %d anyway? (y/n)", blockedCount, len(valid)))
		m.clearSelection()
		return true, nil
	}

	// No blocked tasks, proceed directly
	for _, task := range valid {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyTransition(status)
		})
	}
	m.clearSelection()
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(valid), status))
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// executePendingBulkStatus applies the pending bulk status change after confirmation.
func (m *ProjectTreeModel) executePendingBulkStatus() tea.Cmd {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.clearPendingBulkStatus()
		return nil
	}

	// Capture previous states for event-based workflow triggers
	prevStates := m.bulkOp.GetPrevStates()

	// Apply status to all pending tasks
	for _, task := range m.bulkOp.GetTasks() {
		task.ApplyTransition(m.bulkOp.GetStatus())
	}

	count := len(m.bulkOp.GetTasks())
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", count, m.bulkOp.GetStatus()))
	m.clearSelection()
	m.clearPendingBulkStatus()

	return m.saveAndRefreshWithPrevStates(prevStates)
}

// clearPendingBulkStatus clears the pending bulk status state.
func (m *ProjectTreeModel) clearPendingBulkStatus() {
	m.bulkOp.Clear()
}

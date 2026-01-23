package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/sglavoie/tt/internal/models"
)

// statusPredicate determines if a task is eligible for a status change.
type statusPredicate func(*models.Task) bool

// statusAction applies a status change to a task.
type statusAction func(*models.Task)

// statusValidator checks if a task can be changed; returns error message if blocked, empty string if allowed.
type statusValidator func(*models.Task) string

// handleStatusChange applies a status change to selected tasks or the current task.
// It handles multi-select mode by applying the change to all selected tasks that pass
// the predicate, then clears selection. For single-task mode, it applies the change
// to the task at the cursor if it passes the predicate.
// Captures task states before changes to enable event-based workflow triggers.
func (m *ProjectTreeModel) handleStatusChange(
	visible []*TreeNode,
	predicate statusPredicate,
	action statusAction,
	undoAction string,
) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		count := 0
		for _, task := range m.getSelectedTasks() {
			if predicate(task) {
				action(task)
				count++
			}
		}
		m.clearSelection()
		if count > 0 {
			m.setUndoContext(undoAction, fmt.Sprintf("%d tasks", count))
		}
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			action(task)
			m.setUndoContext(undoAction, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefreshWithPrevStates(prevStates)
		}
	}
	return true, nil
}

// handleStatusChangeWithValidation applies a status change with per-task validation.
// Unlike handleStatusChange, this variant can skip individual tasks based on validation
// and reports skipped tasks via an error message. Used for operations like complete
// where dependency checking may block individual tasks.
// Captures task states before changes to enable event-based workflow triggers.
//
// In multi-select mode with blocked tasks, shows a confirmation prompt before proceeding.
func (m *ProjectTreeModel) handleStatusChangeWithValidation(
	visible []*TreeNode,
	predicate statusPredicate,
	validator statusValidator,
	action statusAction,
	actionVerb string,
	undoAction string,
	targetStatus models.TaskStatus,
) (bool, tea.Cmd) {
	// Capture task states before changes for event-based workflow triggers
	prevStates := captureTaskStates(m.project.Tasks)

	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		// Analyze which tasks can be changed vs blocked
		var validTasks []*models.Task
		blockedCount := 0
		for _, task := range m.getSelectedTasks() {
			if !predicate(task) {
				continue
			}
			if errMsg := validator(task); errMsg != "" {
				blockedCount++
				continue
			}
			validTasks = append(validTasks, task)
		}

		// If there are blocked tasks, show confirmation before proceeding
		if blockedCount > 0 && len(validTasks) > 0 {
			m.pendingBulkStatus = targetStatus
			m.pendingBulkTasks = validTasks
			m.pendingBulkBlockedCount = blockedCount
			m.pendingBulkPrevStates = prevStates
			m.confirmAction = "bulk-status"
			return true, nil
		}

		// If all tasks are blocked, show error
		if blockedCount > 0 && len(validTasks) == 0 {
			m.clearSelection()
			return true, func() tea.Msg {
				return ErrorMsg{Err: fmt.Errorf("no tasks %s: %d have incomplete dependencies", actionVerb, blockedCount)}
			}
		}

		// No blocked tasks, proceed directly
		for _, task := range validTasks {
			action(task)
		}
		m.clearSelection()
		if len(validTasks) > 0 {
			m.setUndoContext(undoAction, fmt.Sprintf("%d tasks", len(validTasks)))
		}
		return true, m.saveAndRefreshWithPrevStates(prevStates)
	}
	if CursorInBounds(m.cursor, len(visible)) {
		task := visible[m.cursor].Task
		if predicate(task) {
			if errMsg := validator(task); errMsg != "" {
				return true, func() tea.Msg {
					return ErrorMsg{Err: fmt.Errorf("%s", errMsg)}
				}
			}
			action(task)
			m.setUndoContext(undoAction, fmt.Sprintf("#%d %s", task.ShortID, task.Title))
			return true, m.saveAndRefreshWithPrevStates(prevStates)
		}
	}
	return true, nil
}

// handleStartTask starts a task (in_progress).
func (m *ProjectTreeModel) handleStartTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status == models.StatusTodo || t.Status == models.StatusBlocked
		},
		func(t *models.Task) { t.Start() },
		"Start Task",
	)
}

// handleCompleteTask marks a task as complete (done).
func (m *ProjectTreeModel) handleCompleteTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChangeWithValidation(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) string {
			incompleteDeps := m.project.GetIncompleteDependencies(t)
			if len(incompleteDeps) == 0 {
				return ""
			}
			depNames := make([]string, len(incompleteDeps))
			for i, dep := range incompleteDeps {
				depNames[i] = fmt.Sprintf("#%d", dep.ShortID)
			}
			return fmt.Sprintf("cannot complete: %d incomplete dependencies (%s)",
				len(incompleteDeps), strings.Join(depNames, ", "))
		},
		func(t *models.Task) { t.Complete() },
		"completed",
		"Complete Task",
		models.StatusDone,
	)
}

// handleBlockTask blocks a task.
func (m *ProjectTreeModel) handleBlockTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool {
			return t.Status.IsActive() && t.Status != models.StatusBlocked
		},
		func(t *models.Task) { t.Block() },
		"Block Task",
	)
}

// handleCancelTask cancels a task.
func (m *ProjectTreeModel) handleCancelTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status.IsActive() },
		func(t *models.Task) { t.Cancel() },
		"Cancel Task",
	)
}

// handleReopenTask reopens a task (back to todo).
func (m *ProjectTreeModel) handleReopenTask(visible []*TreeNode) (bool, tea.Cmd) {
	return m.handleStatusChange(visible,
		func(t *models.Task) bool { return t.Status != models.StatusTodo },
		func(t *models.Task) { t.Reopen() },
		"Reopen Task",
	)
}

// executePendingBulkStatus applies the pending bulk status change after confirmation.
func (m *ProjectTreeModel) executePendingBulkStatus() tea.Cmd {
	if len(m.pendingBulkTasks) == 0 {
		m.clearPendingBulkStatus()
		return nil
	}

	// Capture previous states for event-based workflow triggers
	prevStates := m.pendingBulkPrevStates

	// Apply status to all pending tasks
	for _, task := range m.pendingBulkTasks {
		switch m.pendingBulkStatus {
		case models.StatusDone:
			task.Complete()
		case models.StatusInProgress:
			task.Start()
		case models.StatusBlocked:
			task.Block()
		case models.StatusCancelled:
			task.Cancel()
		case models.StatusTodo:
			task.Reopen()
		}
	}

	count := len(m.pendingBulkTasks)
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", count, m.pendingBulkStatus))
	m.clearSelection()
	m.clearPendingBulkStatus()

	return m.saveAndRefreshWithPrevStates(prevStates)
}

// clearPendingBulkStatus clears the pending bulk status state.
func (m *ProjectTreeModel) clearPendingBulkStatus() {
	m.pendingBulkStatus = ""
	m.pendingBulkTasks = nil
	m.pendingBulkBlockedCount = 0
	m.pendingBulkPrevStates = nil
}

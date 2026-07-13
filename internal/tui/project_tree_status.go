package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleStateTransition applies a workflow-state change to either the selected
// tasks (in multi-select mode) or the task at the cursor.
// For multi-select mode, uses analyzeBulkStateTransition with ConfirmDialog
// when blocked tasks are present.
func (m *ProjectTreeModel) handleStateTransition(state models.WorkflowState, visible []*TreeNode) (bool, tea.Cmd) {
	if m.multiSelectMode && len(m.selectedTasks) > 0 {
		return m.handleBulkStateTransition(state)
	}
	return m.handleSingleStateTransition(state, visible)
}

// handleSingleStateTransition applies a workflow-state change to the task at
// cursor through the application-layer single-task coordinator.
func (m *ProjectTreeModel) handleSingleStateTransition(state models.WorkflowState, visible []*TreeNode) (bool, tea.Cmd) {
	if !CursorInBounds(m.cursor, len(visible)) {
		return true, nil
	}

	task := visible[m.cursor].Task

	// Preflight the coordinator guards so an invalid transition surfaces a status
	// message (not an error toast) and does not fire the flash — matching the
	// pre-migration UX. Valid transitions dispatch the coordinator.
	if msg := models.CanTransitionState(task.State(), state); msg != "" {
		m.setStatus(msg)
		return true, nil
	}
	if state == models.StateDone && m.project.HasIncompleteDependencies(task) {
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("cannot complete: has incomplete dependencies")}
		}
	}
	if state.IsTerminal() {
		if incomplete := task.IncompleteChildren(); len(incomplete) > 0 {
			verb := "complete"
			if state == models.StateCanceled {
				verb = "cancel"
			}
			m.setStatus(fmt.Sprintf("cannot %s #%d: has %d incomplete children", verb, task.ShortID, len(incomplete)))
			return true, nil
		}
	}

	cmd := m.taskItem.Transition(task.ID, state)
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		return true, func() tea.Msg { return ErrorMsg{Err: em.Err} }
	}
	if state == models.StateDone {
		m.setCompletionFlash()
		return true, tea.Batch(func() tea.Msg { return msg }, completionFlashScheduler())
	}
	m.setMutationFlash(stateMutationFlashBg(state))
	return true, tea.Batch(func() tea.Msg { return msg }, mutationFlashTimeout())
}

// handleBulkStateTransition applies a workflow-state change to all multi-selected
// tasks. If some tasks are blocked by transition rules or incomplete
// dependencies, shows a confirmation prompt before proceeding with the valid ones.
func (m *ProjectTreeModel) handleBulkStateTransition(state models.WorkflowState) (bool, tea.Cmd) {
	tasks := m.getSelectedTasks()
	if len(tasks) == 0 {
		return true, nil
	}

	valid, blockedCount := analyzeBulkStateTransition(m.project, tasks, state)

	if len(valid) == 0 {
		m.clearSelection()
		return true, func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("no selected tasks can transition to %s", state)}
		}
	}

	prevStates := captureTaskStates(m.project.Tasks)

	// If there are blocked tasks, show confirmation before proceeding
	if blockedCount > 0 {
		m.bulkOp.SetOperation(state, valid, blockedCount, prevStates)
		m.confirm.Show("bulk-status",
			fmt.Sprintf("%d tasks have incomplete deps, complete %d anyway?", blockedCount, len(valid)))
		m.clearSelection()
		return true, nil
	}

	// No blocked tasks, proceed directly
	for _, task := range valid {
		m.project.MutateTask(task.ID, func(t *models.Task) {
			t.ApplyStateTransition(state)
		})
	}
	m.clearSelection()
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", len(valid), state))
	m.undo.TaskIDs = taskIDsFromSlice(valid)
	return true, m.saveAndRefreshWithPrevStates(prevStates)
}

// executePendingBulkStatus applies the pending bulk state change after confirmation.
func (m *ProjectTreeModel) executePendingBulkStatus() tea.Cmd {
	if len(m.bulkOp.GetTasks()) == 0 {
		m.clearPendingBulkStatus()
		return nil
	}

	// Capture previous states for event-based workflow triggers
	prevStates := m.bulkOp.GetPrevStates()

	// Apply the target state to all pending tasks
	for _, task := range m.bulkOp.GetTasks() {
		task.ApplyStateTransition(m.bulkOp.GetState())
	}

	bulkTasks := m.bulkOp.GetTasks()
	count := len(bulkTasks)
	m.setUndoContext("Bulk Status Change", fmt.Sprintf("%d tasks -> %s", count, m.bulkOp.GetState()))
	m.undo.TaskIDs = taskIDsFromSlice(bulkTasks)
	m.clearSelection()
	m.clearPendingBulkStatus()

	return m.saveAndRefreshWithPrevStates(prevStates)
}

// clearPendingBulkStatus clears the pending bulk status state.
func (m *ProjectTreeModel) clearPendingBulkStatus() {
	m.bulkOp.Clear()
}

// taskIDsFromSlice extracts task IDs from a slice of tasks.
func taskIDsFromSlice(tasks []*models.Task) []string {
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return ids
}

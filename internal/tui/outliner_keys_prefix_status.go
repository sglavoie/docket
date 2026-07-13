package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleStatusPrefix handles the , prefix key for workflow-state mutations and
// recurrence operations.
func (m *OutlinerModel) handleStatusPrefix(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	// Cross-project: route to resolved project; recurrence ops need single-project context.
	if m.project == nil {
		if state, ok := StateForPrefixKey(key); ok {
			if model, cmd, handled := m.crossHandleStateMutation(state); handled {
				return model, cmd
			}
		}
		return m, nil
	}
	if key == "k" {
		cmd, _ := handleSkipRecurrence(m)
		return m, cmd
	}
	if key == "D" {
		if m.Dispatcher != nil {
			m.Dispatcher.SetPendingPrefix(",D")
		} else {
			m.pendingPrefix = ",D"
		}
		return m, nil
	}
	if state, ok := StateForPrefixKey(key); ok {
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m.applyBulkStateChange(state)
		}
		return m.applySingleStateTransition(cursorTask, state)
	}

	return m, nil
}

// applySingleStateTransition applies a single-task workflow-state change in
// single-project mode through the application-layer single-task coordinator.
// It preflights the coordinator guards so an invalid transition surfaces a
// status message (not an error toast) and no flash fires, then dispatches the
// coordinator, which commits one ledger-backed transaction; the shell owns
// reload + toast via handleTransactionResult. The completion / mutation flash is
// the preserved view-local behavior. Mirrors ProjectTree's
// handleSingleStateTransition and the cross-project crossHandleStateMutation.
func (m *OutlinerModel) applySingleStateTransition(task *models.Task, state models.WorkflowState) (tea.Model, tea.Cmd) {
	if task == nil {
		return m, nil
	}
	if msg := models.CanTransitionState(task.State(), state); msg != "" {
		m.setStatus(msg)
		return m, nil
	}
	if state == models.StateDone && m.project.HasIncompleteDependencies(task) {
		return m, func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("cannot complete: has incomplete dependencies")} }
	}
	if state.IsTerminal() {
		if incomplete := task.IncompleteChildren(); len(incomplete) > 0 {
			verb := "complete"
			if state == models.StateCanceled {
				verb = "cancel"
			}
			m.setStatus(fmt.Sprintf("cannot %s #%d: has %d incomplete children", verb, task.ShortID, len(incomplete)))
			return m, nil
		}
	}

	cmd := m.taskItem.Transition(task.ID, state)
	msg := cmd()
	if em, ok := msg.(TransactionErrorMsg); ok {
		return m, func() tea.Msg { return ErrorMsg{Err: em.Err} }
	}
	if state == models.StateDone {
		m.setCompletionFlash()
		return m, tea.Batch(func() tea.Msg { return msg }, completionFlashScheduler())
	}
	m.setMutationFlash(stateMutationFlashBg(state))
	return m, tea.Batch(func() tea.Msg { return msg }, mutationFlashTimeout())
}

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// handleStatusPrefix handles the , prefix key for status mutations and recurrence operations.
func (m *OutlinerModel) handleStatusPrefix(key string, cursorTask *models.Task) (tea.Model, tea.Cmd) {
	// Cross-project: route to resolved project; recurrence ops need single-project context.
	if m.project == nil {
		if status, ok := StatusForPrefixKey(key); ok {
			if model, cmd, handled := m.crossHandleStatusMutation(status); handled {
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
	if status, ok := StatusForPrefixKey(key); ok {
		if m.multiSelectMode && len(m.selectedTasks) > 0 {
			return m.applyBulkStatusChange(status)
		}
		if cursorTask == nil {
			return m, nil
		}
		m.setUndoContext(StatusVerb(status)+" Task", fmt.Sprintf("#%d %s", cursorTask.ShortID, cursorTask.Title))
		m.undo.TaskIDs = []string{cursorTask.ID}
		pf := m.workflowPersistFn()
		result := applyStatusChange(pf, m.project, cursorTask.ID, status)
		if result.Err != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: result.Err} }
		}
		m.setStatus(result.StatusMessage)
		if status == models.StatusDone {
			m.setCompletionFlash()
			return m, tea.Batch(result.Cmd, completionFlashScheduler())
		}
		m.setMutationFlash(statusMutationFlashBg(status))
		return m, tea.Batch(result.Cmd, mutationFlashTimeout())
	}

	return m, nil
}

// handleDeferRecurrencePrefix handles the ,D prefix for deferring a recurring task.
func (m *OutlinerModel) handleDeferRecurrencePrefix(key string) (tea.Model, tea.Cmd) {
	if m.project == nil {
		return m, nil
	}
	if cmd, ok := handleDeferRecurrence(key, m); ok {
		return m, cmd
	}
	return m, nil
}

package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// crossHandleStateMutation resolves the owning project from the cursor row,
// applies a workflow-state transition, and forces a row rebuild via rebuildRows.
// It is called from handlePrefixKey when prefix="," and m.project==nil.
//
// In multi-select mode with selections active, routes to crossApplyBulkStateChange.
//
// Returns (model, cmd, true) when the state change was consumed.
// Returns (nil, nil, false) when not in cross-project mode.
func (m *OutlinerModel) crossHandleStateMutation(state models.WorkflowState) (tea.Model, tea.Cmd, bool) {
	if m.project != nil {
		return nil, nil, false
	}
	if m.multiSelectMode && m.SelectedCount() > 0 {
		model, cmd := m.crossApplyBulkStateChange(state)
		return model, cmd, true
	}
	row, _, valid := m.crossGetMutationTarget()
	if !valid {
		return m, nil, true
	}
	// Route the transition through the single-task coordinator: the application
	// layer commits one ledger-backed transaction and the shell owns reload +
	// toast via handleTransactionResult. The completion-flash is the sole
	// preserved view-local behavior — the per-key mutation flash for non-terminal
	// transitions was dropped for the migrated cross handlers.
	cmd := m.taskItem.TransitionWorkflow(row.task.ID, state, m.singleWorkflowOpts(row.task, StateVerb(state)+" Task"), nil)
	if state == models.StateDone {
		m.setCompletionFlash()
		return m, tea.Batch(cmd, completionFlashScheduler()), true
	}
	return m, cmd, true
}

// crossApplyBulkStateChange collects selected tasks from current rows, shows a
// confirmation dialog with the aggregate count, and stores the pending operation
// in m.bulkOp for execution after the user confirms.
func (m *OutlinerModel) crossApplyBulkStateChange(newState models.WorkflowState) (tea.Model, tea.Cmd) {
	var tasks []*models.Task
	for _, row := range m.rows {
		if !row.isTaskRow() || row.task == nil {
			continue
		}
		if m.isRowSelected(row) {
			tasks = append(tasks, row.task)
		}
	}
	if len(tasks) == 0 {
		return m, nil
	}
	m.bulkOp.SetLocatedOperation(newState, tasks, taskLocatorsFromTaskRefs(m.getSelectedTaskProjectRefs()))
	m.confirm.Show("bulk-status", fmt.Sprintf("Change %d tasks to %s?", len(tasks), string(newState)))
	m.clearMultiSelect()
	return m, nil
}

// crossExecutePendingBulkStatus applies the pending cross-project bulk status
// change after the user confirms the dialog. It routes the transition through the
// application-layer BulkCoordinator, which commits every touched project as ONE
// atomic transaction with a workspace-ledger undo entry and runs the workflow-rule
// cascade. SkipInvalid preserves the valid-subset behavior (tasks that cannot
// transition — invalid transition or a complete/cancel guarded by incomplete
// children — are dropped rather than aborting the batch). The shell owns project
// reloads, selection-clear, and user feedback via finalizeBatchResult.
func (m *OutlinerModel) crossExecutePendingBulkStatus() (tea.Model, tea.Cmd) {
	tasks := m.bulkOp.GetTasks()
	locators := m.bulkOp.GetLocators()
	state := m.bulkOp.GetState()
	m.bulkOp.Clear()
	if len(tasks) == 0 {
		return m, nil
	}
	if len(locators) == 0 {
		locators = m.crossTaskLocators(tasks)
	}

	cmd := m.bulk.TransitionWithWorkflow(
		app.BulkTransitionRequest{
			TaskLocators: locators,
			Target:       state,
			Rules:        m.enabledWorkflowRules(),
			UndoAction:   "Bulk Status Change",
			UndoContext:  fmt.Sprintf("%d tasks -> %s", len(locators), state),
			SkipInvalid:  true,
		},
		func(changed int) string {
			if changed == 0 {
				return "No tasks were changed"
			}
			return fmt.Sprintf("Changed %d tasks to %s", changed, state)
		},
	)
	return m, finalizeLocatedBatchResult(cmd, locators, "")
}

func (m *OutlinerModel) crossTaskLocators(tasks []*models.Task) []app.TaskLocator {
	locators := make([]app.TaskLocator, 0, len(tasks))
	for _, target := range tasks {
		if target == nil {
			continue
		}
		var unique app.TaskLocator
		matches := 0
		for _, project := range m.loadedProjects {
			if project == nil {
				continue
			}
			candidate := project.GetTaskByID(target.ID)
			if candidate == target {
				locators = append(locators, app.TaskLocator{ProjectID: project.ID, TaskID: target.ID})
				matches = 0
				break
			}
			if candidate != nil {
				unique = app.TaskLocator{ProjectID: project.ID, TaskID: target.ID}
				matches++
			}
		}
		if matches == 1 {
			locators = append(locators, unique)
		}
	}
	return locators
}

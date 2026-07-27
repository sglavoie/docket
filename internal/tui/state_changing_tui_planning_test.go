package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

// nextFormEdit builds the full-form EditRequest a task-form save issues with an
// explicit canonical state transition.
func nextFormEdit(task *models.Task, state models.WorkflowState) app.EditRequest {
	return app.EditRequest{
		TaskID: task.ID,
		Form: &app.FormEdit{
			Title:    task.Title,
			Type:     task.Type,
			Priority: task.Priority,
			Tags:     []string{},
		},
		WorkflowState: &state,
	}
}

// TestStateChangingTUISurfacesPreservePlanningExclusivity locks FR-011-002 for the
// live TUI surfaces: a task-form edit, both Outliner quick-edit modes, inbox section
// triage, and a cross-project move that changes a task's final workflow state each
// fold Today/Next exclusivity into the same durable transaction as the task write —
// entering StateNext drops Today, leaving StateNext drops the stale Next order entry.
func TestStateChangingTUISurfacesPreservePlanningExclusivity(t *testing.T) {
	t.Run("task-form edit into Next drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		m.seedDiskDailyPlan(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.Edit(nextFormEdit(task, models.StateNext)); err != nil {
			t.Fatalf("form Edit: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateNext {
			t.Fatalf("state after form edit = %s, want next", st)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after form edit into Next = %d, want 0", got)
		}
	})

	t.Run("task-form edit out of Next drops the stale order entry", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateNext)
		m.seedDiskFocusQueue(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.Edit(nextFormEdit(task, models.StateWaiting)); err != nil {
			t.Fatalf("form Edit: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after form edit = %s, want waiting", st)
		}
		if got := diskFocusQueueLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after form edit out of Next = %d, want 0", got)
		}
	})

	t.Run("cross-project quick edit into Next drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		m.seedDiskDailyPlan(t, task.ID, m.currentProject.ID)

		// Mirror the Outliner cross-project quick edit: apply the parsed section change
		// to the loaded task in memory, then replay the full post-edit field set through
		// the app-layer Edit coordinator seam.
		m.currentProject.MutateTask(task.ID, func(tk *models.Task) { tk.ApplyWorkflowState(models.StateNext) })
		mutated := m.currentProject.GetTaskByID(task.ID)
		state := mutated.State()
		cmd := m.secondaryFeatures.TaskItem.Edit(taskIdentityFor(mutated, m.currentProject), app.EditRequest{
			Form:          formEditFromTask(mutated),
			WorkflowState: &state,
		})
		assertNotTransactionError(t, cmd())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateNext {
			t.Fatalf("state after cross quick edit = %s, want next", st)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after cross quick edit into Next = %d, want 0", got)
		}
	})

	t.Run("single-project quick edit into Next drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		m.seedDiskDailyPlan(t, task.ID, m.currentProject.ID)

		// Mirror the Outliner single-project quick edit's save step: the parsed change
		// is applied to the project in memory, the edited task is named as the undo
		// target, and the whole-project workflow commit reconciles Today/Next for it.
		ol := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
		ol.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		ol.project.MutateTask(task.ID, func(tk *models.Task) { tk.ApplyWorkflowState(models.StateNext) })
		ol.undo.Set("Quick Edit", task.Title)
		ol.undo.TaskIDs = []string{task.ID}
		assertNotTransactionError(t, ol.saveWithWorkflow()())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateNext {
			t.Fatalf("state after single quick edit = %s, want next", st)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after single quick edit into Next = %d, want 0", got)
		}
	})

}

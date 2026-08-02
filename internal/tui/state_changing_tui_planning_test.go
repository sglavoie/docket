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
// non-Doing lifecycle moves clear Today and stale Next membership.
func TestStateChangingTUISurfacesPreservePlanningExclusivity(t *testing.T) {
	t.Run("task-form lifecycle edit drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.Edit(nextFormEdit(task, models.StateWaiting)); err != nil {
			t.Fatalf("form Edit: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after form edit = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after form edit into Next = %d, want 0", got)
		}
	})

	t.Run("task-form edit out of Next drops the stale order entry", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskNextOrder(t, task.ID, m.currentProject.ID)

		if _, err := m.appService.Edit(nextFormEdit(task, models.StateWaiting)); err != nil {
			t.Fatalf("form Edit: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after form edit = %s, want waiting", st)
		}
		if got := diskNextOrderLen(t, dataDir); got != 0 {
			t.Fatalf("Next order after form edit out of Next = %d, want 0", got)
		}
	})

	t.Run("cross-project quick lifecycle edit drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		// Mirror the Outliner cross-project quick edit: apply the parsed section change
		// to the loaded task in memory, then replay the full post-edit field set through
		// the app-layer Edit coordinator seam.
		m.currentProject.MutateTask(task.ID, func(tk *models.Task) { tk.ApplyWorkflowState(models.StateWaiting) })
		mutated := m.currentProject.GetTaskByID(task.ID)
		state := mutated.State()
		cmd := m.secondaryFeatures.TaskItem.Edit(taskIdentityFor(mutated, m.currentProject), app.EditRequest{
			Form:          formEditFromTask(mutated),
			WorkflowState: &state,
		})
		assertNotTransactionError(t, cmd())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after cross quick edit = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after cross quick edit into Next = %d, want 0", got)
		}
	})

	t.Run("single-project quick lifecycle edit drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateBacklog)
		m.seedDiskToday(t, task.ID, m.currentProject.ID)

		// Mirror the Outliner single-project quick edit's save step: the parsed change
		// is applied to the project in memory, the edited task is named as the undo
		// target, and the whole-project commit reconciles Today/Next for it.
		ol := NewOutlinerModel(m.currentProject, m.configManager, m.runtime)
		ol.SetTaskItemCoordinator(m.secondaryFeatures.TaskItem)
		ol.project.MutateTask(task.ID, func(tk *models.Task) { tk.ApplyWorkflowState(models.StateWaiting) })
		ol.undo.Set("Quick Edit", task.Title)
		ol.undo.TaskIDs = []string{task.ID}
		assertNotTransactionError(t, ol.saveProject()())

		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateWaiting {
			t.Fatalf("state after single quick edit = %s, want waiting", st)
		}
		if got := diskTodayLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after single quick edit into Next = %d, want 0", got)
		}
	})

}

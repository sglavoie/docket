package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// nextFormEdit builds the full-form EditRequest a task-form save (or Outliner
// cross-project quick edit) issues, overriding the section to Next so the derived
// workflow state changes — the transition the planning reconciliation must fold in.
func nextFormEdit(task *models.Task, section string) app.EditRequest {
	return app.EditRequest{
		TaskID: task.ID,
		Form: &app.FormEdit{
			Title:    task.Title,
			Type:     task.Type,
			Priority: task.Priority,
			Section:  section,
			Tags:     []string{},
		},
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

		if _, err := m.appService.Edit(nextFormEdit(task, models.SectionNext)); err != nil {
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

		if _, err := m.appService.Edit(nextFormEdit(task, models.SectionWaiting)); err != nil {
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
		m.currentProject.MutateTask(task.ID, func(tk *models.Task) { tk.SetSection(models.SectionNext) })
		mutated := m.currentProject.GetTaskByID(task.ID)
		cmd := m.secondaryFeatures.TaskItem.Edit(app.EditRequest{TaskID: mutated.ID, Form: formEditFromTask(mutated)})
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
		ol.project.MutateTask(task.ID, func(tk *models.Task) { tk.SetSection(models.SectionNext) })
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

	t.Run("inbox section triage into Next drops Today", func(t *testing.T) {
		m, _, task, dataDir := newPlanningLedgerApp(t, models.StateReady)
		m.seedDiskDailyPlan(t, task.ID, m.currentProject.ID)

		// Mirror inbox section triage: the section is set on the loaded project in
		// memory, then the whole inbox project commits through commitInboxProject.
		m.currentProject.MutateTask(task.ID, func(tk *models.Task) { tk.SetSection(models.SectionNext) })
		deps := m.configManager.RuntimeDependencies()
		ledger := workspaceLedgerFromConfig(m.runtime)
		if err := commitInboxProject(dataDir, deps, ledger, m.configManager, m.runtime, m.currentProject); err != nil {
			t.Fatalf("commitInboxProject: %v", err)
		}
		if st := diskTaskState(t, m.currentProject, task.ID); st != models.StateNext {
			t.Fatalf("state after inbox triage = %s, want next", st)
		}
		if got := diskDailyPlanLen(t, dataDir); got != 0 {
			t.Fatalf("Daily Plan after inbox triage into Next = %d, want 0", got)
		}
	})

	t.Run("cross-project move into the Next section drops Today", func(t *testing.T) {
		env := newMoveHandlerEnv(t, "move-task-1", "Move me")
		source, dest := env.source, env.dest
		moved := source.GetTaskByID("move-task-1")

		env.cfg.Planning().DailyPlan = []config.DailyPlanItem{{TaskID: moved.ID, ProjectID: source.ID, PlannedDate: "2026-07-19"}}
		if err := saveTestRuntime(env.mgr, env.cfg); err != nil {
			t.Fatalf("save config: %v", err)
		}

		appModel := env.appModel()
		destSection := models.SectionNext
		handled, cmd := appModel.handleMoveTaskToProject(MoveTaskToProjectMsg{
			Task:          moved,
			SourceProject: source,
			DestProjectID: dest.ID,
			DestSection:   &destSection,
		})
		if !handled {
			t.Fatal("move message was not handled")
		}
		assertMoveMsg(t, cmd)

		destTask := env.reloadProject(t, dest).GetTaskByID(moved.ID)
		if destTask == nil {
			t.Fatal("destination should contain moved task")
		}
		if got := destTask.State(); got != models.StateNext {
			t.Fatalf("moved task state = %s, want next", got)
		}
		// The move folds the destination-section reconciliation into its transaction, so
		// the Today entry (reassigned to dest) is dropped now that the task holds Next.
		cfg, err := config.NewManager(env.mgr.GetConfigPath()).LoadRuntimeState()
		if err != nil {
			t.Fatalf("load config from disk: %v", err)
		}
		if got := len(cfg.Planning().DailyPlan); got != 0 {
			t.Fatalf("Daily Plan after move into Next = %d, want 0 (reconciled)", got)
		}
		if got := len(cfg.Planning().FocusQueue); got != 0 {
			t.Fatalf("Next order after move into Next = %d, want 0 (no stale entry added)", got)
		}
	})
}

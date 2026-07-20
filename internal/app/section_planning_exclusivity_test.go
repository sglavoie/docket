package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// exclusivityBuilder is a PlanningArtifactBuilder over an in-memory planning
// domain that reconciles Today/Next exclusivity for exactly the transitioned
// tasks — the app-layer analog of the TUI's planningReconcileArtifactBuilder. A
// task now in StateNext leaves the Daily Plan (Today); a task that left StateNext
// drops its stale Next order entry. It mutates planning in place so the test can
// assert the reconciliation happened, and returns one revision-checked artifact so
// the fold into the transaction is exercised.
func exclusivityBuilder(planning *config.PlanningState) app.PlanningArtifactBuilder {
	return func(targets []app.PlanningReconcileTarget) ([]storage.ArtifactChange, error) {
		changed := false
		for _, tgt := range targets {
			if tgt.State == models.StateNext {
				if planning.RemoveFromDailyPlan(tgt.TaskID) {
					changed = true
				}
				continue
			}
			if planning.RemoveFromFocusQueue(tgt.TaskID) {
				changed = true
			}
		}
		if !changed {
			return nil, nil
		}
		rev := ""
		return []storage.ArtifactChange{{
			Path:             "mem-workspace/.tt/planning.json",
			Data:             []byte("planning"),
			ExpectedRevision: &rev,
		}}, nil
	}
}

// TestSectionMovesPreservePlanningExclusivity locks FR-010-002 at the application
// layer: a section mutation changes the derived workflow state, so it must fold the
// same Today/Next exclusivity reconciliation a state transition does. Every seam —
// the generic SetSection, the workflow-aware SetSectionWithWorkflow, and the bulk
// CommitProjectWithWorkflow — drops Today when a task enters StateNext and drops the
// stale Next order entry when a task leaves StateNext, within the same transaction.
func TestSectionMovesPreservePlanningExclusivity(t *testing.T) {
	t.Run("generic SetSection into Next drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateReady)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-19"}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.SetSection(task.ID, models.SectionNext); err != nil {
			t.Fatalf("SetSection: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateNext {
			t.Fatalf("state = %s, want next", got)
		}
		if got := planning.GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after entering Next = %+v, want empty", got)
		}
	})

	t.Run("generic SetSection out of Next drops the stale order entry", func(t *testing.T) {
		task := newTask("task-1", 1, "On deck")
		task.ApplyWorkflowState(models.StateNext)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetFocusQueue([]config.FocusQueueItem{{TaskID: task.ID, ProjectID: project.ID}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.SetSection(task.ID, models.SectionWaiting); err != nil {
			t.Fatalf("SetSection: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", got)
		}
		if got := planning.GetFocusQueue(); len(got) != 0 {
			t.Fatalf("focus queue after leaving Next = %+v, want empty", got)
		}
	})

	t.Run("workflow-aware SetSectionWithWorkflow into Next drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateReady)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-19"}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.SetSectionWithWorkflow(task.ID, models.SectionNext, app.SingleTaskWorkflowOptions{}); err != nil {
			t.Fatalf("SetSectionWithWorkflow: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateNext {
			t.Fatalf("state = %s, want next", got)
		}
		if got := planning.GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after entering Next = %+v, want empty", got)
		}
	})

	t.Run("bulk CommitProjectWithWorkflow into Next drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateReady)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-19"}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		// The bulk section move mutates the project in memory before committing.
		task.SetSection(models.SectionNext)
		if _, err := svc.CommitProjectWithWorkflow(project, app.ProjectWorkflowOptions{
			UndoAction:      "Move to Section",
			PlanningTaskIDs: []string{task.ID},
		}); err != nil {
			t.Fatalf("CommitProjectWithWorkflow: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateNext {
			t.Fatalf("state = %s, want next", got)
		}
		if got := planning.GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after bulk section move = %+v, want empty", got)
		}
	})
}

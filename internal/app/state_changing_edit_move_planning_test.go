package app_test

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// formEditReplacing builds a full-form EditRequest that replaces every editable
// field from the task while overriding the section, mirroring how the task form
// (and the Outliner cross-project quick edit) always writes the complete post-edit
// field set. Changing the section changes the derived workflow state, which is the
// transition the planning reconciliation must fold in.
func formEditReplacing(task *models.Task, section string) app.EditRequest {
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

// TestStateChangingEditAndMovePreservePlanningExclusivity locks FR-011-002 at the
// application layer: a full-form edit (and its workflow-aware variant) that changes
// a task's section — and therefore its workflow state — must fold the same
// Today/Next exclusivity reconciliation a state transition does, and a cross-project
// move into a section applies that destination state so its caller reconciles the
// task's true final state rather than its pre-move state.
func TestStateChangingEditAndMovePreservePlanningExclusivity(t *testing.T) {
	t.Run("form Edit into Next drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateReady)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-19"}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.Edit(formEditReplacing(task, models.SectionNext)); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateNext {
			t.Fatalf("state = %s, want next", got)
		}
		if got := planning.GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after form edit into Next = %+v, want empty", got)
		}
	})

	t.Run("form Edit out of Next drops the stale order entry", func(t *testing.T) {
		task := newTask("task-1", 1, "On deck")
		task.ApplyWorkflowState(models.StateNext)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetFocusQueue([]config.FocusQueueItem{{TaskID: task.ID, ProjectID: project.ID}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.Edit(formEditReplacing(task, models.SectionWaiting)); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", got)
		}
		if got := planning.GetFocusQueue(); len(got) != 0 {
			t.Fatalf("focus queue after form edit out of Next = %+v, want empty", got)
		}
	})

	t.Run("workflow-aware EditWithWorkflow into Next drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateReady)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetDailyPlan([]config.DailyPlanItem{{TaskID: task.ID, ProjectID: project.ID, PlannedDate: "2026-07-19"}})
		svc := newTestService(repo).WithPlanningArtifacts(exclusivityBuilder(planning))

		if _, err := svc.EditWithWorkflow(formEditReplacing(task, models.SectionNext), app.SingleTaskWorkflowOptions{}); err != nil {
			t.Fatalf("EditWithWorkflow: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateNext {
			t.Fatalf("state = %s, want next", got)
		}
		if got := planning.GetDailyPlan(); len(got) != 0 {
			t.Fatalf("daily plan after workflow form edit into Next = %+v, want empty", got)
		}
	})

	t.Run("MoveToProjectInSection applies the destination section state", func(t *testing.T) {
		task := newTask("task-1", 1, "Mover")
		task.ApplyWorkflowState(models.StateReady)
		src := newProject("src", "Source", task)
		dest := newProject("dest", "Dest")
		repo := newFakeRepo(src, dest)
		svc := newTestService(repo)

		res, err := svc.MoveToProjectInSection(task.ID, dest.ID, models.SectionNext)
		if err != nil {
			t.Fatalf("MoveToProjectInSection: %v", err)
		}
		if res.DestProjectID != dest.ID {
			t.Fatalf("dest project = %s, want %s", res.DestProjectID, dest.ID)
		}
		moved := dest.GetTaskByID(task.ID)
		if moved == nil {
			t.Fatal("moved task not found in destination project")
		}
		// The move applies the destination section before relocating, so the moved
		// root holds StateNext — the final state the TUI reconciles Today/Next against.
		if got := moved.State(); got != models.StateNext {
			t.Fatalf("moved task state = %s, want next", got)
		}
	})
}

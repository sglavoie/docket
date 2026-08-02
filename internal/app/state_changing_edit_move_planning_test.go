package app_test

import (
	"errors"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// formEditReplacing builds a full-form EditRequest that replaces every editable
// field while applying an explicit canonical workflow state.
func formEditReplacing(task *models.Task, state models.WorkflowState) app.EditRequest {
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

// TestStateChangingEditAndMovePreservePlanningExclusivity locks FR-011-002 at the
// application layer: a full-form edit that changes a task's section — and
// therefore its workflow state — must fold the same
// Today/Next exclusivity reconciliation a state transition does, and a cross-project
// move into a section applies that destination state so its caller reconciles the
// task's true final state rather than its pre-move state.
func TestStateChangingEditAndMovePreservePlanningExclusivity(t *testing.T) {
	t.Run("form Edit into Waiting drops Today", func(t *testing.T) {
		task := newTask("task-1", 1, "Today task")
		task.ApplyWorkflowState(models.StateBacklog)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetToday([]config.ProjectTaskLocator{{TaskID: task.ID, ProjectID: project.ID}})
		svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

		if _, err := svc.Edit(formEditReplacing(task, models.StateWaiting)); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", got)
		}
		if got := planning.GetToday(); len(got) != 0 {
			t.Fatalf("daily plan after form edit into Next = %+v, want empty", got)
		}
	})

	t.Run("form Edit out of Next drops the stale order entry", func(t *testing.T) {
		task := newTask("task-1", 1, "On deck")
		task.ApplyWorkflowState(models.StateBacklog)
		project := newProject("proj", "Proj", task)
		repo := newFakeRepo(project)
		planning := config.NewDomains().Planning
		planning.SetNextOrder([]config.ProjectTaskLocator{{TaskID: task.ID, ProjectID: project.ID}})
		svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

		if _, err := svc.Edit(formEditReplacing(task, models.StateWaiting)); err != nil {
			t.Fatalf("Edit: %v", err)
		}
		if got := project.GetTaskByID(task.ID).State(); got != models.StateWaiting {
			t.Fatalf("state = %s, want waiting", got)
		}
		if got := planning.GetNextOrder(); len(got) != 0 {
			t.Fatalf("focus queue after form edit out of Next = %+v, want empty", got)
		}
	})

	t.Run("MoveToProject preserves the canonical state", func(t *testing.T) {
		task := newTask("task-1", 1, "Mover")
		task.ApplyWorkflowState(models.StateBacklog)
		src := newProject("src", "Source", task)
		dest := newProject("dest", "Dest")
		repo := newFakeRepo(src, dest)
		svc := newTestService(repo)

		res, err := svc.MoveToProject(task.ID, dest.ID)
		if err != nil {
			t.Fatalf("MoveToProject: %v", err)
		}
		if res.DestProjectID != dest.ID {
			t.Fatalf("dest project = %s, want %s", res.DestProjectID, dest.ID)
		}
		moved := dest.GetTaskByID(task.ID)
		if moved == nil {
			t.Fatal("moved task not found in destination project")
		}
		if got := moved.State(); got != models.StateBacklog {
			t.Fatalf("moved task state = %s, want ready", got)
		}
	})
}

func TestEditStateChangeUsesCanonicalTransitionGuards(t *testing.T) {
	target := newTask("target", 1, "Target")
	blocker := newTask("blocker", 2, "Blocker")
	blocker.ParentID = &target.ID
	project := newProject("proj", "Proj", target, blocker)
	repo := newFakeRepo(project)
	svc := newTestService(repo)
	done := models.StateDone

	_, err := svc.Edit(app.EditRequest{
		TaskID:        target.ID,
		WorkflowState: &done,
	})
	if !errors.Is(err, app.ErrParentHasIncompleteChildren) {
		t.Fatalf("Edit error = %v, want ErrParentHasIncompleteChildren", err)
	}
	if got := project.GetTaskByID(target.ID).State(); got != models.StateBacklog {
		t.Fatalf("target state = %q, want unchanged ready", got)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("commits = %d, want none for rejected transition", len(repo.commits))
	}
}

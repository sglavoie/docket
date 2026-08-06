package app_test

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestBulkSetDueDateAndPlanTodayCommitsOnePlanningTransaction(t *testing.T) {
	task := newTask("due-today", 1, "Due today")
	task.ApplyWorkflowState(models.StateWaiting)
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.AddToNext(task.ID, project.ID)
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))
	due := time.Date(2026, time.July, 10, 18, 30, 0, 0, time.UTC)

	result, err := svc.BulkSetDueDateAndPlanTodayByLocators([]app.TaskLocator{{
		ProjectID: project.ID,
		TaskID:    task.ID,
	}}, due)
	if err != nil {
		t.Fatalf("BulkSetDueDateAndPlanTodayByLocators: %v", err)
	}
	if len(result.Tasks) != 1 || len(repo.commits) != 1 || len(repo.commits[0]) != 1 {
		t.Fatalf("result/commits = %+v / %+v, want one task in one transaction", result, repo.commits)
	}
	if got := task.DueDate; got == nil || got.Format("2006-01-02") != "2026-07-10" {
		t.Fatalf("due date = %v, want 2026-07-10", got)
	}
	if task.State() != models.StateBacklog {
		t.Fatalf("state = %s, want backlog after Today planning", task.State())
	}
	if len(planning.GetNext()) != 0 || len(planning.GetToday()) != 1 ||
		planning.GetToday()[0].TaskID != task.ID || planning.GetToday()[0].ProjectID != project.ID {
		t.Fatalf("planning = Today %+v Next %+v", planning.GetToday(), planning.GetNext())
	}
	if len(repo.commits[0][0].Artifacts) != 1 {
		t.Fatalf("transaction artifacts = %d, want planning artifact", len(repo.commits[0][0].Artifacts))
	}
}

func TestEditPlanForTodayIncludesFormDueDateAndPlanning(t *testing.T) {
	task := newTask("form-due-today", 1, "Form task")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))
	due := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)
	state := models.StateBacklog

	_, err := svc.Edit(app.EditRequest{
		Locator:       app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		WorkflowState: &state,
		Form: &app.FormEdit{
			Title: task.Title, DueDate: &due,
		},
		PlanForToday: true,
	})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if task.DueDate == nil || len(planning.GetToday()) != 1 || len(repo.commits) != 1 {
		t.Fatalf("due/planning/commits = %v / %+v / %d", task.DueDate, planning.GetToday(), len(repo.commits))
	}
}

func TestTaskOperationDueTodayCarriesPlanningIntent(t *testing.T) {
	task := newTask("operation-due-today", 1, "Operation task")
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	svc := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))
	due := time.Date(2026, time.July, 10, 0, 0, 0, 0, time.UTC)

	result, err := svc.ExecuteTaskOperation(app.TaskOperationRequest{
		Operation:    app.TaskOperationSetDueDate,
		Targets:      []app.TaskLocator{{ProjectID: project.ID, TaskID: task.ID}},
		Date:         &due,
		PlanForToday: true,
	})
	if err != nil {
		t.Fatalf("ExecuteTaskOperation: %v", err)
	}
	if len(result.Tasks) != 1 || task.DueDate == nil || len(planning.GetToday()) != 1 {
		t.Fatalf("result/due/Today = %+v / %v / %+v", result, task.DueDate, planning.GetToday())
	}
}

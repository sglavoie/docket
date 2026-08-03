package app_test

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestReleasePlanningMaintenanceOperations(t *testing.T) {
	nextOne := newTask("next-1", 1, "First next task")
	nextOne.ApplyWorkflowState(models.StateBacklog)
	nextTwo := newTask("next-2", 2, "Second next task")
	nextTwo.ApplyWorkflowState(models.StateBacklog)
	todayOne := newTask("today-1", 3, "First Today task")
	todayTwo := newTask("today-2", 4, "Second Today task")
	project := newProject("project", "Project", nextOne, nextTwo, todayOne, todayTwo)
	repo := newFakeRepo(project)
	planning := config.NewPlanningState()
	planning.Next = []config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: nextTwo.ID},
		{ProjectID: project.ID, TaskID: nextOne.ID},
	}
	planning.Today = []config.ProjectTaskLocator{
		{ProjectID: project.ID, TaskID: todayOne.ID},
		{ProjectID: project.ID, TaskID: todayTwo.ID},
	}
	service := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if err := service.ReorderNext([]app.TaskLocator{
		{ProjectID: project.ID, TaskID: nextOne.ID},
	}); err != nil {
		t.Fatalf("ReorderNext: %v", err)
	}
	if got, want := planningTaskIDs(planning.Next), []string{nextOne.ID, nextTwo.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Next order = %v, want %v", got, want)
	}

	if err := service.ReorderToday([]app.TaskLocator{
		{ProjectID: project.ID, TaskID: todayTwo.ID},
	}); err != nil {
		t.Fatalf("ReorderToday: %v", err)
	}
	if got, want := planningTaskIDs(planning.Today), []string{todayTwo.ID, todayOne.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Today order = %v, want %v", got, want)
	}

	if err := service.ClearToday(); err != nil {
		t.Fatalf("ClearToday: %v", err)
	}
	if len(planning.Today) != 0 {
		t.Fatalf("ClearToday left entries: %+v", planning.Today)
	}

	planning.Next = append(planning.Next, config.ProjectTaskLocator{
		ProjectID: project.ID,
		TaskID:    "missing",
	})
	changed, err := service.ReconcilePlanning()
	if err != nil {
		t.Fatalf("ReconcilePlanning: %v", err)
	}
	if !changed {
		t.Fatal("ReconcilePlanning reported no change for a dangling Next-order entry")
	}
	if got, want := planningTaskIDs(planning.GetNext()), []string{nextOne.ID, nextTwo.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reconciled Next order = %v, want %v", got, want)
	}
	if changed, err = service.ReconcilePlanning(); err != nil || changed {
		t.Fatalf("idempotent ReconcilePlanning = changed %v, err %v", changed, err)
	}
}

func TestReleaseQualifiedProjectQuery(t *testing.T) {
	task := newTask("task", 1, "Qualified task")
	task.Tags = []string{"release", "quality"}
	project := newProject("project", "Project", task)
	repo := newFakeRepo(project)
	service := newTestService(repo)

	summary, err := service.Project(project.ID)
	if err != nil {
		t.Fatalf("Project: %v", err)
	}
	if summary.ID != project.ID || summary.TaskCount != 1 || len(summary.Tags) != 2 {
		t.Fatalf("Project summary = %+v", summary)
	}
	summary.Tags[0] = "mutated-copy"
	if task.Tags[0] != "release" {
		t.Fatal("Project summary tags alias the project snapshot")
	}
}

func planningTaskIDs(items []config.ProjectTaskLocator) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

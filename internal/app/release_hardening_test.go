package app_test

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestReleasePlanningMaintenanceOperations(t *testing.T) {
	nextOne := newTask("next-1", 1, "First next task")
	nextOne.ApplyWorkflowState(models.StateNext)
	nextTwo := newTask("next-2", 2, "Second next task")
	nextTwo.ApplyWorkflowState(models.StateNext)
	todayOne := newTask("today-1", 3, "First Today task")
	todayTwo := newTask("today-2", 4, "Second Today task")
	project := newProject("project", "Project", nextOne, nextTwo, todayOne, todayTwo)
	repo := newFakeRepo(project)
	repo.files[filepath.Clean("mem-workspace/.tt/planning.json")] = []byte{}
	planning := config.NewPlanningState()
	planning.FocusQueue = []config.FocusQueueItem{
		{ProjectID: project.ID, TaskID: nextTwo.ID},
		{ProjectID: project.ID, TaskID: nextOne.ID},
	}
	planning.DailyPlan = []config.DailyPlanItem{
		{ProjectID: project.ID, TaskID: todayOne.ID, PlannedDate: "2026-07-26"},
		{ProjectID: project.ID, TaskID: todayTwo.ID, PlannedDate: "2026-07-26"},
	}
	service := newTestService(repo).WithPlanningArtifacts(memoryPlanningBuilder(planning))

	if err := service.ReorderNext([]app.TaskLocator{
		{ProjectID: project.ID, TaskID: nextOne.ID},
	}); err != nil {
		t.Fatalf("ReorderNext: %v", err)
	}
	if got, want := focusQueueTaskIDs(planning.FocusQueue), []string{nextOne.ID, nextTwo.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Next order = %v, want %v", got, want)
	}

	if err := service.ReorderToday([]app.TaskLocator{
		{ProjectID: project.ID, TaskID: todayTwo.ID},
	}); err != nil {
		t.Fatalf("ReorderToday: %v", err)
	}
	if got, want := dailyPlanTaskIDs(planning.DailyPlan), []string{todayTwo.ID, todayOne.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Today order = %v, want %v", got, want)
	}

	if err := service.UpdateTodayDates([]app.PlanningTodayUpdate{
		{
			Locator:     app.TaskLocator{ProjectID: project.ID, TaskID: todayOne.ID},
			PlannedDate: "2026-07-27",
		},
		{
			Locator:     app.TaskLocator{ProjectID: project.ID},
			PlannedDate: "ignored-invalid-locator",
		},
	}); err != nil {
		t.Fatalf("UpdateTodayDates: %v", err)
	}
	if got := plannedDateForTask(planning.DailyPlan, todayOne.ID); got != "2026-07-27" {
		t.Fatalf("updated Today date = %q, want 2026-07-27", got)
	}

	if err := service.ClearToday(); err != nil {
		t.Fatalf("ClearToday: %v", err)
	}
	if len(planning.DailyPlan) != 0 {
		t.Fatalf("ClearToday left entries: %+v", planning.DailyPlan)
	}

	planning.FocusQueue = append(planning.FocusQueue, config.FocusQueueItem{
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
	if got, want := focusQueueTaskIDs(planning.FocusQueue), []string{nextOne.ID, nextTwo.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("reconciled Next order = %v, want %v", got, want)
	}
	if changed, err = service.ReconcilePlanning(); err != nil || changed {
		t.Fatalf("idempotent ReconcilePlanning = changed %v, err %v", changed, err)
	}
}

func TestReleaseQualifiedQueryAndScorePaths(t *testing.T) {
	task := newTask("task", 1, "Qualified task")
	project := newProject("project", "Project", task)
	project.Tags = []string{"release", "quality"}
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
	if project.Tags[0] != "release" {
		t.Fatal("Project summary tags alias the project snapshot")
	}

	if _, err := service.SetScoreDimensionsByLocator(
		app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		map[string]int{"impact": 5, "effort": 2},
	); err != nil {
		t.Fatalf("SetScoreDimensionsByLocator: %v", err)
	}
	if got := task.ScoreDimensions; got["impact"] != 5 || got["effort"] != 2 {
		t.Fatalf("score dimensions = %v", got)
	}

	if _, err := service.ReplaceScoreDimensionsByLocator(
		app.TaskLocator{ProjectID: project.ID, TaskID: task.ID},
		map[string]int{"confidence": 4},
	); err != nil {
		t.Fatalf("ReplaceScoreDimensionsByLocator: %v", err)
	}
	if got := task.ScoreDimensions; len(got) != 1 || got["confidence"] != 4 {
		t.Fatalf("replaced score dimensions = %v", got)
	}
}

func focusQueueTaskIDs(items []config.FocusQueueItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func dailyPlanTaskIDs(items []config.DailyPlanItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TaskID)
	}
	return ids
}

func plannedDateForTask(items []config.DailyPlanItem, taskID string) string {
	for _, item := range items {
		if item.TaskID == taskID {
			return item.PlannedDate
		}
	}
	return ""
}

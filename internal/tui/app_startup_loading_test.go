package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

func TestNewAppModelLoadsEachReferencedStartupProjectOnce(t *testing.T) {
	runtime := newTestConfig()
	project := &models.Project{
		ID:       "project-1",
		Slug:     "project-1",
		Name:     "Project 1",
		Path:     t.TempDir(),
		Tasks:    []*models.Task{},
		Sections: []string{models.SectionNext},
	}
	setTestProjects(runtime, []*models.Project{project})
	runtime.Machine().CurrentProject = project.ID
	runtime.Planning().SetFocusQueue([]config.FocusQueueItem{{ProjectID: project.ID, TaskID: "task-1"}})
	runtime.Planning().SetDailyPlan([]config.DailyPlanItem{{
		ProjectID:   project.ID,
		TaskID:      "task-1",
		PlannedDate: time.Now().Format("2006-01-02"),
	}})
	runtime.Machine().SetRecentTaskVisits([]config.RecentTaskVisit{{ProjectID: project.ID, TaskID: "task-1"}})

	task := &models.Task{ID: "task-1", ShortID: 1, Title: "Task", WorkflowState: models.StateReady}
	stored := *project
	stored.Tasks = []*models.Task{task}
	mock := NewMockPersister()
	mock.Projects[project.ID] = &stored

	app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(mock))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	if app == nil {
		t.Fatal("NewAppModel returned nil")
	}
	if got := len(mock.LoadCalls); got != 1 {
		t.Fatalf("startup loaded the same referenced project %d times, want 1", got)
	}
	if got := len(app.focusQueue); got != 1 {
		t.Fatalf("focus queue items = %d, want 1", got)
	}
	if got := len(app.dailyPlan); got != 1 {
		t.Fatalf("daily plan items = %d, want 1", got)
	}
	if got := len(app.recentTaskVisits); got != 1 {
		t.Fatalf("recent visits = %d, want 1", got)
	}
}

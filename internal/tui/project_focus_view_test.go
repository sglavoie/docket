package tui

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func newTestProjectFocusView(t *testing.T) (*ProjectFocusViewModel, *models.Project) {
	t.Helper()
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("focus-test")

	// Add an active task with a due date.
	task := mustNewTestTask(project, "Active task with due")
	task.Active = true
	due := time.Now().Add(24 * time.Hour)
	task.DueDate = &due
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	m := NewProjectFocusViewModel(project, cfg)
	m.SetSize(80, 24)
	return m, project
}

func mustNewTestTask(project *models.Project, title string) *models.Task {
	task, err := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, title)
	if err != nil {
		panic(err)
	}
	project.NextShortID++
	return task
}

func TestProjectFocusViewModel_GetConfigNotNil(t *testing.T) {
	m, _ := newTestProjectFocusView(t)

	cfg := m.getConfig()
	if cfg == nil {
		t.Fatal("getConfig() returned nil; expected non-nil *config.Config")
	}
}

func TestProjectFocusViewModel_RenderWithDueDate_NoPanic(t *testing.T) {
	m, _ := newTestProjectFocusView(t)

	// This would panic before the fix because getConfig() returned nil
	// and renderTaskLine called m.getConfig().GetTimeFormat().
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("View() panicked: %v", r)
		}
	}()
	_ = m.View()
}

func TestProjectFocusViewModel_LoadTasks_FiltersCorrectly(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("filter-test")

	active := mustNewTestTask(project, "Active")
	active.Active = true
	project.Tasks = append(project.Tasks, active)

	inactive := mustNewTestTask(project, "Inactive")
	inactive.Active = false
	project.Tasks = append(project.Tasks, inactive)

	completed := mustNewTestTask(project, "Completed")
	completed.Active = true
	completed.Status = models.StatusDone
	project.Tasks = append(project.Tasks, completed)

	snoozed := mustNewTestTask(project, "Snoozed")
	snoozed.Active = true
	snoozeDate := time.Now().Add(48 * time.Hour)
	snoozed.SnoozedUntil = &snoozeDate
	project.Tasks = append(project.Tasks, snoozed)

	project.BuildTaskTree()

	m := NewProjectFocusViewModel(project, cfg)

	if len(m.tasks) != 1 {
		t.Errorf("expected 1 active non-complete non-snoozed task, got %d", len(m.tasks))
	}
	if len(m.tasks) > 0 && m.tasks[0].Title != "Active" {
		t.Errorf("expected task title 'Active', got %q", m.tasks[0].Title)
	}
}

func TestProjectFocusViewModel_NilConfigConstructor(t *testing.T) {
	// Passing nil config should not panic during construction.
	project := testutil.NewTestProject("nil-cfg")
	m := NewProjectFocusViewModel(project, nil)

	if m.getConfig() != nil {
		t.Error("expected getConfig() to return nil when constructed with nil config")
	}
}

func TestProjectFocusViewModel_StagesPrefixSafe(t *testing.T) {
	// When config has Stages defined, getConfig().Stages should not panic.
	cfg := &config.Config{
		Stages: []string{"todo", "doing", "done"},
	}
	project := testutil.NewTestProject("stages-test")
	task := mustNewTestTask(project, "Staged task")
	task.Active = true
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	m := NewProjectFocusViewModel(project, cfg)

	stages := m.getConfig().Stages
	if len(stages) != 3 {
		t.Errorf("expected 3 stages, got %d", len(stages))
	}
}

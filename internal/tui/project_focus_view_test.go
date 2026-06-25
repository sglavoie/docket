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

func TestProjectFocusViewModel_SharedFilters_AffectVisibleTasks(t *testing.T) {
	cfg := testutil.NewTestConfig()
	cfg.Stages = []string{"todo", "doing", "done"}
	project := testutil.NewTestProject("filter-shared")

	plain := mustNewTestTask(project, "Plain task")
	plain.Active = true
	plain.Priority = 4
	project.Tasks = append(project.Tasks, plain)

	dimmed := mustNewTestTask(project, "Dimmed task")
	dimmed.Active = true
	dimmed.Dimmed = true
	project.Tasks = append(project.Tasks, dimmed)

	highPriority := mustNewTestTask(project, "High priority")
	highPriority.Active = true
	highPriority.Priority = 1
	project.Tasks = append(project.Tasks, highPriority)

	bug, err := models.NewTask("bug-id", project.NextShortID, models.TaskTypeBug, "Bug task")
	if err != nil {
		t.Fatalf("NewTask bug: %v", err)
	}
	project.NextShortID++
	bug.Active = true
	project.Tasks = append(project.Tasks, bug)

	tagged := mustNewTestTask(project, "Tagged task")
	tagged.Active = true
	tagged.Tags = []string{"focus"}
	project.Tasks = append(project.Tasks, tagged)

	staged := mustNewTestTask(project, "Staged task")
	staged.Active = true
	staged.Stage = "doing"
	project.Tasks = append(project.Tasks, staged)

	project.BuildTaskTree()
	m := NewProjectFocusViewModel(project, cfg)

	if got := len(m.filteredTasks()); got != 6 {
		t.Fatalf("baseline filteredTasks() = %d, want 6", got)
	}

	// Fh: hide dimmed.
	m.filter.HideDimmed = true
	if got := len(m.filteredTasks()); got != 5 {
		t.Errorf("HideDimmed filteredTasks() = %d, want 5", got)
	}
	for _, task := range m.filteredTasks() {
		if task.Dimmed {
			t.Errorf("HideDimmed should hide dimmed task %q", task.Title)
		}
	}
	clearCommonFilters(&m.filter)

	// Fp: priority filter shows high-priority only (priority <= 2).
	m.filter.Priority = 1
	tasks := m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "High priority" {
		t.Errorf("Priority=1 filteredTasks() = %v, want only 'High priority'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// Ft: type filter (bug).
	m.filter.Type = models.TaskTypeBug
	tasks = m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Bug task" {
		t.Errorf("Type=bug filteredTasks() = %v, want only 'Bug task'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// F#: tag filter.
	m.filter.Tags = map[string]bool{"focus": true}
	tasks = m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Tagged task" {
		t.Errorf("Tags={focus} filteredTasks() = %v, want only 'Tagged task'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// F.: stage filter.
	m.filter.Stage = "doing"
	tasks = m.filteredTasks()
	if len(tasks) != 1 || tasks[0].Title != "Staged task" {
		t.Errorf("Stage=doing filteredTasks() = %v, want only 'Staged task'", taskTitles(tasks))
	}
	clearCommonFilters(&m.filter)

	// Fx (clear) returns the full set.
	if got := len(m.filteredTasks()); got != 6 {
		t.Errorf("after clear filteredTasks() = %d, want 6", got)
	}
}

func TestProjectFocusViewModel_FilterHandlers_DispatchThroughKeymap(t *testing.T) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProject("filter-dispatch")

	plain := mustNewTestTask(project, "Plain")
	plain.Active = true
	project.Tasks = append(project.Tasks, plain)

	dimmed := mustNewTestTask(project, "Dimmed")
	dimmed.Active = true
	dimmed.Dimmed = true
	project.Tasks = append(project.Tasks, dimmed)

	project.BuildTaskTree()
	m := NewProjectFocusViewModel(project, cfg)

	if got := len(m.filteredTasks()); got != 2 {
		t.Fatalf("baseline filteredTasks() = %d, want 2", got)
	}

	// Activate Fh through the same dispatch path used by Update().
	if _, _ = m.handleFilterPrefix("h"); m.filter.HideDimmed != true {
		t.Fatalf("handleFilterPrefix(\"h\") did not toggle HideDimmed; got %v", m.filter.HideDimmed)
	}
	if got := len(m.filteredTasks()); got != 1 {
		t.Errorf("after Fh filteredTasks() = %d, want 1", got)
	}

	// Fx clears.
	if _, _ = m.handleFilterPrefix("x"); m.filter.HideDimmed != false {
		t.Errorf("handleFilterPrefix(\"x\") did not clear HideDimmed; got %v", m.filter.HideDimmed)
	}
	if got := len(m.filteredTasks()); got != 2 {
		t.Errorf("after Fx filteredTasks() = %d, want 2", got)
	}
}

func taskTitles(tasks []*models.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.Title
	}
	return out
}

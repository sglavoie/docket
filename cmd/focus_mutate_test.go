package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// setupFocusMutateCLI creates a single-project workspace with one task and a
// viper-backed config path, returning the project, config path, and a cleanup.
func setupFocusMutateCLI(t *testing.T) (*models.Project, string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "projects", "test-project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project path: %v", err)
	}

	project, err := models.NewProject("proj-focus", "Test Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "test-project"
	project.Path = projectPath
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Build feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.WorkflowState = models.StateReady
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	if err := storage.SaveProject(project); err != nil {
		storage.DefaultRepository = oldRepo
		t.Fatalf("initial SaveProject: %v", err)
	}

	cfg := config.NewConfig()
	cfg.DataDir = tmpDir
	cfg.CurrentProject = project.ID
	cfg.Projects = []*models.Project{project}
	configPath := filepath.Join(tmpDir, "config.json")
	if err := config.NewManager(configPath).Save(cfg); err != nil {
		storage.DefaultRepository = oldRepo
		t.Fatalf("save config: %v", err)
	}

	oldConfig := viper.GetString("config")
	viper.Set("config", configPath)

	return project, configPath, func() {
		viper.Set("config", oldConfig)
		storage.DefaultRepository = oldRepo
	}
}

func reloadFocusConfig(t *testing.T, configPath string) *config.Config {
	t.Helper()
	cfg, err := config.NewManager(configPath).Load()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	return cfg
}

func TestFocusQueueAddRemoveMove(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add: %v", err)
	}
	cfg := reloadFocusConfig(t, configPath)
	if got := cfg.GetFocusQueue(); len(got) != 1 || got[0].TaskID != taskID {
		t.Fatalf("queue after add = %+v, want one item for %s", got, taskID)
	}

	// Adding the same task again must be idempotent.
	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add (idempotent): %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetFocusQueue(); len(got) != 1 {
		t.Fatalf("queue should stay at 1 item after re-add, got %d", len(got))
	}

	// Remove empties the queue.
	if err := runFocusQueueRemove(focusQueueRemoveCmd, []string{"1"}); err != nil {
		t.Fatalf("queue remove: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetFocusQueue(); len(got) != 0 {
		t.Fatalf("queue should be empty after remove, got %d", len(got))
	}
}

func TestFocusQueueMoveReorders(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()

	// Add a second task so the queue has two entries to reorder.
	second, err := models.NewTask("task-2", 2, models.TaskTypeTask, "Second")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = append(project.Tasks, second)
	project.NextShortID = 3
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("save project: %v", err)
	}

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"1"}); err != nil {
		t.Fatalf("queue add 1: %v", err)
	}
	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"2"}); err != nil {
		t.Fatalf("queue add 2: %v", err)
	}

	// Move task 2 to the front.
	if err := runFocusQueueMove(focusQueueMoveCmd, []string{"2", "1"}); err != nil {
		t.Fatalf("queue move: %v", err)
	}
	got := reloadFocusConfig(t, configPath).GetFocusQueue()
	if len(got) != 2 || got[0].TaskID != project.Tasks[1].ID {
		t.Fatalf("queue after move = %+v, want task-2 first", got)
	}
}

func TestFocusTodayAddRemoveClear(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today add: %v", err)
	}
	plan := reloadFocusConfig(t, configPath).GetDailyPlan()
	if len(plan) != 1 || plan[0].TaskID != taskID || plan[0].PlannedDate == "" {
		t.Fatalf("plan after add = %+v, want one dated item for %s", plan, taskID)
	}

	if err := runFocusTodayRemove(focusTodayRemoveCmd, []string{"1"}); err != nil {
		t.Fatalf("today remove: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetDailyPlan(); len(got) != 0 {
		t.Fatalf("plan should be empty after remove, got %d", len(got))
	}

	// Re-add then clear.
	if err := runFocusTodayAdd(focusTodayAddCmd, []string{"1"}); err != nil {
		t.Fatalf("today re-add: %v", err)
	}
	if err := runFocusTodayClear(focusTodayClearCmd, nil); err != nil {
		t.Fatalf("today clear: %v", err)
	}
	if got := reloadFocusConfig(t, configPath).GetDailyPlan(); len(got) != 0 {
		t.Fatalf("plan should be empty after clear, got %d", len(got))
	}
}

func TestFocusQueueAddUnknownTaskErrors(t *testing.T) {
	_, _, cleanup := setupFocusMutateCLI(t)
	defer cleanup()

	if err := runFocusQueueAdd(focusQueueAddCmd, []string{"999"}); err == nil {
		t.Fatal("expected error adding a nonexistent task to the queue")
	}
}

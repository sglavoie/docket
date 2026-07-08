package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestRunTaskParkPersistsMetadataAndUndoSnapshot(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StatusTodo)
	defer cleanup()

	if err := runTaskPark(taskParkCmd, []string{"1", "blocked", "on", "API"}); err != nil {
		t.Fatalf("runTaskPark failed: %v", err)
	}

	reloaded := reloadParkTestProject(t, project)
	task := reloaded.Tasks[0]
	if !task.IsParked() {
		t.Fatal("task should be parked after command")
	}
	if task.ParkedNote != "blocked on API" {
		t.Errorf("ParkedNote = %q, want %q", task.ParkedNote, "blocked on API")
	}
	if task.ParkedAt == nil {
		t.Fatal("ParkedAt should be persisted")
	}

	content := readParkTestTasks(t, project)
	if !strings.Contains(content, "parked: blocked on API") {
		t.Fatalf("tasks.md should contain parked note, got:\n%s", content)
	}
	if !strings.Contains(content, "parked_at:") {
		t.Fatalf("tasks.md should contain parked_at, got:\n%s", content)
	}

	entries, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d undo entries, want 1", len(entries))
	}
	if entries[0].Action != "Park Task" {
		t.Errorf("undo action = %q, want %q", entries[0].Action, "Park Task")
	}
}

func TestRunTaskUnparkClearsMetadataAndRecordsUndoSnapshot(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StatusTodo)
	defer cleanup()

	if err := runTaskPark(taskParkCmd, []string{"1", "state"}); err != nil {
		t.Fatalf("runTaskPark failed: %v", err)
	}
	if err := runTaskUnpark(taskUnparkCmd, []string{"1"}); err != nil {
		t.Fatalf("runTaskUnpark failed: %v", err)
	}

	reloaded := reloadParkTestProject(t, project)
	task := reloaded.Tasks[0]
	if task.IsParked() {
		t.Fatal("task should not be parked after unpark")
	}

	content := readParkTestTasks(t, project)
	if strings.Contains(content, "parked:") || strings.Contains(content, "parked_at:") {
		t.Fatalf("tasks.md should not contain parked metadata after unpark, got:\n%s", content)
	}

	entries, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d undo entries, want 2", len(entries))
	}
	if entries[0].Action != "Unpark Task" {
		t.Errorf("latest undo action = %q, want %q", entries[0].Action, "Unpark Task")
	}
}

func TestRunTaskUnparkNonParkedIsNoop(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StatusTodo)
	defer cleanup()

	if err := runTaskUnpark(taskUnparkCmd, []string{"1"}); err != nil {
		t.Fatalf("runTaskUnpark failed: %v", err)
	}

	entries, err := storage.LoadUndoHistory(project)
	if err != nil {
		t.Fatalf("LoadUndoHistory failed: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("got %d undo entries, want 0", len(entries))
	}
}

func TestRunTaskParkRejectsCompletedTask(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StatusDone)
	defer cleanup()

	err := runTaskPark(taskParkCmd, []string{"1", "state"})
	if err == nil {
		t.Fatal("expected error parking completed task")
	}
	if !strings.Contains(err.Error(), "cannot park a done task") {
		t.Fatalf("unexpected error: %v", err)
	}

	reloaded := reloadParkTestProject(t, project)
	if reloaded.Tasks[0].IsParked() {
		t.Fatal("completed task should not be parked")
	}
}

func setupTaskParkCLI(t *testing.T, status models.TaskStatus) (*models.Project, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	projectPath := filepath.Join(tmpDir, "projects", "test-project")
	if err := os.MkdirAll(projectPath, 0o755); err != nil {
		t.Fatalf("mkdir project path: %v", err)
	}

	project, err := models.NewProject("proj-park", "Test Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = "test-project"
	project.Path = projectPath
	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Build feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Status = status
	if status.IsComplete() {
		task.Complete()
	}
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

	return project, func() {
		viper.Set("config", oldConfig)
		storage.DefaultRepository = oldRepo
	}
}

func reloadParkTestProject(t *testing.T, project *models.Project) *models.Project {
	t.Helper()
	reloaded := &models.Project{
		ID:   project.ID,
		Name: project.Name,
		Slug: project.Slug,
		Path: project.Path,
	}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return reloaded
}

func readParkTestTasks(t *testing.T, project *models.Project) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(project.Path, "tasks.md"))
	if err != nil {
		t.Fatalf("read tasks.md: %v", err)
	}
	return string(content)
}

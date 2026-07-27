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

// TestRunTaskParkPersistsMetadataAndLedgerUndo proves the migrated park command
// persists the parked metadata to tasks.md and records the mutation in the
// workspace undo ledger (not the legacy per-project history). A ledger undo
// reverses the park, restoring the un-parked task.
func TestRunTaskParkPersistsMetadataAndLedgerUndo(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StateReady)
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

	ledger := parkTestLedger(project.Path)
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo failed: %v", err)
	}
	afterUndo := reloadParkTestProject(t, project)
	if afterUndo.Tasks[0].IsParked() {
		t.Fatal("ledger undo should restore the un-parked task")
	}
}

// TestRunTaskUnparkClearsMetadataAndLedgerUndo proves unpark clears the parked
// metadata and records the clear in the workspace ledger; a ledger undo restores
// the parked state.
func TestRunTaskUnparkClearsMetadataAndLedgerUndo(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StateReady)
	defer cleanup()

	if err := runTaskPark(taskParkCmd, []string{"1", "state"}); err != nil {
		t.Fatalf("runTaskPark failed: %v", err)
	}
	if err := runTaskUnpark(taskUnparkCmd, []string{"1"}); err != nil {
		t.Fatalf("runTaskUnpark failed: %v", err)
	}

	reloaded := reloadParkTestProject(t, project)
	if reloaded.Tasks[0].IsParked() {
		t.Fatal("task should not be parked after unpark")
	}

	content := readParkTestTasks(t, project)
	if strings.Contains(content, "parked:") || strings.Contains(content, "parked_at:") {
		t.Fatalf("tasks.md should not contain parked metadata after unpark, got:\n%s", content)
	}

	// Ledger undo of the unpark restores the parked state (the most recent
	// transaction is the unpark).
	ledger := parkTestLedger(project.Path)
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo failed: %v", err)
	}
	afterUndo := reloadParkTestProject(t, project)
	if !afterUndo.Tasks[0].IsParked() {
		t.Fatal("ledger undo of the unpark should restore the parked task")
	}
}

// TestRunTaskUnparkNonParkedIsNoop proves unparking a task that is not parked is a
// friendly no-op that records nothing in the workspace ledger.
func TestRunTaskUnparkNonParkedIsNoop(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StateReady)
	defer cleanup()

	if err := runTaskUnpark(taskUnparkCmd, []string{"1"}); err != nil {
		t.Fatalf("runTaskUnpark failed: %v", err)
	}

	// A no-op precedes any commit, so the ledger has no transaction to undo.
	ledger := parkTestLedger(project.Path)
	if _, err := ledger.Undo(storage.GlobalScope()); err == nil {
		t.Fatal("expected ledger undo to fail with no recorded transaction")
	}
}

func TestRunTaskParkRejectsCompletedTask(t *testing.T) {
	project, cleanup := setupTaskParkCLI(t, models.StateDone)
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

// parkTestLedger builds a workspace ledger over the data directory that owns the
// given project path (tmpDir/projects/<slug> → tmpDir), matching the ledger the
// CLI repository writes through so a test can undo the recorded transaction.
func parkTestLedger(projectPath string) *storage.WorkspaceLedger {
	dataDir := filepath.Dir(filepath.Dir(projectPath))
	return storage.NewWorkspaceLedger(config.UndoLedgerDir(dataDir), config.DefaultUndoHistoryDepth, []string{dataDir})
}

func setupTaskParkCLI(t *testing.T, status models.WorkflowState) (*models.Project, func()) {
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
	if status.IsTerminal() {
		task.Complete()
	} else {
		task.WorkflowState = status
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2

	oldRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	if err := storage.SaveProject(project); err != nil {
		storage.DefaultRepository = oldRepo
		t.Fatalf("initial SaveProject: %v", err)
	}

	domains := config.NewDomains()
	domains.Catalog.UpsertProject(project)
	domains.Machine.CurrentProject = project.ID
	cfg := config.NewRuntimeState(tmpDir, domains)
	configPath := filepath.Join(tmpDir, "config.json")
	if err := saveTestRuntime(config.NewManager(configPath), cfg); err != nil {
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

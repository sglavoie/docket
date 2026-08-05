package tui

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func TestNewAppModelLoadsEachReferencedStartupProjectOnce(t *testing.T) {
	runtime := newTestConfig()
	project := &models.Project{
		ID:    "project-1",
		Slug:  "project-1",
		Name:  "Project 1",
		Path:  t.TempDir(),
		Tasks: []*models.Task{},
	}
	setTestProjects(runtime, []*models.Project{project})
	runtime.Planning().SetNext([]config.ProjectTaskLocator{{ProjectID: project.ID, TaskID: "task-next"}})
	runtime.Planning().SetToday([]config.ProjectTaskLocator{{
		ProjectID: project.ID,
		TaskID:    "task-today",
	}})
	// Next and Today are mutually exclusive, so use separate valid members while
	// proving all planning references share one startup project load.
	nextTask := &models.Task{ID: "task-next", ShortID: 1, Title: "Next task", WorkflowState: models.StateBacklog}
	todayTask := &models.Task{ID: "task-today", ShortID: 2, Title: "Today task", WorkflowState: models.StateBacklog}
	stored := *project
	stored.Tasks = []*models.Task{nextTask, todayTask}
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
	if got := len(app.nextOrder); got != 1 {
		t.Fatalf("focus queue items = %d, want 1", got)
	}
	if got := len(app.today); got != 1 {
		t.Fatalf("daily plan items = %d, want 1", got)
	}
}

func TestEnsureProjectColorSlotsCommitsStartupBackfillOnce(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	store := config.NewDomainFileStore(dataDir)
	catalog := config.NewWorkspaceCatalog()
	catalog.Projects = []config.CatalogProject{{ID: "legacy", Slug: "legacy", Name: "Legacy"}}
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	app := &AppModel{runtime: runtime, projects: make(map[string]*models.Project)}
	if ensureErr := app.ensureProjectColorSlots(); ensureErr != nil {
		t.Fatalf("ensureProjectColorSlots: %v", ensureErr)
	}
	if project := app.projectByID("legacy"); project == nil || project.ColorSlot != 1 {
		t.Fatalf("startup cache project = %+v, want ColorSlot 1", project)
	}

	catalogPath := config.WorkspaceCatalogPath(dataDir)
	persisted, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	concurrentBytes := append(append([]byte(nil), persisted...), '\n')
	if writeErr := os.WriteFile(catalogPath, concurrentBytes, config.FilePermissions); writeErr != nil {
		t.Fatalf("write concurrent marker: %v", writeErr)
	}
	if ensureErr := app.ensureProjectColorSlots(); ensureErr != nil {
		t.Fatalf("idempotent ensureProjectColorSlots: %v", ensureErr)
	}
	after, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("ReadFile after no-op: %v", err)
	}
	if !bytes.Equal(after, concurrentBytes) {
		t.Fatal("second startup rewrote an already-backfilled catalog")
	}
}

func TestEnsureProjectColorSlotsCommitFailureIsNonfatal(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dataDir := filepath.Join(t.TempDir(), "workspace")
	store := config.NewDomainFileStore(dataDir)
	now := time.Date(2026, time.January, 1, 0, 0, 0, 0, time.UTC)
	catalog := config.NewWorkspaceCatalog()
	catalog.Projects = []config.CatalogProject{
		{
			ID: "legacy", Slug: "legacy", Name: "Legacy",
			CreatedAt: now, UpdatedAt: now,
		},
		{
			ID: "system-inbox", Slug: config.SystemUnfiledSlug, Name: config.SystemUnfiledName,
			CreatedAt: now, UpdatedAt: now, ColorSlot: 1,
		},
	}
	catalog.SystemUnfiledProjectID = "system-inbox"
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("SaveCatalog: %v", err)
	}

	inboxPath := catalog.Projects[1].CatalogPath(dataDir)
	if err := os.MkdirAll(inboxPath, 0o755); err != nil {
		t.Fatalf("create Inbox directory: %v", err)
	}
	if err := os.WriteFile(storage.TasksFilePath(inboxPath), []byte("# Inbox\n"), config.FilePermissions); err != nil {
		t.Fatalf("write Inbox tasks: %v", err)
	}

	runtime, err := config.LoadRuntimeState(dataDir)
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	catalogPath := config.WorkspaceCatalogPath(dataDir)
	original, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if writeErr := os.WriteFile(catalogPath, []byte("{broken"), config.FilePermissions); writeErr != nil {
		t.Fatalf("corrupt catalog: %v", writeErr)
	}

	previousLogOutput := log.Writer()
	var logOutput bytes.Buffer
	log.SetOutput(&logOutput)
	t.Cleanup(func() { log.SetOutput(previousLogOutput) })
	app, err := NewAppModel(runtime, &config.Manager{}, WithPersister(NewMockPersister()))
	if err != nil {
		t.Fatalf("NewAppModel returned startup persistence error: %v", err)
	}
	if !bytes.Contains(logOutput.Bytes(), []byte("failed to persist project color slots")) {
		t.Fatalf("startup did not log the failed color-slot commit: %s", logOutput.String())
	}
	if project := app.projectByID("legacy"); project == nil || project.ColorSlot != 0 {
		t.Fatalf("startup cache project = %+v, want muted ColorSlot 0 after failed commit", project)
	}
	if project := runtime.GetProject("legacy"); project == nil || project.ColorSlot != 0 {
		t.Fatalf("runtime project = %+v, want unchanged ColorSlot 0 after failed commit", project)
	}

	if writeErr := os.WriteFile(catalogPath, original, config.FilePermissions); writeErr != nil {
		t.Fatalf("restore catalog: %v", writeErr)
	}
	if repairErr := app.ensureProjectColorSlots(); repairErr != nil {
		t.Fatalf("retry ensureProjectColorSlots: %v", repairErr)
	}
	project := app.projectByID("legacy")
	if project == nil || project.ColorSlot == 0 {
		t.Fatalf("retry cache project = %+v, want assigned ColorSlot", project)
	}
	persisted, err := store.LoadCatalog()
	if err != nil {
		t.Fatalf("LoadCatalog after retry: %v", err)
	}
	for _, candidate := range persisted.Projects {
		if candidate.ID == "legacy" && candidate.ColorSlot == project.ColorSlot {
			return
		}
	}
	t.Fatalf("persisted legacy project did not retain retry ColorSlot %d: %+v", project.ColorSlot, persisted.Projects)
}

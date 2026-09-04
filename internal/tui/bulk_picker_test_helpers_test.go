package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/coordinator"
)

func newBulkPickerTestTask(t *testing.T, shortID int, title string, tags ...string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(uuid.New().String(), shortID, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Tags = append([]string{}, tags...)
	return task
}

func newBulkPickerTestProject(t *testing.T, name string, tasks ...*models.Task) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), name, filepath.Join(t.TempDir(), models.Slugify(name)))
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := os.MkdirAll(project.Path, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", project.Path, err)
	}
	project.Tasks = tasks
	project.NextShortID = len(tasks) + 1
	for _, task := range tasks {
		task.Project = project
	}
	project.BuildTaskTree()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	return project
}

type appBulkPickerEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	features coordinator.SecondaryCoordinators
	mgr      *config.Manager
	cfg      *config.RuntimeState
	projects []*models.Project
}

func newAppBulkPickerEnv(t *testing.T, taskGroups ...[]*models.Task) *appBulkPickerEnv {
	t.Helper()
	dataDir := t.TempDir()
	t.Setenv("HOME", dataDir)
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("bp%d", i)
		project, err := models.NewProject(uuid.New().String(), slug, "")
		if err != nil {
			t.Fatalf("NewProject: %v", err)
		}
		project.Slug = slug
		project.Path = filepath.Join(dataDir, "projects", slug)
		project.Tasks = tasks
		next := 1
		for _, task := range tasks {
			if task.ShortID >= next {
				next = task.ShortID + 1
			}
		}
		project.NextShortID = next
		project.BuildTaskTree()
		projects[i] = project
	}

	repo := newScenarioRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)

	mgr := config.NewManager(filepath.Join(dataDir, ".tt.json"))
	cfg := newTestConfigAt(dataDir)
	setTestProjects(cfg, projects)
	if err := saveTestRuntime(mgr, cfg); err != nil {
		t.Fatalf("seed config save: %v", err)
	}
	return &appBulkPickerEnv{
		dataDir: dataDir, ledger: ledger, svc: svc,
		features: coordinator.NewSecondaryCoordinators(svc),
		mgr:      mgr, cfg: cfg, projects: projects,
	}
}

func (e *appBulkPickerEnv) appModel() *AppModel {
	projects := make(map[string]*models.Project, len(e.projects))
	for _, project := range e.projects {
		projects[project.ID] = project
	}
	return &AppModel{
		configManager: e.mgr, runtime: testRuntimeAt(e.cfg.DataDir(), e.cfg),
		deps:     newShellDeps(nil),
		projects: projects, persister: coordinator.NewProjectService(), appService: e.svc,
		secondaryFeatures: e.features, viewNav: newViewNavigationController(),
	}
}

func (e *appBulkPickerEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	project := e.projects[idx]
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *appBulkPickerEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

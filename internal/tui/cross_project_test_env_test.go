package tui

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/coordinator"
)

func newCrossProjectTestEnv(t *testing.T, taskGroups ...[]*models.Task) *crossProjectTestEnv {
	t.Helper()
	dataDir := t.TempDir()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	projects := make([]*models.Project, len(taskGroups))
	for i, tasks := range taskGroups {
		slug := fmt.Sprintf("out%d", i)
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
	coords := coordinator.NewSecondaryCoordinators(svc)
	return &crossProjectTestEnv{
		dataDir: dataDir, ledger: ledger, svc: svc,
		bulk: coords.Bulk, taskItem: coords.TaskItem, projects: projects,
	}
}

func (e *crossProjectTestEnv) reload(t *testing.T, idx int) *models.Project {
	t.Helper()
	project := e.projects[idx]
	fresh := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("reload project %d: %v", idx, err)
	}
	return fresh
}

func (e *crossProjectTestEnv) undo(t *testing.T) {
	t.Helper()
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
}

// crossProjectTestEnv seeds projects on disk and wires the coordinators used
// by cross-project Outliner mutation tests.
type crossProjectTestEnv struct {
	dataDir  string
	ledger   *storage.WorkspaceLedger
	svc      *app.Service
	bulk     coordinator.BulkCoordinator
	taskItem coordinator.TaskItemCoordinator
	projects []*models.Project
}

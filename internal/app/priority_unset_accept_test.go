package app_test

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// These acceptance tests commit through the real transactional storage path and
// then reload from disk, because that is the round trip the in-memory fake repo
// cannot see: a p0 clear used to commit successfully and rehydrate as P3.

func newUnsetPriorityService(t *testing.T, dataDir string, projects ...*models.Project) *app.Service {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newStorageRepo(commitSvc, projects...)
	repo.seed(t)
	return app.NewServiceWithArtifacts(repo, nil, nil, nil)
}

// prioritizedProject builds a project rooted under the canonical
// <data>/projects/<slug> layout whose tasks all start at an explicit priority.
func prioritizedProject(t *testing.T, dataDir, slug string, priority int, titles ...string) *models.Project {
	t.Helper()
	project, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Slug = slug
	project.Path = filepath.Join(dataDir, "projects", slug)
	for i, title := range titles {
		task, taskErr := models.NewGenericTask(uuid.New().String(), i+1, title)
		if taskErr != nil {
			t.Fatalf("NewTask: %v", taskErr)
		}
		task.Priority = priority
		project.Tasks = append(project.Tasks, task)
	}
	project.NextShortID = len(titles) + 1
	project.BuildTaskTree()
	return project
}

func assertReloadedPriority(t *testing.T, project *models.Project, taskID string) {
	t.Helper()
	reloaded := &models.Project{ID: project.ID, Name: project.Name, Slug: project.Slug, Path: project.Path}
	if err := storage.LoadProject(reloaded); err != nil {
		t.Fatalf("reload project %q: %v", project.Slug, err)
	}
	task := reloaded.GetTaskByID(taskID)
	if task == nil {
		t.Fatalf("reloaded task %q is missing from %q", taskID, project.Slug)
	}
	if task.Priority != models.PriorityUnset {
		t.Fatalf("reloaded task %q priority = %d, want %d", taskID, task.Priority, models.PriorityUnset)
	}
}

func TestSetPriorityZeroSurvivesReloadFromDisk(t *testing.T) {
	dataDir := t.TempDir()
	project := prioritizedProject(t, dataDir, "alpha", models.PriorityMin, "Build login form")
	svc := newUnsetPriorityService(t, dataDir, project)
	task := project.Tasks[0]

	if _, err := svc.SetPriority(task.ID, models.PriorityUnset); err != nil {
		t.Fatalf("SetPriority(unset): %v", err)
	}
	assertReloadedPriority(t, project, task.ID)
}

func TestBulkSetPriorityZeroSurvivesReloadFromDisk(t *testing.T) {
	dataDir := t.TempDir()
	alpha := prioritizedProject(t, dataDir, "alpha", models.PriorityMin, "Build login form", "Add password validation")
	beta := prioritizedProject(t, dataDir, "beta", models.PriorityMax, "Triage support inbox")
	svc := newUnsetPriorityService(t, dataDir, alpha, beta)

	ids := []string{alpha.Tasks[0].ID, alpha.Tasks[1].ID, beta.Tasks[0].ID}
	res, err := svc.BulkSetPriority(ids, models.PriorityUnset)
	if err != nil {
		t.Fatalf("BulkSetPriority(unset): %v", err)
	}
	if len(res.TouchedProjectIDs) != 2 {
		t.Fatalf("touched projects = %v, want both", res.TouchedProjectIDs)
	}
	assertReloadedPriority(t, alpha, alpha.Tasks[0].ID)
	assertReloadedPriority(t, alpha, alpha.Tasks[1].ID)
	assertReloadedPriority(t, beta, beta.Tasks[0].ID)
}

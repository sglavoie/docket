package app_test

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newProjectCreation(projects ...*models.Project) app.ProjectCreation {
	revision := "catalog-revision"
	return app.ProjectCreation{
		Projects: projects,
		Catalog:  storage.ArtifactChange{Path: fakeCatalogPath, Data: []byte("{}"), ExpectedRevision: &revision},
	}
}

// The created project, the existing one, and the catalog ride one Commit, so
// the repository records them as one transaction.
func TestCaptureManyCreatingProjectsCommitsCatalogWithTasks(t *testing.T) {
	alpha := newProject("p1", "Alpha")
	repo := newFakeRepo(alpha)
	svc := newTestService(repo)
	garden := newProject("p2", "garden")

	results, err := svc.CaptureManyCreatingProjects(newProjectCreation(garden), []app.CaptureRequest{
		{ProjectID: garden.ID, Title: "Plant the beans"},
		{ProjectID: alpha.ID, Title: "Buy compost"},
	})
	if err != nil {
		t.Fatalf("CaptureManyCreatingProjects: %v", err)
	}
	if len(results) != 2 || results[0].ProjectID != garden.ID {
		t.Fatalf("results = %+v", results)
	}
	if len(repo.commits) != 1 {
		t.Fatalf("commits = %d, want one transaction", len(repo.commits))
	}
	var catalogs int
	for _, changeset := range repo.commits[0] {
		if changeset.Project.ID == garden.ID && (changeset.ExpectedRevision != "" || changeset.Base != nil) {
			t.Fatalf("the created project's changeset claims a prior document: %+v", changeset)
		}
		for _, artifact := range changeset.Artifacts {
			if artifact.Path == fakeCatalogPath {
				catalogs++
			}
		}
	}
	if catalogs != 1 {
		t.Fatalf("catalog artifacts = %d, want exactly one", catalogs)
	}
	if len(garden.Tasks) != 0 {
		t.Fatal("the capture wrote into the caller's project value instead of its own copy")
	}
}

// A project the snapshot already holds under that slug refuses the whole batch
// before anything commits.
func TestCaptureManyCreatingProjectsRefusesAHeldSlug(t *testing.T) {
	alpha := newProject("p1", "Alpha")
	repo := newFakeRepo(alpha)
	svc := newTestService(repo)
	duplicate := newProject("p2", "alpha")

	_, err := svc.CaptureManyCreatingProjects(newProjectCreation(duplicate), []app.CaptureRequest{
		{ProjectID: duplicate.ID, Title: "Plant the beans"},
	})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("err = %v, want a held-slug refusal", err)
	}
	if len(repo.commits) != 0 {
		t.Fatalf("a refused capture committed %d transactions", len(repo.commits))
	}
}

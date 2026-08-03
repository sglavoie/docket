package app_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestArtifactsContract exercises the note use cases plus the
// snapshot-backed queries against BOTH an in-memory repository and the real
// filesystem commit service, proving every request behaves identically regardless
// of the backing store.
func TestArtifactsContract(t *testing.T) {
	t.Run("in-memory", func(t *testing.T) {
		project := newProject("p1", "alpha")
		repo := newFakeRepo(project)
		port := repo.artifactPort()
		svc := app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, port)
		runArtifactContract(t, svc, project.ID)
	})

	t.Run("filesystem", func(t *testing.T) {
		dataDir := t.TempDir()
		commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
		project := realTestProject(t, dataDir, "alpha", "seed task")
		repo := newStorageRepo(commitSvc, project)
		repo.seed(t)
		seedNoteTestCatalog(t, dataDir, project)
		svc := app.NewServiceWithArtifacts(repo, nil, nil, nil)
		runArtifactContract(t, svc, project.ID)
	})
}

func runArtifactContract(t *testing.T, svc *app.Service, projectID string) {
	t.Helper()

	// Save a note as a transaction artifact, then read it back through the query.
	ts := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	note := &models.Note{Title: "Design", Filename: "design.md", Body: "hello world", CreatedAt: ts, ModifiedAt: ts}
	saveRes, err := svc.SaveNote(projectID, note)
	if err != nil {
		t.Fatalf("SaveNote: %v", err)
	}
	if saveRes.Filename != "design.md" {
		t.Fatalf("SaveNote filename = %q, want design.md", saveRes.Filename)
	}
	notes, err := svc.Notes(projectID)
	if err != nil {
		t.Fatalf("Notes: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("Notes len = %d, want 1", len(notes))
	}
	if notes[0].Title != "Design" || !strings.Contains(notes[0].Body, "hello world") {
		t.Fatalf("Notes[0] = %+v, want title Design body containing 'hello world'", notes[0])
	}

	// Delete the note; the query then reports none.
	if _, deleteErr := svc.DeleteNote(projectID, "design.md"); deleteErr != nil {
		t.Fatalf("DeleteNote: %v", deleteErr)
	}
	notes, err = svc.Notes(projectID)
	if err != nil {
		t.Fatalf("Notes after delete: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Notes after delete len = %d, want 0", len(notes))
	}

	if _, err := svc.ProjectTags(projectID); err != nil {
		t.Fatalf("ProjectTags: %v", err)
	}
}

// TestArtifactsValidation covers the request-validation branches that reject a
// mutation before any commit.
func TestArtifactsValidation(t *testing.T) {
	project := newProject("p1", "alpha")
	repo := newFakeRepo(project)
	port := repo.artifactPort()
	svc := app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, port)

	if _, err := svc.SaveNote(project.ID, &models.Note{Title: "  "}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SaveNote blank title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNote(app.NoteCreateRequest{Location: app.GlobalNoteLocation(), Note: &models.Note{Title: "  "}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("CreateNote blank global title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SaveNote("missing", &models.Note{Title: "x", Filename: "x.md"}); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("SaveNote unknown project err = %v, want ErrProjectNotFound", err)
	}
	if _, err := svc.DeleteNote(project.ID, ""); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("DeleteNote blank filename err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 {
		t.Fatalf("a rejected request committed %d changesets and %d artifact transactions, want 0", len(repo.commits), len(repo.artifactCommits))
	}
}

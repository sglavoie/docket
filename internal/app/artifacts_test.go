package app_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
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
		commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir})
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

	// Create a note as a transaction artifact, then read it back through the query.
	ts := time.Date(2026, 7, 10, 9, 0, 0, 0, time.UTC)
	location := app.ProjectNoteLocation(projectID)
	note := &models.Note{Title: "Design", Filename: "design.md", Body: "hello world", CreatedAt: ts, ModifiedAt: ts}
	saveRes, err := svc.CreateNote(app.NoteCreateRequest{Location: location, Note: note})
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if saveRes.Filename != "design.md" {
		t.Fatalf("CreateNote filename = %q, want design.md", saveRes.Filename)
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
	if _, deleteErr := svc.RemoveNote(app.NoteDeleteRequest{Location: location, Note: notes[0]}); deleteErr != nil {
		t.Fatalf("RemoveNote: %v", deleteErr)
	}
	notes, err = svc.Notes(projectID)
	if err != nil {
		t.Fatalf("Notes after delete: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("Notes after delete len = %d, want 0", len(notes))
	}
}

// TestArtifactsValidation covers the request-validation branches that reject a
// mutation before any commit.
func TestArtifactsValidation(t *testing.T) {
	project := newProject("p1", "alpha")
	repo := newFakeRepo(project)
	port := repo.artifactPort()
	svc := app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, port)

	if _, err := svc.CreateNote(app.NoteCreateRequest{Location: app.ProjectNoteLocation(project.ID), Note: &models.Note{Title: "  "}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("CreateNote blank title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNote(app.NoteCreateRequest{Location: app.GlobalNoteLocation(), Note: &models.Note{Title: "  "}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("CreateNote blank global title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNote(app.NoteCreateRequest{Location: app.ProjectNoteLocation("missing"), Note: &models.Note{Title: "x", Filename: "x.md"}}); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("CreateNote unknown project err = %v, want ErrProjectNotFound", err)
	}
	if _, err := svc.RemoveNote(app.NoteDeleteRequest{Location: app.ProjectNoteLocation(project.ID), Note: &models.Note{Filename: ""}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("RemoveNote blank filename err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 {
		t.Fatalf("a rejected request committed %d changesets and %d artifact transactions, want 0", len(repo.commits), len(repo.artifactCommits))
	}
}

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

// TestArtifactsContract exercises the note and blocker use cases plus the
// snapshot-backed queries against BOTH an in-memory repository and the real
// filesystem commit service, proving every request behaves identically regardless
// of the backing store.
func TestArtifactsContract(t *testing.T) {
	t.Run("in-memory", func(t *testing.T) {
		project := newProject("p1", "alpha")
		repo := newFakeRepo(project)
		port := repo.artifactPort()
		svc := app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, port, port)
		runArtifactContract(t, svc, project.ID)
	})

	t.Run("filesystem", func(t *testing.T) {
		dataDir := t.TempDir()
		commitSvc := storage.NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
		project := realTestProject(t, dataDir, "alpha", "seed task")
		repo := newStorageRepo(commitSvc, project)
		repo.seed(t)
		seedNoteTestCatalog(t, dataDir, project)
		svc := app.NewService(repo, nil, nil)
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

	// Set a blocker list; the missing ID is minted and the count is reported.
	blocker := &models.Blocker{Title: "API access"}
	bRes, err := svc.SetBlockers(projectID, []*models.Blocker{blocker})
	if err != nil {
		t.Fatalf("SetBlockers: %v", err)
	}
	if bRes.Count != 1 || bRes.ActiveCount != 1 {
		t.Fatalf("SetBlockers result = %+v, want Count 1 ActiveCount 1", bRes)
	}
	if blocker.ID == "" {
		t.Fatalf("SetBlockers did not mint a blocker ID")
	}
	blockers, err := svc.Blockers(projectID)
	if err != nil {
		t.Fatalf("Blockers: %v", err)
	}
	if len(blockers) != 1 || blockers[0].Title != "API access" {
		t.Fatalf("Blockers = %+v, want one titled 'API access'", blockers)
	}
	if got, countErr := svc.ActiveBlockerCount(projectID); countErr != nil || got != 1 {
		t.Fatalf("ActiveBlockerCount = %d, %v, want 1, nil", got, countErr)
	}

	// Resolve the blocker and rewrite the list: the active count drops to zero.
	blockers[0].Resolve()
	bRes, err = svc.SetBlockers(projectID, blockers)
	if err != nil {
		t.Fatalf("SetBlockers (resolved): %v", err)
	}
	if bRes.ActiveCount != 0 {
		t.Fatalf("ActiveCount after resolve = %d, want 0", bRes.ActiveCount)
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

	// Snapshot-backed project queries.
	summaries, err := svc.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(summaries) != 1 || summaries[0].Name != "alpha" {
		t.Fatalf("ListProjects = %+v, want one project named alpha", summaries)
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
	svc := app.NewServiceWithArtifacts(repo, fixedClock{t: time.Date(2026, 7, 10, 12, 0, 0, 0, time.UTC)}, &seqIDs{}, port, port)

	if _, err := svc.SaveNote(project.ID, &models.Note{Title: "  "}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SaveNote blank title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.CreateNote(app.NoteCreateRequest{Location: app.GlobalNoteLocation(), Note: &models.Note{Title: "  "}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("CreateNote blank global title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.SaveNote("missing", &models.Note{Title: "x", Filename: "x.md"}); !errors.Is(err, app.ErrProjectNotFound) {
		t.Fatalf("SaveNote unknown project err = %v, want ErrProjectNotFound", err)
	}
	if _, err := svc.SetBlockers(project.ID, []*models.Blocker{{Title: ""}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("SetBlockers blank title err = %v, want ErrInvalidInput", err)
	}
	if _, err := svc.DeleteNote(project.ID, ""); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("DeleteNote blank filename err = %v, want ErrInvalidInput", err)
	}
	if len(repo.commits) != 0 || len(repo.artifactCommits) != 0 {
		t.Fatalf("a rejected request committed %d changesets and %d artifact transactions, want 0", len(repo.commits), len(repo.artifactCommits))
	}
}

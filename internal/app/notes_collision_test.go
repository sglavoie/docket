package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func TestWorkspaceNoteCollisionRetriesAtCommitBoundary(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	_, baseRepo := newNoteTestService(t, dataDir, project)
	repo := &collisionRepo{storageRepo: baseRepo}
	port := &guardedNotePort{WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir)}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port)

	result, err := svc.CreateNote(app.NoteCreateRequest{
		Location: app.ProjectNoteLocation(project.ID),
		Note:     &models.Note{Title: "collision", Filename: "collision.md"},
	})
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if result.Filename != "collision-2.md" {
		t.Fatalf("filename = %q, want collision-2.md", result.Filename)
	}
	original, err := os.ReadFile(filepath.Join(storage.NotesDir(project), "collision.md"))
	if err != nil || string(original) != "concurrent destination" {
		t.Fatalf("concurrent destination = %q, %v", original, err)
	}
	if _, err := os.Stat(filepath.Join(storage.NotesDir(project), result.Filename)); err != nil {
		t.Fatalf("deduplicated destination missing: %v", err)
	}
	if port.calls != 2 {
		t.Fatalf("project precondition builds = %d, want one per collision attempt", port.calls)
	}
}

func TestWorkspaceNoteBatchIsAllOrNoneOnStaleSource(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	projectLocation := app.ProjectNoteLocation(project.ID)
	globalLocation := app.GlobalNoteLocation()

	projectCreated, err := svc.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: &models.Note{Title: "project", Filename: "project.md", Body: "base project"}})
	if err != nil {
		t.Fatal(err)
	}
	globalCreated, err := svc.CreateNote(app.NoteCreateRequest{Location: globalLocation, Note: &models.Note{Title: "global", Filename: "global.md", Body: "base global"}})
	if err != nil {
		t.Fatal(err)
	}
	projectUpdate := cloneNoteForTest(noteByFilename(t, svc, projectLocation, projectCreated.Filename))
	projectUpdate.Body = "batch project"
	globalUpdate := cloneNoteForTest(noteByFilename(t, svc, globalLocation, globalCreated.Filename))
	globalUpdate.Body = "batch global"

	concurrent := cloneNoteForTest(noteByFilename(t, svc, globalLocation, globalCreated.Filename))
	concurrent.Body = "concurrent global"
	concurrentBytes, err := storage.RenderNoteContent(concurrent)
	if err != nil {
		t.Fatal(err)
	}
	if writeErr := os.WriteFile(notePathForTest(dataDir, project, globalLocation, concurrent.Filename), []byte(concurrentBytes), storage.FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	_, err = svc.ApplyNoteBatch(app.NoteBatchRequest{Operations: []app.NoteMutationRequest{
		{Kind: app.NoteUpdate, Location: projectLocation, Note: projectUpdate},
		{Kind: app.NoteUpdate, Location: globalLocation, Note: globalUpdate},
	}})
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("ApplyNoteBatch err = %v, want ErrStaleArtifact", err)
	}
	projectAfterFailure := noteByFilename(t, svc, projectLocation, projectUpdate.Filename)
	if projectAfterFailure.Body != "base project" {
		t.Fatalf("first batch operation partially committed: %q", projectAfterFailure.Body)
	}

	projectAfterFailure.Body = "batch project"
	globalFresh := noteByFilename(t, svc, globalLocation, globalUpdate.Filename)
	globalFresh.Body = "batch global"
	result, err := svc.ApplyNoteBatch(app.NoteBatchRequest{Operations: []app.NoteMutationRequest{
		{Kind: app.NoteUpdate, Location: projectLocation, Note: projectAfterFailure},
		{Kind: app.NoteUpdate, Location: globalLocation, Note: globalFresh},
	}})
	if err != nil {
		t.Fatalf("fresh ApplyNoteBatch: %v", err)
	}
	if len(result.Results) != 2 {
		t.Fatalf("batch result = %+v", result)
	}
	if result.TransactionID == "" || result.Results[0].TransactionID != result.TransactionID || result.Results[1].TransactionID != result.TransactionID {
		t.Fatalf("batch transaction correlation = %+v", result)
	}
}

func cloneNoteForTest(note *models.Note) *models.Note {
	copyNote := *note
	return &copyNote
}

func mutateNoteTestCatalog(t *testing.T, dataDir string, mutate func(*config.WorkspaceCatalog)) {
	t.Helper()
	store := config.NewDomainFileStore(dataDir)
	catalog, revision, err := store.LoadCatalogChecked()
	if err != nil {
		t.Fatalf("load workspace catalog: %v", err)
	}
	mutate(catalog)
	if err := store.SaveCatalogChecked(catalog, revision); err != nil {
		t.Fatalf("save workspace catalog: %v", err)
	}
}

func noteUndoCount(t *testing.T, dataDir string) int {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	count, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}
	return count
}

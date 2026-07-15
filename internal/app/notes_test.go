package app_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newNoteTestService(t *testing.T, dataDir string, projects ...*models.Project) (*app.Service, *storageRepo) {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newStorageRepo(commitSvc, projects...)
	repo.seed(t)
	svc := app.NewServiceWithArtifacts(
		repo,
		fixedClock{t: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)},
		&seqIDs{},
		app.NewStorageWorkspaceNotePort(dataDir),
		nil,
	)
	return svc, repo
}

func noteByFilename(t *testing.T, svc *app.Service, location app.NoteLocation, filename string) *models.Note {
	t.Helper()
	notes, err := svc.NotesAt(location)
	if err != nil {
		t.Fatalf("NotesAt(%+v): %v", location, err)
	}
	for _, note := range notes {
		if note.Filename == filename {
			return note
		}
	}
	t.Fatalf("note %q not found at %+v", filename, location)
	return nil
}

func TestWorkspaceNoteProjectAndGlobalCRUD(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	tasksBefore, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatal(err)
	}

	locations := []struct {
		name     string
		location app.NoteLocation
		filename string
	}{
		{name: "project", location: app.ProjectNoteLocation(project.ID), filename: "project-note.md"},
		{name: "global", location: app.GlobalNoteLocation(), filename: "global-note.md"},
	}
	for _, tc := range locations {
		t.Run(tc.name, func(t *testing.T) {
			note := &models.Note{Title: tc.name + " note", Filename: tc.filename, Body: "v1"}
			created, createErr := svc.CreateNote(app.NoteCreateRequest{Location: tc.location, Note: note})
			if createErr != nil {
				t.Fatalf("CreateNote: %v", createErr)
			}
			if created.Note == note || created.Revision == "" || created.Note.RawRevision != created.Revision {
				t.Fatalf("CreateNote result did not return an isolated revision-bearing note: %+v", created)
			}

			loaded := noteByFilename(t, svc, tc.location, created.Filename)
			loaded.Body = "v2"
			updated, updateErr := svc.UpdateNote(app.NoteUpdateRequest{Location: tc.location, Note: loaded})
			if updateErr != nil {
				t.Fatalf("UpdateNote: %v", updateErr)
			}
			if updated.Revision == loaded.RawRevision || updated.Note.Body != "v2" {
				t.Fatalf("UpdateNote result = %+v", updated)
			}

			renamed, renameErr := svc.RenameNote(app.NoteRenameRequest{Location: tc.location, Note: updated.Note, NewTitle: tc.name + " renamed"})
			if renameErr != nil {
				t.Fatalf("RenameNote: %v", renameErr)
			}
			if renamed.Filename == created.Filename || renamed.PreviousFilename != created.Filename {
				t.Fatalf("RenameNote result = %+v", renamed)
			}
			if _, statErr := os.Stat(notePathForTest(dataDir, project, tc.location, created.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("old filename still exists after rename: %v", statErr)
			}

			if _, deleteErr := svc.RemoveNote(app.NoteDeleteRequest{Location: tc.location, Note: renamed.Note}); deleteErr != nil {
				t.Fatalf("RemoveNote: %v", deleteErr)
			}
			if notes, listErr := svc.NotesAt(tc.location); listErr != nil || len(notes) != 0 {
				t.Fatalf("notes after delete = %v, %v", notes, listErr)
			}
		})
	}

	tasksAfter, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(tasksAfter, tasksBefore) {
		t.Fatal("note-only CRUD rewrote tasks.md")
	}
}

func notePathForTest(dataDir string, project *models.Project, location app.NoteLocation, filename string) string {
	if location.Scope == app.NoteScopeGlobal {
		return filepath.Join(storage.GlobalNotesDir(dataDir), filename)
	}
	return filepath.Join(storage.NotesDir(project), filename)
}

func TestWorkspaceNoteMoveIsAtomicOnConcurrentSourceEdit(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	global := app.GlobalNoteLocation()
	destination := app.ProjectNoteLocation(project.ID)

	created, err := svc.CreateNote(app.NoteCreateRequest{Location: global, Note: &models.Note{Title: "move me", Filename: "move.md", Body: "base"}})
	if err != nil {
		t.Fatal(err)
	}
	stale := created.Note
	concurrent := *stale
	concurrent.Body = "concurrent"
	content, err := storage.RenderNoteContent(&concurrent)
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := notePathForTest(dataDir, project, global, stale.Filename)
	if writeErr := os.WriteFile(sourcePath, []byte(content), storage.FilePermissions); writeErr != nil {
		t.Fatal(writeErr)
	}

	_, err = svc.MoveNote(app.NoteMoveRequest{Source: global, Destination: destination, Note: stale})
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("MoveNote err = %v, want ErrStaleArtifact", err)
	}
	if _, statErr := os.Stat(notePathForTest(dataDir, project, destination, stale.Filename)); !os.IsNotExist(statErr) {
		t.Fatalf("stale move created destination: %v", statErr)
	}
	if got, readErr := os.ReadFile(sourcePath); readErr != nil || !strings.Contains(string(got), "concurrent") {
		t.Fatalf("stale move changed source: %q, %v", got, readErr)
	}

	fresh := noteByFilename(t, svc, global, stale.Filename)
	moved, err := svc.MoveNote(app.NoteMoveRequest{Source: global, Destination: destination, Note: fresh})
	if err != nil {
		t.Fatalf("fresh MoveNote: %v", err)
	}
	if moved.Location != destination || moved.SourceLocation != global || moved.ProjectID != project.ID {
		t.Fatalf("MoveNote result = %+v", moved)
	}
	if _, statErr := os.Stat(sourcePath); !os.IsNotExist(statErr) {
		t.Fatalf("source remains after move: %v", statErr)
	}
}

type collisionRepo struct {
	*storageRepo
	injected bool
}

func (r *collisionRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	if !r.injected {
		r.injected = true
		for _, artifact := range request.Artifacts {
			if artifact.ExpectedRevision != nil && *artifact.ExpectedRevision == "" {
				if err := os.MkdirAll(filepath.Dir(artifact.Path), storage.DirPermissions); err != nil {
					return "", err
				}
				if err := os.WriteFile(artifact.Path, []byte("concurrent destination"), storage.FilePermissions); err != nil {
					return "", err
				}
				break
			}
		}
	}
	return r.storageRepo.CommitArtifactTransaction(request)
}

func TestWorkspaceNoteCollisionRetriesAtCommitBoundary(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	_, baseRepo := newNoteTestService(t, dataDir, project)
	repo := &collisionRepo{storageRepo: baseRepo}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)

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
	projectUpdate := cloneNoteForTest(projectCreated.Note)
	projectUpdate.Body = "batch project"
	globalUpdate := cloneNoteForTest(globalCreated.Note)
	globalUpdate.Body = "batch global"

	concurrent := cloneNoteForTest(globalCreated.Note)
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
	if result.TransactionID == "" || len(result.Results) != 2 || result.Results[0].TransactionID != result.TransactionID || result.Results[1].TransactionID != result.TransactionID {
		t.Fatalf("batch result = %+v", result)
	}
}

func cloneNoteForTest(note *models.Note) *models.Note {
	copyNote := *note
	copyNote.Tags = append([]string(nil), note.Tags...)
	copyNote.LinkedTasks = append([]string(nil), note.LinkedTasks...)
	return &copyNote
}

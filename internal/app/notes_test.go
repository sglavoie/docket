package app_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

func newNoteTestService(t *testing.T, dataDir string, projects ...*models.Project) (*app.Service, *storageRepo) {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newStorageRepo(commitSvc, projects...)
	repo.seed(t)
	seedNoteTestCatalog(t, dataDir, projects...)
	svc := app.NewServiceWithArtifacts(
		repo,
		fixedClock{t: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)},
		&seqIDs{},
		app.NewStorageWorkspaceNotePort(dataDir),
	)
	return svc, repo
}

func seedNoteTestCatalog(t *testing.T, dataDir string, projects ...*models.Project) {
	t.Helper()
	store := config.NewDomainFileStore(dataDir)
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		if err := catalog.UpsertProject(project); err != nil {
			t.Fatalf("seed workspace catalog project: %v", err)
		}
	}
	if err := store.SaveCatalog(catalog); err != nil {
		t.Fatalf("seed workspace catalog: %v", err)
	}
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

func TestWorkspaceNoteEmptyDataDirDerivesCanonicalWorkspaceRoot(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	_, repo := newNoteTestService(t, dataDir, project)
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(""))

	created, err := svc.CreateNote(app.NoteCreateRequest{
		Location: app.ProjectNoteLocation(project.ID),
		Note:     &models.Note{Title: "derived workspace", Filename: "derived-workspace.md"},
	})
	if err != nil {
		t.Fatalf("CreateNote with derived workspace root: %v", err)
	}
	if created.TransactionID == "" {
		t.Fatal("CreateNote omitted transaction correlation")
	}
	if _, err := os.Stat(filepath.Join(storage.NotesDir(project), created.Filename)); err != nil {
		t.Fatalf("created note missing: %v", err)
	}
}

func TestWorkspaceNoteEmptyDataDirRejectsNonCanonicalProjectPath(t *testing.T) {
	project := &models.Project{
		ID:   "project-id",
		Path: filepath.Join(t.TempDir(), "worktrees", "alpha"),
	}
	_, err := app.NewStorageWorkspaceNotePort("").ProjectLocationPrecondition([]*models.Project{project})

	var invalid *app.InvalidInputError
	if !errors.Is(err, app.ErrInvalidInput) || !errors.As(err, &invalid) {
		t.Fatalf("ProjectLocationPrecondition error = %T %v, want InvalidInputError", err, err)
	}
	if invalid.Field != "note location" || invalid.Reason != "project path is outside the canonical workspace projects directory" {
		t.Fatalf("ProjectLocationPrecondition error = %+v", invalid)
	}
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
			// The request note is the caller's own object: the create must leave it
			// untouched and persist a revision-bearing copy instead.
			if note.RawRevision != "" {
				t.Fatalf("CreateNote wrote back into the caller's note: %+v", note)
			}

			loaded := noteByFilename(t, svc, tc.location, created.Filename)
			if loaded.RawRevision == "" {
				t.Fatalf("CreateNote did not persist a revision-bearing note: %+v", loaded)
			}
			createdRevision := loaded.RawRevision
			loaded.Body = "v2"
			updated, updateErr := svc.UpdateNote(app.NoteUpdateRequest{Location: tc.location, Note: loaded})
			if updateErr != nil {
				t.Fatalf("UpdateNote: %v", updateErr)
			}
			reloaded := noteByFilename(t, svc, tc.location, updated.Filename)
			if reloaded.RawRevision == createdRevision || reloaded.Body != "v2" {
				t.Fatalf("UpdateNote result = %+v, reloaded = %+v", updated, reloaded)
			}

			renamed, renameErr := svc.RenameNote(app.NoteRenameRequest{Location: tc.location, Note: reloaded, NewTitle: tc.name + " renamed"})
			if renameErr != nil {
				t.Fatalf("RenameNote: %v", renameErr)
			}
			if renamed.Filename == created.Filename {
				t.Fatalf("RenameNote result = %+v", renamed)
			}
			if _, statErr := os.Stat(notePathForTest(dataDir, project, tc.location, created.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("old filename still exists after rename: %v", statErr)
			}

			renamedNote := noteByFilename(t, svc, tc.location, renamed.Filename)
			if _, deleteErr := svc.RemoveNote(app.NoteDeleteRequest{Location: tc.location, Note: renamedNote}); deleteErr != nil {
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

func writeMalformedNoteForTest(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), storage.DirPermissions); err != nil {
		t.Fatalf("create malformed note directory: %v", err)
	}
	if err := os.WriteFile(path, raw, storage.FilePermissions); err != nil {
		t.Fatalf("write malformed note: %v", err)
	}
}

func TestWorkspaceMalformedNoteDeleteProjectAndGlobal(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	raw := []byte("---\ntitle: [\n---\ncontent that must never be rendered\n")

	locations := []struct {
		name     string
		location app.NoteLocation
		filename string
	}{
		{name: "project", location: app.ProjectNoteLocation(project.ID), filename: "malformed-project.md"},
		{name: "global", location: app.GlobalNoteLocation(), filename: "malformed-global.md"},
	}
	for _, tc := range locations {
		t.Run(tc.name, func(t *testing.T) {
			path := notePathForTest(dataDir, project, tc.location, tc.filename)
			writeMalformedNoteForTest(t, path, raw)
			observed := noteByFilename(t, svc, tc.location, tc.filename)
			if observed.ParseError == "" || observed.RawRevision != storage.RawArtifactRevision(raw) {
				t.Fatalf("malformed note did not retain parse error and raw revision: %+v", observed)
			}

			// A delete has no dependency on parsed metadata. Emptying the fallback
			// title makes that contract explicit rather than relying on the title
			// derived from the filename by the loader.
			observed.Title = ""
			result, err := svc.RemoveNote(app.NoteDeleteRequest{Location: tc.location, Note: observed})
			if err != nil {
				t.Fatalf("RemoveNote: %v", err)
			}
			if result.Filename != tc.filename {
				t.Fatalf("RemoveNote result = %+v", result)
			}
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Fatalf("malformed note still exists after delete: %v", err)
			}
		})
	}
}

func TestWorkspaceMalformedNoteNonDeleteMutationsRemainRejected(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	projectLocation := app.ProjectNoteLocation(project.ID)
	filename := "malformed-source.md"
	path := notePathForTest(dataDir, project, projectLocation, filename)
	raw := []byte("---\ntitle: [\n---\noriginal malformed bytes\n")
	writeMalformedNoteForTest(t, path, raw)
	observed := noteByFilename(t, svc, projectLocation, filename)

	tests := []struct {
		name   string
		mutate func(*models.Note) error
	}{
		{
			name: "create",
			mutate: func(note *models.Note) error {
				note.Filename = "malformed-create.md"
				note.RawRevision = ""
				_, err := svc.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: note})
				return err
			},
		},
		{
			name: "update",
			mutate: func(note *models.Note) error {
				_, err := svc.UpdateNote(app.NoteUpdateRequest{Location: projectLocation, Note: note})
				return err
			},
		},
		{
			name: "rename",
			mutate: func(note *models.Note) error {
				_, err := svc.RenameNote(app.NoteRenameRequest{Location: projectLocation, Note: note, NewTitle: "renamed"})
				return err
			},
		},
		{
			name: "move",
			mutate: func(note *models.Note) error {
				_, err := svc.MoveNote(app.NoteMoveRequest{Source: projectLocation, Destination: app.GlobalNoteLocation(), Note: note})
				return err
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.mutate(cloneNoteForTest(observed))
			if err == nil || !strings.Contains(err.Error(), "invalid frontmatter") {
				t.Fatalf("mutation error = %v, want invalid frontmatter rejection", err)
			}
		})
	}

	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatalf("rejected mutations changed malformed source: equal=%v err=%v", bytes.Equal(got, raw), err)
	}
	if _, err := os.Stat(notePathForTest(dataDir, project, app.GlobalNoteLocation(), filename)); !os.IsNotExist(err) {
		t.Fatalf("rejected move created a global destination: %v", err)
	}
}

func TestWorkspaceMalformedNoteDeleteBatchIsAtomicOnStaleRevision(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	location := app.ProjectNoteLocation(project.ID)
	valid, err := svc.CreateNote(app.NoteCreateRequest{
		Location: location,
		Note:     &models.Note{Title: "valid", Filename: "valid.md", Body: "keep on failure"},
	})
	if err != nil {
		t.Fatal(err)
	}

	malformedPath := notePathForTest(dataDir, project, location, "malformed.md")
	originalMalformed := []byte("---\ntitle: [\n---\noriginal malformed bytes\n")
	writeMalformedNoteForTest(t, malformedPath, originalMalformed)
	staleMalformed := noteByFilename(t, svc, location, "malformed.md")
	newerMalformed := []byte("---\ntitle: [\n---\nnewer malformed bytes\n")
	writeMalformedNoteForTest(t, malformedPath, newerMalformed)

	_, err = svc.ApplyNoteBatch(app.NoteBatchRequest{Operations: []app.NoteMutationRequest{
		{Kind: app.NoteDelete, Location: location, Note: noteByFilename(t, svc, location, valid.Filename)},
		{Kind: app.NoteDelete, Location: location, Note: staleMalformed},
	}})
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("ApplyNoteBatch error = %v, want ErrStaleArtifact", err)
	}
	if _, statErr := os.Stat(notePathForTest(dataDir, project, location, valid.Filename)); statErr != nil {
		t.Fatalf("stale malformed delete partially removed valid note: %v", statErr)
	}
	got, err := os.ReadFile(malformedPath)
	if err != nil || !bytes.Equal(got, newerMalformed) {
		t.Fatalf("stale malformed delete changed concurrent bytes: equal=%v err=%v", bytes.Equal(got, newerMalformed), err)
	}
}

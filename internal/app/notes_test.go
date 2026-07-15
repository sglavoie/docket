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
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func newNoteTestService(t *testing.T, dataDir string, projects ...*models.Project) (*app.Service, *storageRepo) {
	t.Helper()
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	commitSvc := storage.NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	repo := newStorageRepo(commitSvc, projects...)
	repo.seed(t)
	seedNoteTestCatalog(t, dataDir, projects...)
	svc := app.NewServiceWithArtifacts(
		repo,
		fixedClock{t: time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)},
		&seqIDs{},
		app.NewStorageWorkspaceNotePort(dataDir),
		nil,
	)
	return svc, repo
}

func seedNoteTestCatalog(t *testing.T, dataDir string, projects ...*models.Project) {
	t.Helper()
	store := config.NewDomainFileStore(dataDir)
	catalog := config.NewWorkspaceCatalog()
	for _, project := range projects {
		catalog.UpsertProject(project)
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
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(""), nil)

	created, err := svc.CreateNote(app.NoteCreateRequest{
		Location: app.ProjectNoteLocation(project.ID),
		Note:     &models.Note{Title: "derived workspace", Filename: "derived-workspace.md"},
	})
	if err != nil {
		t.Fatalf("CreateNote with derived workspace root: %v", err)
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
			if result.Note == nil || result.Note.ParseError == "" || result.Filename != tc.filename {
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
		{Kind: app.NoteDelete, Location: location, Note: valid.Note},
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

func TestWorkspaceNoteMoveIsAtomicOnConcurrentSourceEdit(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	tasksBefore, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatal(err)
	}
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
	if tasksAfter, readErr := os.ReadFile(storage.TasksFilePath(project.Path)); readErr != nil || !bytes.Equal(tasksAfter, tasksBefore) {
		t.Fatalf("note-only move changed tasks.md: equal=%v err=%v", bytes.Equal(tasksAfter, tasksBefore), readErr)
	}
}

type collisionRepo struct {
	*storageRepo
	injected bool
}

type guardedNotePort struct {
	app.WorkspaceNotePort
	before func()
	calls  int
}

func (p *guardedNotePort) ProjectLocationPrecondition(projects []*models.Project) (*storage.ArtifactPrecondition, error) {
	p.calls++
	if p.before != nil {
		before := p.before
		p.before = nil
		before()
	}
	return p.WorkspaceNotePort.ProjectLocationPrecondition(projects)
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
	port := &guardedNotePort{WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir)}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)

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
	ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	count, err := ledger.UndoCount(storage.GlobalScope())
	if err != nil {
		t.Fatalf("UndoCount: %v", err)
	}
	return count
}

func TestWorkspaceNoteEveryProjectMutationCarriesLocationGuard(t *testing.T) {
	for _, operation := range []string{"create", "update", "rename", "delete", "global-to-project", "project-to-global"} {
		t.Run(operation, func(t *testing.T) {
			dataDir := t.TempDir()
			project := realTestProject(t, dataDir, "alpha", "seed")
			baseService, repo := newNoteTestService(t, dataDir, project)
			projectLocation := app.ProjectNoteLocation(project.ID)
			globalLocation := app.GlobalNoteLocation()
			tasksBefore, err := os.ReadFile(storage.TasksFilePath(project.Path))
			if err != nil {
				t.Fatal(err)
			}

			var note *models.Note
			var sourcePath, destinationPath string
			switch operation {
			case "create":
			case "update", "rename", "delete", "project-to-global":
				created, createErr := baseService.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md", Body: "before"}})
				if createErr != nil {
					t.Fatal(createErr)
				}
				note = created.Note
				if operation == "project-to-global" {
					sourcePath = notePathForTest(dataDir, project, projectLocation, created.Filename)
					destinationPath = notePathForTest(dataDir, project, globalLocation, created.Filename)
				}
			case "global-to-project":
				created, createErr := baseService.CreateNote(app.NoteCreateRequest{Location: globalLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
				if createErr != nil {
					t.Fatal(createErr)
				}
				note = created.Note
				sourcePath = notePathForTest(dataDir, project, globalLocation, created.Filename)
				destinationPath = notePathForTest(dataDir, project, projectLocation, created.Filename)
			}

			baselineUndo := noteUndoCount(t, dataDir)
			port := &guardedNotePort{
				WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir),
				before: func() {
					mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
						catalog.Projects[0].Archived = true
					})
				},
			}
			guardedService := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)
			var mutate func() error
			switch operation {
			case "create":
				mutate = func() error {
					_, mutationErr := guardedService.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
					return mutationErr
				}
			case "update":
				note.Body = "after"
				mutate = func() error {
					_, mutationErr := guardedService.UpdateNote(app.NoteUpdateRequest{Location: projectLocation, Note: note})
					return mutationErr
				}
			case "rename":
				mutate = func() error {
					_, mutationErr := guardedService.RenameNote(app.NoteRenameRequest{Location: projectLocation, Note: note, NewTitle: "renamed"})
					return mutationErr
				}
			case "delete":
				mutate = func() error {
					_, mutationErr := guardedService.RemoveNote(app.NoteDeleteRequest{Location: projectLocation, Note: note})
					return mutationErr
				}
			case "global-to-project":
				mutate = func() error {
					_, mutationErr := guardedService.MoveNote(app.NoteMoveRequest{Source: globalLocation, Destination: projectLocation, Note: note})
					return mutationErr
				}
			case "project-to-global":
				mutate = func() error {
					_, mutationErr := guardedService.MoveNote(app.NoteMoveRequest{Source: projectLocation, Destination: globalLocation, Note: note})
					return mutationErr
				}
			}

			err = mutate()
			var locationConflict *app.ProjectLocationConflictError
			if !errors.Is(err, storage.ErrStaleArtifact) || !errors.As(err, &locationConflict) || locationConflict.Reason != "archived" {
				t.Fatalf("mutation error = %T %v, want archived ProjectLocationConflictError", err, err)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo {
				t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
			}
			if sourcePath != "" {
				if _, statErr := os.Stat(sourcePath); statErr != nil {
					t.Fatalf("rejected move removed its source: %v", statErr)
				}
				if _, statErr := os.Stat(destinationPath); !os.IsNotExist(statErr) {
					t.Fatalf("rejected move created its destination: %v", statErr)
				}
			}
			tasksAfter, readErr := os.ReadFile(storage.TasksFilePath(project.Path))
			if readErr != nil || !bytes.Equal(tasksAfter, tasksBefore) {
				t.Fatalf("note-only failure changed tasks.md: equal=%v err=%v", bytes.Equal(tasksAfter, tasksBefore), readErr)
			}
		})
	}
}

func TestWorkspaceNoteRejectsMissingAndMovedCatalogLocations(t *testing.T) {
	for _, state := range []string{"missing", "moved"} {
		t.Run(state, func(t *testing.T) {
			dataDir := t.TempDir()
			project := realTestProject(t, dataDir, "alpha", "seed")
			_, repo := newNoteTestService(t, dataDir, project)
			baselineUndo := noteUndoCount(t, dataDir)
			port := &guardedNotePort{
				WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir),
				before: func() {
					mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
						if state == "missing" {
							catalog.Projects = nil
							return
						}
						catalog.Projects[0].Slug = "renamed"
					})
				},
			}
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)
			_, err := svc.CreateNote(app.NoteCreateRequest{Location: app.ProjectNoteLocation(project.ID), Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
			var conflict *app.ProjectLocationConflictError
			if !errors.As(err, &conflict) || conflict.Reason != state || !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("CreateNote error = %T %v, want %s ProjectLocationConflictError", err, err, state)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo {
				t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
			}
			if _, statErr := os.Stat(filepath.Join(storage.NotesDir(project), "guarded.md")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected create wrote note: %v", statErr)
			}
		})
	}
}

func TestWorkspaceNoteBatchChecksEveryTouchedProjectLocation(t *testing.T) {
	dataDir := t.TempDir()
	alpha := realTestProject(t, dataDir, "alpha", "alpha task")
	beta := realTestProject(t, dataDir, "beta", "beta task")
	baseService, repo := newNoteTestService(t, dataDir, alpha, beta)
	alphaLocation := app.ProjectNoteLocation(alpha.ID)
	betaLocation := app.ProjectNoteLocation(beta.ID)
	alphaNote, err := baseService.CreateNote(app.NoteCreateRequest{Location: alphaLocation, Note: &models.Note{Title: "alpha", Filename: "alpha.md", Body: "before"}})
	if err != nil {
		t.Fatal(err)
	}
	betaNote, err := baseService.CreateNote(app.NoteCreateRequest{Location: betaLocation, Note: &models.Note{Title: "beta", Filename: "beta.md", Body: "before"}})
	if err != nil {
		t.Fatal(err)
	}
	alphaUpdate := cloneNoteForTest(alphaNote.Note)
	alphaUpdate.Body = "after"
	betaUpdate := cloneNoteForTest(betaNote.Note)
	betaUpdate.Body = "after"
	baselineUndo := noteUndoCount(t, dataDir)
	port := &guardedNotePort{
		WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir),
		before: func() {
			mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
				for i := range catalog.Projects {
					if catalog.Projects[i].ID == beta.ID {
						catalog.Projects[i].Archived = true
					}
				}
			})
		},
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)
	_, err = svc.ApplyNoteBatch(app.NoteBatchRequest{Operations: []app.NoteMutationRequest{
		{Kind: app.NoteUpdate, Location: alphaLocation, Note: alphaUpdate},
		{Kind: app.NoteUpdate, Location: betaLocation, Note: betaUpdate},
	}})
	var conflict *app.ProjectLocationConflictError
	if !errors.As(err, &conflict) || conflict.ProjectID != beta.ID || conflict.Reason != "archived" {
		t.Fatalf("ApplyNoteBatch error = %T %v, want beta location conflict", err, err)
	}
	if got := noteByFilename(t, baseService, alphaLocation, alphaUpdate.Filename).Body; got != "before" {
		t.Fatalf("alpha note partially committed: %q", got)
	}
	if got := noteByFilename(t, baseService, betaLocation, betaUpdate.Filename).Body; got != "before" {
		t.Fatalf("beta note changed: %q", got)
	}
	if got := noteUndoCount(t, dataDir); got != baselineUndo {
		t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
	}
}

type catalogRaceRepo struct {
	*storageRepo
	beforeCommit func()
}

func (r *catalogRaceRepo) CommitArtifactTransaction(request storage.ArtifactCommitRequest) (string, error) {
	if r.beforeCommit != nil {
		before := r.beforeCommit
		r.beforeCommit = nil
		before()
	}
	return r.storageRepo.CommitArtifactTransaction(request)
}

func TestWorkspaceNoteCatalogPreconditionClosesCommitRace(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	_, baseRepo := newNoteTestService(t, dataDir, project)
	tasksBefore, err := os.ReadFile(storage.TasksFilePath(project.Path))
	if err != nil {
		t.Fatal(err)
	}
	baselineUndo := noteUndoCount(t, dataDir)
	var concurrentCatalog []byte
	repo := &catalogRaceRepo{
		storageRepo: baseRepo,
		beforeCommit: func() {
			mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
				catalog.Projects[0].Name = "Concurrent catalog edit"
			})
			concurrentCatalog, err = os.ReadFile(config.WorkspaceCatalogPath(dataDir))
			if err != nil {
				t.Fatal(err)
			}
		},
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	_, err = svc.CreateNote(app.NoteCreateRequest{Location: app.ProjectNoteLocation(project.ID), Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
	var conflict *storage.ArtifactConflictError
	if !errors.As(err, &conflict) || !errors.Is(err, storage.ErrStaleArtifact) || conflict.Path != config.WorkspaceCatalogPath(dataDir) {
		t.Fatalf("CreateNote error = %T %v, want workspace catalog ArtifactConflictError", err, err)
	}
	if _, statErr := os.Stat(filepath.Join(storage.NotesDir(project), "guarded.md")); !os.IsNotExist(statErr) {
		t.Fatalf("catalog race wrote note: %v", statErr)
	}
	if got := noteUndoCount(t, dataDir); got != baselineUndo {
		t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
	}
	if got, readErr := os.ReadFile(config.WorkspaceCatalogPath(dataDir)); readErr != nil || !bytes.Equal(got, concurrentCatalog) {
		t.Fatalf("catalog race changed concurrent catalog: equal=%v err=%v", bytes.Equal(got, concurrentCatalog), readErr)
	}
	if got, readErr := os.ReadFile(storage.TasksFilePath(project.Path)); readErr != nil || !bytes.Equal(got, tasksBefore) {
		t.Fatalf("catalog race changed tasks.md: equal=%v err=%v", bytes.Equal(got, tasksBefore), readErr)
	}
}

func TestWorkspaceNoteCatalogRaceDoesNotRecreateStaleProjectDirectory(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	baseService, baseRepo := newNoteTestService(t, dataDir, project)
	globalLocation := app.GlobalNoteLocation()
	projectLocation := app.ProjectNoteLocation(project.ID)
	created, err := baseService.CreateNote(app.NoteCreateRequest{Location: globalLocation, Note: &models.Note{Title: "move", Filename: "move.md"}})
	if err != nil {
		t.Fatal(err)
	}
	sourcePath := notePathForTest(dataDir, project, globalLocation, created.Filename)
	baselineUndo := noteUndoCount(t, dataDir)
	repo := &catalogRaceRepo{
		storageRepo: baseRepo,
		beforeCommit: func() {
			if removeErr := os.RemoveAll(project.Path); removeErr != nil {
				t.Fatal(removeErr)
			}
			mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
				catalog.Projects[0].Slug = "renamed"
			})
		},
	}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir), nil)
	_, err = svc.MoveNote(app.NoteMoveRequest{Source: globalLocation, Destination: projectLocation, Note: created.Note})
	if !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("MoveNote error = %v, want ErrStaleArtifact", err)
	}
	if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
		t.Fatalf("catalog race recreated stale project directory: %v", statErr)
	}
	if _, statErr := os.Stat(sourcePath); statErr != nil {
		t.Fatalf("catalog race removed global source: %v", statErr)
	}
	if got := noteUndoCount(t, dataDir); got != baselineUndo {
		t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
	}
}

func TestWorkspaceNoteStaleProjectDirectoryIsNotRecreated(t *testing.T) {
	for _, operation := range []string{"create", "project-to-global"} {
		t.Run(operation, func(t *testing.T) {
			dataDir := t.TempDir()
			project := realTestProject(t, dataDir, "alpha", "seed")
			baseService, repo := newNoteTestService(t, dataDir, project)
			projectLocation := app.ProjectNoteLocation(project.ID)
			var note *models.Note
			if operation == "project-to-global" {
				created, err := baseService.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
				if err != nil {
					t.Fatal(err)
				}
				note = created.Note
			}
			baselineUndo := noteUndoCount(t, dataDir)
			port := &guardedNotePort{
				WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir),
				before: func() {
					if err := os.RemoveAll(project.Path); err != nil {
						t.Fatal(err)
					}
					mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) { catalog.Projects = nil })
				},
			}
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)
			var err error
			if operation == "create" {
				_, err = svc.CreateNote(app.NoteCreateRequest{Location: projectLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
			} else {
				_, err = svc.MoveNote(app.NoteMoveRequest{Source: projectLocation, Destination: app.GlobalNoteLocation(), Note: note})
			}
			if !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("mutation error = %v, want ErrStaleArtifact", err)
			}
			if _, statErr := os.Stat(project.Path); !os.IsNotExist(statErr) {
				t.Fatalf("rejected mutation recreated stale project directory: %v", statErr)
			}
			if _, statErr := os.Stat(filepath.Join(storage.GlobalNotesDir(dataDir), "guarded.md")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected mutation created global destination: %v", statErr)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo {
				t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
			}
		})
	}
}

func TestWorkspaceGlobalOnlyNoteMutationSkipsCatalogRead(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	_, repo := newNoteTestService(t, dataDir, project)
	if err := os.Remove(config.WorkspaceCatalogPath(dataDir)); err != nil {
		t.Fatal(err)
	}
	port := &guardedNotePort{WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir)}
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port, nil)
	result, err := svc.CreateNote(app.NoteCreateRequest{Location: app.GlobalNoteLocation(), Note: &models.Note{Title: "global", Filename: "global.md"}})
	if err != nil {
		t.Fatalf("CreateNote: %v", err)
	}
	if port.calls != 0 {
		t.Fatalf("global-only mutation built %d project preconditions", port.calls)
	}
	if _, err := os.Stat(filepath.Join(storage.GlobalNotesDir(dataDir), result.Filename)); err != nil {
		t.Fatalf("global note missing: %v", err)
	}
}

package app_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

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
				note = noteByFilename(t, baseService, projectLocation, created.Filename)
				if operation == "project-to-global" {
					sourcePath = notePathForTest(dataDir, project, projectLocation, created.Filename)
					destinationPath = notePathForTest(dataDir, project, globalLocation, created.Filename)
				}
			case "global-to-project":
				created, createErr := baseService.CreateNote(app.NoteCreateRequest{Location: globalLocation, Note: &models.Note{Title: "guarded", Filename: "guarded.md"}})
				if createErr != nil {
					t.Fatal(createErr)
				}
				note = noteByFilename(t, baseService, globalLocation, created.Filename)
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
			guardedService := app.NewServiceWithArtifacts(repo, nil, nil, port)
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

func TestWorkspaceNoteMoveChecksBothProjectLocations(t *testing.T) {
	for _, archived := range []string{"source", "destination"} {
		t.Run(archived, func(t *testing.T) {
			dataDir := t.TempDir()
			alpha := realTestProject(t, dataDir, "alpha", "alpha task")
			beta := realTestProject(t, dataDir, "beta", "beta task")
			baseService, repo := newNoteTestService(t, dataDir, alpha, beta)
			source := app.ProjectNoteLocation(alpha.ID)
			destination := app.ProjectNoteLocation(beta.ID)
			created, err := baseService.CreateNote(app.NoteCreateRequest{
				Location: source,
				Note:     &models.Note{Title: "guarded move", Filename: "guarded-move.md"},
			})
			if err != nil {
				t.Fatal(err)
			}
			baselineUndo := noteUndoCount(t, dataDir)
			archivedID := alpha.ID
			if archived == "destination" {
				archivedID = beta.ID
			}
			port := &guardedNotePort{
				WorkspaceNotePort: app.NewStorageWorkspaceNotePort(dataDir),
				before: func() {
					mutateNoteTestCatalog(t, dataDir, func(catalog *config.WorkspaceCatalog) {
						for i := range catalog.Projects {
							if catalog.Projects[i].ID == archivedID {
								catalog.Projects[i].Archived = true
							}
						}
					})
				},
			}
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port)

			_, err = svc.MoveNote(app.NoteMoveRequest{Source: source, Destination: destination, Note: noteByFilename(t, baseService, source, created.Filename)})
			var conflict *app.ProjectLocationConflictError
			if !errors.Is(err, storage.ErrStaleArtifact) || !errors.As(err, &conflict) || conflict.ProjectID != archivedID || conflict.Reason != "archived" {
				t.Fatalf("MoveNote error = %T %v, want archived conflict for %s", err, err, archivedID)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo {
				t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
			}
			if _, statErr := os.Stat(notePathForTest(dataDir, alpha, source, created.Filename)); statErr != nil {
				t.Fatalf("rejected move removed source: %v", statErr)
			}
			if _, statErr := os.Stat(notePathForTest(dataDir, beta, destination, created.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("rejected move created destination: %v", statErr)
			}
		})
	}
}

func TestWorkspaceNoteMoveRejectsMissingAndMovedCatalogLocations(t *testing.T) {
	for _, state := range []string{"missing", "moved"} {
		t.Run(state, func(t *testing.T) {
			dataDir := t.TempDir()
			project := realTestProject(t, dataDir, "alpha", "seed")
			baseService, repo := newNoteTestService(t, dataDir, project)
			source := app.GlobalNoteLocation()
			destination := app.ProjectNoteLocation(project.ID)
			created, err := baseService.CreateNote(app.NoteCreateRequest{
				Location: source,
				Note:     &models.Note{Title: "guarded", Filename: "guarded.md"},
			})
			if err != nil {
				t.Fatal(err)
			}
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
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port)
			_, err = svc.MoveNote(app.NoteMoveRequest{Source: source, Destination: destination, Note: noteByFilename(t, baseService, source, created.Filename)})
			var conflict *app.ProjectLocationConflictError
			if !errors.As(err, &conflict) || conflict.Reason != state || !errors.Is(err, storage.ErrStaleArtifact) {
				t.Fatalf("MoveNote error = %T %v, want %s ProjectLocationConflictError", err, err, state)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo {
				t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
			}
			if _, statErr := os.Stat(filepath.Join(storage.NotesDir(project), "guarded.md")); !os.IsNotExist(statErr) {
				t.Fatalf("rejected move created destination: %v", statErr)
			}
			if _, statErr := os.Stat(notePathForTest(dataDir, project, source, created.Filename)); statErr != nil {
				t.Fatalf("rejected move removed source: %v", statErr)
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
	alphaUpdate := cloneNoteForTest(noteByFilename(t, baseService, alphaLocation, alphaNote.Filename))
	alphaUpdate.Body = "after"
	betaUpdate := cloneNoteForTest(noteByFilename(t, baseService, betaLocation, betaNote.Filename))
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
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port)
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

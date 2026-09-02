package app_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
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

func TestWorkspaceNoteMoveCatalogRaceDoesNotRecreateStaleProjectDirectory(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	baseService, baseRepo := newNoteTestService(t, dataDir, project)
	globalLocation := app.GlobalNoteLocation()
	projectLocation := app.ProjectNoteLocation(project.ID)
	created, err := baseService.CreateNote(app.NoteCreateRequest{
		Location: globalLocation,
		Note:     &models.Note{Title: "move", Filename: "move.md"},
	})
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
	svc := app.NewServiceWithArtifacts(repo, nil, nil, app.NewStorageWorkspaceNotePort(dataDir))
	_, err = svc.MoveNote(app.NoteMoveRequest{Source: globalLocation, Destination: projectLocation, Note: noteByFilename(t, baseService, globalLocation, created.Filename)})
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
				created, err := baseService.CreateNote(app.NoteCreateRequest{
					Location: projectLocation,
					Note:     &models.Note{Title: "guarded", Filename: "guarded.md"},
				})
				if err != nil {
					t.Fatal(err)
				}
				note = noteByFilename(t, baseService, projectLocation, created.Filename)
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
			svc := app.NewServiceWithArtifacts(repo, nil, nil, port)
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
	svc := app.NewServiceWithArtifacts(repo, nil, nil, port)
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

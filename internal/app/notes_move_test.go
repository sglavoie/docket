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
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

//nolint:gocyclo // One table-driven integration test keeps the three location transitions comparable.
func TestWorkspaceNoteMovesBetweenAllLocationsAndUndoRedo(t *testing.T) {
	for _, tc := range []struct {
		name        string
		source      func(alpha, beta *models.Project) app.NoteLocation
		destination func(alpha, beta *models.Project) app.NoteLocation
	}{
		{
			name:        "global-to-project",
			source:      func(_, _ *models.Project) app.NoteLocation { return app.GlobalNoteLocation() },
			destination: func(alpha, _ *models.Project) app.NoteLocation { return app.ProjectNoteLocation(alpha.ID) },
		},
		{
			name:        "project-to-global",
			source:      func(alpha, _ *models.Project) app.NoteLocation { return app.ProjectNoteLocation(alpha.ID) },
			destination: func(_, _ *models.Project) app.NoteLocation { return app.GlobalNoteLocation() },
		},
		{
			name:        "project-to-project",
			source:      func(alpha, _ *models.Project) app.NoteLocation { return app.ProjectNoteLocation(alpha.ID) },
			destination: func(_, beta *models.Project) app.NoteLocation { return app.ProjectNoteLocation(beta.ID) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dataDir := t.TempDir()
			alpha := realTestProject(t, dataDir, "alpha", "alpha task")
			beta := realTestProject(t, dataDir, "beta", "beta task")
			svc, _ := newNoteTestService(t, dataDir, alpha, beta)
			source := tc.source(alpha, beta)
			destination := tc.destination(alpha, beta)
			notePath := func(location app.NoteLocation, filename string) string {
				project := alpha
				if location.ProjectID == beta.ID {
					project = beta
				}
				return notePathForTest(dataDir, project, location, filename)
			}
			createdAt := time.Date(2026, 6, 10, 9, 30, 0, 0, time.UTC)
			modifiedAt := time.Date(2026, 6, 11, 10, 45, 0, 0, time.UTC)
			created, err := svc.CreateNote(app.NoteCreateRequest{
				Location: source,
				Note: &models.Note{
					Title:      "move intact",
					Filename:   "move-intact.md",
					Body:       "preserve this body",
					CreatedAt:  createdAt,
					ModifiedAt: modifiedAt,
				},
			})
			if err != nil {
				t.Fatalf("CreateNote: %v", err)
			}
			baselineUndo := noteUndoCount(t, dataDir)

			createdNote := noteByFilename(t, svc, source, created.Filename)
			createdBody := createdNote.Body
			moved, err := svc.MoveNote(app.NoteMoveRequest{Source: source, Destination: destination, Note: createdNote})
			if err != nil {
				t.Fatalf("MoveNote: %v", err)
			}
			if moved.Kind != app.NoteMove || moved.Location != destination || moved.ProjectID != destination.ProjectID {
				t.Fatalf("MoveNote identity = %+v", moved)
			}
			if moved.Filename != created.Filename || moved.Title != created.Title {
				t.Fatalf("MoveNote filename/title result = %+v", moved)
			}
			if _, statErr := os.Stat(notePath(source, created.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("move source still exists: %v", statErr)
			}
			movedNote := noteByFilename(t, svc, destination, moved.Filename)
			if movedNote.Body != createdBody || !movedNote.CreatedAt.Equal(createdAt) || !movedNote.ModifiedAt.Equal(modifiedAt) {
				t.Fatalf("MoveNote changed note content or timestamps: %+v", movedNote)
			}
			if got := noteUndoCount(t, dataDir); got != baselineUndo+1 {
				t.Fatalf("undo count = %d, want %d", got, baselineUndo+1)
			}

			ledger := storage.NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
			entry, ok, err := ledger.SelectNewestUndo(storage.GlobalScope())
			if err != nil || !ok {
				t.Fatalf("select move undo entry: ok=%v err=%v", ok, err)
			}
			var expectedProjectIDs []string
			for _, location := range []app.NoteLocation{source, destination} {
				if location.Scope == app.NoteScopeProject && (len(expectedProjectIDs) == 0 || expectedProjectIDs[len(expectedProjectIDs)-1] != location.ProjectID) {
					expectedProjectIDs = append(expectedProjectIDs, location.ProjectID)
				}
			}
			if got, want := strings.Join(entry.ProjectIDs, ","), strings.Join(expectedProjectIDs, ","); got != want {
				t.Fatalf("move ledger project IDs = %q, want %q", got, want)
			}
			if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
				t.Fatalf("undo move: %v", err)
			}
			if _, statErr := os.Stat(notePath(destination, moved.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("undo left destination behind: %v", statErr)
			}
			if restored := noteByFilename(t, svc, source, created.Filename); restored.Body != createdBody {
				t.Fatalf("undo restored source = %+v", restored)
			}

			if _, err := ledger.Redo(storage.GlobalScope()); err != nil {
				t.Fatalf("redo move: %v", err)
			}
			if _, statErr := os.Stat(notePath(source, created.Filename)); !os.IsNotExist(statErr) {
				t.Fatalf("redo left source behind: %v", statErr)
			}
			if redone := noteByFilename(t, svc, destination, moved.Filename); redone.Body != createdBody {
				t.Fatalf("redo restored destination = %+v", redone)
			}
		})
	}
}

func TestWorkspaceNoteMoveRetriesDestinationCollision(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	global := app.GlobalNoteLocation()
	destination := app.ProjectNoteLocation(project.ID)
	source, err := svc.CreateNote(app.NoteCreateRequest{
		Location: global,
		Note:     &models.Note{Title: "move me", Filename: "collision.md", Body: "source body"},
	})
	if err != nil {
		t.Fatal(err)
	}
	existing, err := svc.CreateNote(app.NoteCreateRequest{
		Location: destination,
		Note:     &models.Note{Title: "existing", Filename: "collision.md", Body: "destination body"},
	})
	if err != nil {
		t.Fatal(err)
	}
	baselineUndo := noteUndoCount(t, dataDir)

	moved, err := svc.MoveNote(app.NoteMoveRequest{Source: global, Destination: destination, Note: noteByFilename(t, svc, global, source.Filename)})
	if err != nil {
		t.Fatalf("MoveNote: %v", err)
	}
	if moved.Filename != "collision-2.md" {
		t.Fatalf("MoveNote collision result = %+v", moved)
	}
	if got := noteByFilename(t, svc, destination, existing.Filename); got.Body != "destination body" {
		t.Fatalf("existing destination changed: %+v", got)
	}
	if got := noteByFilename(t, svc, destination, moved.Filename); got.Body != "source body" {
		t.Fatalf("deduplicated destination = %+v", got)
	}
	if _, statErr := os.Stat(notePathForTest(dataDir, project, global, source.Filename)); !os.IsNotExist(statErr) {
		t.Fatalf("move source still exists: %v", statErr)
	}
	if got := noteUndoCount(t, dataDir); got != baselineUndo+1 {
		t.Fatalf("collision retry created %d undo entries, want one new entry", got-baselineUndo)
	}
}

func TestWorkspaceNoteMoveRejectsSameLocationWithoutUndoEntry(t *testing.T) {
	dataDir := t.TempDir()
	project := realTestProject(t, dataDir, "alpha", "seed")
	svc, _ := newNoteTestService(t, dataDir, project)
	location := app.ProjectNoteLocation(project.ID)
	created, err := svc.CreateNote(app.NoteCreateRequest{
		Location: location,
		Note:     &models.Note{Title: "stay put", Filename: "stay-put.md"},
	})
	if err != nil {
		t.Fatal(err)
	}
	baselineUndo := noteUndoCount(t, dataDir)

	_, err = svc.MoveNote(app.NoteMoveRequest{Source: location, Destination: location, Note: noteByFilename(t, svc, location, created.Filename)})
	var invalid *app.InvalidInputError
	if !errors.Is(err, app.ErrInvalidInput) || !errors.As(err, &invalid) || invalid.Field != "note destination" {
		t.Fatalf("MoveNote error = %T %v, want note destination InvalidInputError", err, err)
	}
	if got := noteUndoCount(t, dataDir); got != baselineUndo {
		t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
	}
	if got := noteByFilename(t, svc, location, created.Filename); got.Title != created.Title {
		t.Fatalf("same-location rejection changed source: %+v", got)
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
	baselineUndo := noteUndoCount(t, dataDir)
	stale := noteByFilename(t, svc, global, created.Filename)
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
	if got := noteUndoCount(t, dataDir); got != baselineUndo {
		t.Fatalf("undo count = %d, want unchanged %d", got, baselineUndo)
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
	if moved.Location != destination || moved.ProjectID != project.ID {
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

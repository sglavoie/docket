package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/route"
)

type notesMoveService struct {
	combinedNotesService
	result   app.NoteBatchResult
	applyErr error
}

func (s *notesMoveService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, notes.CloneNoteBatchRequest(request))
	if s.applyErr != nil {
		return app.NoteBatchResult{}, s.applyErr
	}
	return s.result, nil
}

func TestNotesMoveResultSelectsCommittedDestinationIdentity(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, alpha, beta)
	_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.AllNotesScope()})
	view := shell.getActiveNotesView()
	source := app.ProjectNoteLocation(alpha.ID)
	destination := app.ProjectNoteLocation(beta.ID)
	filename := "duplicate.md"
	service := &notesMoveService{combinedNotesService: combinedNotesService{
		byLocation: map[app.NoteLocation][]*models.Note{
			source: {
				{Title: "Needle source", Filename: filename, Body: "needle", ModifiedAt: time.Unix(1, 0)},
			},
			app.GlobalNoteLocation(): {
				{Title: "Needle global", Filename: filename, Body: "needle", ModifiedAt: time.Unix(3, 0)},
			},
		},
	}}
	view.NoteService = service
	view.SearchApplied = "needle"
	view.LoadNotes()

	service.byLocation[source] = nil
	service.byLocation[destination] = []*models.Note{{
		Title: "Needle source", Filename: filename, Body: "needle", ModifiedAt: time.Unix(2, 0),
	}}
	shell.reconcileNoteResult(notes.ProjectNoteMutationResultMsg{
		Location: source,
		Result: app.NoteBatchResult{Results: []app.NoteResult{{
			Kind:      app.NoteMove,
			Location:  destination,
			ProjectID: destination.ProjectID,
			Filename:  filename,
		}}},
		SuccessMessage: "Moved",
	})

	want := notes.NoteRowIdentity{Location: destination, Filename: filename}
	if view.SearchQuery() != "needle" || len(view.VisibleRows) != 2 ||
		view.CurrentRow() == nil || view.CurrentRow().Identity() != want {
		t.Fatalf(
			"move reconciliation = query %q rows %d current %+v, want destination %+v",
			view.SearchQuery(),
			len(view.VisibleRows),
			view.CurrentRow(),
			want,
		)
	}
}

func TestNotesMoveSourceScopeReappliesSearchAndClampsAfterRowDisappears(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, alpha, beta)
	_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{
		Scope: route.ProjectNotesScope(alpha.ID),
	})
	view := shell.getActiveNotesView()
	source := app.ProjectNoteLocation(alpha.ID)
	destination := app.ProjectNoteLocation(beta.ID)
	filename := "source.md"
	service := &notesMoveService{combinedNotesService: combinedNotesService{
		byLocation: map[app.NoteLocation][]*models.Note{
			source: {
				{Title: "Needle source", Filename: filename, Body: "needle"},
			},
		},
	}}
	view.NoteService = service
	view.SearchApplied = "needle"
	view.LoadNotes()

	service.byLocation[source] = nil
	service.byLocation[destination] = []*models.Note{{
		Title: "Needle source", Filename: filename, Body: "needle",
	}}
	shell.reconcileNoteResult(notes.ProjectNoteMutationResultMsg{
		Location: source,
		Result: app.NoteBatchResult{Results: []app.NoteResult{{
			Kind:      app.NoteMove,
			Location:  destination,
			ProjectID: destination.ProjectID,
			Filename:  filename,
		}}},
		SuccessMessage: "Moved",
	})

	if view.SearchQuery() != "needle" || len(view.LoadedRows) != 1 ||
		len(view.VisibleRows) != 0 || view.Cursor != 0 {
		t.Fatalf(
			"source-scope reconciliation = query %q loaded %d visible %d cursor %d",
			view.SearchQuery(),
			len(view.LoadedRows),
			len(view.VisibleRows),
			view.Cursor,
		)
	}
}

func TestNotesMoveUndoRedoRefreshesEveryAffectedOwner(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Undoable move")
	env.view.SearchApplied = "undoable"
	env.view.RederiveVisibleRows()
	source := app.GlobalNoteLocation()
	destination := app.ProjectNoteLocation(env.project.ID)

	_, _ = env.view.Update(syntheticKey('%'))
	_, pickerCmd := env.view.Update(syntheticSpecialKey(tea.KeyEnter))
	pickerResult := pickerCmd().(notes.NotesLocationPickerResultMsg)
	_, mutationCmd := env.view.Update(pickerResult)
	mutation := runProjectNoteMutation(t, func() any { return mutationCmd() })
	env.shell.handleNotesMessages(mutation)
	if env.view.SearchQuery() != "undoable" || env.view.CurrentRow() == nil ||
		env.view.CurrentRow().Location != destination {
		t.Fatalf("committed move did not refresh destination: %#v", env.view.CurrentRow())
	}

	undo, ok := env.view.UndoDelete()().(notes.ProjectNoteLedgerResultMsg)
	if !ok || undo.Redo {
		t.Fatalf("move undo result = %#v", undo)
	}
	env.shell.handleNotesMessages(undo)
	if env.view.SearchQuery() != "undoable" || env.view.CurrentRow() == nil ||
		env.view.CurrentRow().Location != source {
		t.Fatalf("move undo did not refresh source: %#v", env.view.CurrentRow())
	}

	redo, ok := env.view.RedoNoteMutation()().(notes.ProjectNoteLedgerResultMsg)
	if !ok || !redo.Redo {
		t.Fatalf("move redo result = %#v", redo)
	}
	env.shell.handleNotesMessages(redo)
	if env.view.SearchQuery() != "undoable" || env.view.CurrentRow() == nil ||
		env.view.CurrentRow().Location != destination {
		t.Fatalf("move redo did not refresh destination: %#v", env.view.CurrentRow())
	}
}

package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

type notesMoveService struct {
	combinedNotesService
	result   app.NoteBatchResult
	applyErr error
}

func (s *notesMoveService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, cloneNoteBatchRequest(request))
	if s.applyErr != nil {
		return app.NoteBatchResult{}, s.applyErr
	}
	return s.result, nil
}

func TestNotesMovePickerOffersEveryOtherOwner(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	locations := []app.NoteLocation{
		app.GlobalNoteLocation(),
		app.ProjectNoteLocation(alpha.ID),
		app.ProjectNoteLocation(beta.ID),
	}

	for _, source := range []app.NoteLocation{
		app.GlobalNoteLocation(),
		app.ProjectNoteLocation(alpha.ID),
	} {
		t.Run(string(source.Scope)+"-"+source.ProjectID, func(t *testing.T) {
			note := &models.Note{Title: "Move me", Filename: "move.md", RawRevision: "observed"}
			service := &notesMoveService{combinedNotesService: combinedNotesService{
				byLocation: map[app.NoteLocation][]*models.Note{source: {note}},
			}}
			view := NewNotesViewModelWithService(
				t.TempDir(),
				AllNotesScope(),
				app.GlobalNoteLocation(),
				service,
				nil,
				alpha,
				beta,
			)

			_, _ = view.Update(syntheticKey('m'))
			if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerMove {
				t.Fatal("m did not open the move destination picker")
			}
			var got []app.NoteLocation
			for _, item := range view.locationPicker.SourceItems() {
				got = append(got, item.Location)
				if item.Scope.Kind == NotesScopeAll {
					t.Fatal("move picker offered All Notes as a destination")
				}
			}
			var want []app.NoteLocation
			for _, location := range locations {
				if location != source {
					want = append(want, location)
				}
			}
			if !slices.Equal(got, want) {
				t.Fatalf("move destinations from %+v = %+v, want %+v", source, got, want)
			}
		})
	}
}

func TestNotesMoveEnterCommitsFrozenLocatedRequestWithoutConfirmation(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	source := app.ProjectNoteLocation(alpha.ID)
	destination := app.ProjectNoteLocation(beta.ID)
	note := &models.Note{
		Title:       "Frozen title",
		Filename:    "same.md",
		Body:        "frozen body",
		RawRevision: "observed-revision",
	}
	service := &notesMoveService{
		combinedNotesService: combinedNotesService{
			byLocation: map[app.NoteLocation][]*models.Note{source: {note}},
		},
		result: app.NoteBatchResult{Results: []app.NoteResult{{
			Kind:           app.NoteMove,
			Location:       destination,
			SourceLocation: source,
			ProjectID:      destination.ProjectID,
			Filename:       note.Filename,
		}}},
	}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		app.GlobalNoteLocation(),
		service,
		nil,
		alpha,
		beta,
	)

	_, _ = view.Update(syntheticKey('m'))
	view.locationPicker.SetCursorToID(notesLocationPickerItem{
		Scope: ProjectNotesScope(beta.ID),
	}.ItemID())
	note.Title = "live pointer changed"
	note.Body = "live body changed"
	note.RawRevision = "new-revision"

	_, resultCmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	if resultCmd == nil || view.confirm.IsActive() {
		t.Fatal("move Enter did not select immediately")
	}
	result, ok := resultCmd().(notesLocationPickerResultMsg)
	if !ok || !result.Selected || result.Item.Location != destination {
		t.Fatalf("move picker result = %#v, want destination %+v", result, destination)
	}
	_, mutationCmd := view.Update(result)
	if mutationCmd == nil || view.locationPicker != nil {
		t.Fatal("selected move destination did not emit an immediate mutation")
	}
	message, ok := mutationCmd().(ProjectNoteMutationResultMsg)
	if !ok || message.Location != source {
		t.Fatalf("move message = %#v, want source context %+v", message, source)
	}
	if len(service.requests) != 1 || len(service.requests[0].Operations) != 1 {
		t.Fatalf("move requests = %#v, want one operation", service.requests)
	}
	operation := service.requests[0].Operations[0]
	if operation.Kind != app.NoteMove || operation.Location != source ||
		operation.Destination != destination || operation.Note == note ||
		operation.Note.Title != "Frozen title" ||
		operation.Note.Body != "frozen body" ||
		operation.Note.RawRevision != "observed-revision" {
		t.Fatalf("move operation did not retain its frozen located note: %#v", operation)
	}
}

func TestNotesMoveRejectsMalformedNotesAndMissingDestinations(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	t.Run("malformed", func(t *testing.T) {
		note := &models.Note{
			Title: "Broken", Filename: "broken.md", RawRevision: "observed",
			ParseError: "invalid frontmatter",
		}
		service := &notesMoveService{combinedNotesService: combinedNotesService{
			byLocation: map[app.NoteLocation][]*models.Note{
				app.GlobalNoteLocation(): {note},
			},
		}}
		view := NewNotesViewModelWithService(
			t.TempDir(),
			AllNotesScope(),
			app.GlobalNoteLocation(),
			service,
			nil,
			project,
		)

		_, cmd := view.Update(syntheticKey('m'))
		if cmd != nil || view.locationPicker != nil || !view.statusIsError ||
			!strings.Contains(view.statusMessage, "press Enter to repair") {
			t.Fatalf("malformed move state = cmd %v picker %v status %q", cmd, view.locationPicker, view.statusMessage)
		}
	})

	t.Run("no other owner", func(t *testing.T) {
		note := &models.Note{Title: "Only", Filename: "only.md", RawRevision: "observed"}
		service := &notesMoveService{combinedNotesService: combinedNotesService{
			byLocation: map[app.NoteLocation][]*models.Note{
				app.GlobalNoteLocation(): {note},
			},
		}}
		view := NewNotesViewModelWithService(
			t.TempDir(),
			AllNotesScope(),
			app.GlobalNoteLocation(),
			service,
			nil,
		)

		_, cmd := view.Update(syntheticKey('m'))
		if cmd != nil || view.locationPicker != nil || view.statusIsError ||
			!strings.Contains(view.statusMessage, "note was not moved") ||
			view.currentRow() == nil {
			t.Fatalf("missing-destination move state = cmd %v picker %v status %q", cmd, view.locationPicker, view.statusMessage)
		}
	})
}

func TestNotesMoveResultSelectsCommittedDestinationIdentity(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, alpha, beta)
	_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{Scope: AllNotesScope()})
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
	view.noteService = service
	view.searchApplied = "needle"
	view.loadNotes()

	service.byLocation[source] = nil
	service.byLocation[destination] = []*models.Note{{
		Title: "Needle source", Filename: filename, Body: "needle", ModifiedAt: time.Unix(2, 0),
	}}
	shell.reconcileNoteResult(ProjectNoteMutationResultMsg{
		Location: source,
		Result: app.NoteBatchResult{Results: []app.NoteResult{{
			Kind:           app.NoteMove,
			Location:       destination,
			SourceLocation: source,
			ProjectID:      destination.ProjectID,
			Filename:       filename,
		}}},
		SuccessMessage: "Moved",
	})

	want := NoteRowIdentity{Location: destination, Filename: filename}
	if view.SearchQuery() != "needle" || len(view.visibleRows) != 2 ||
		view.currentRow() == nil || view.currentRow().Identity() != want {
		t.Fatalf(
			"move reconciliation = query %q rows %d current %+v, want destination %+v",
			view.SearchQuery(),
			len(view.visibleRows),
			view.currentRow(),
			want,
		)
	}
}

func TestNotesMoveSourceScopeReappliesSearchAndClampsAfterRowDisappears(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, alpha, beta)
	_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{
		Scope: ProjectNotesScope(alpha.ID),
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
	view.noteService = service
	view.searchApplied = "needle"
	view.loadNotes()

	service.byLocation[source] = nil
	service.byLocation[destination] = []*models.Note{{
		Title: "Needle source", Filename: filename, Body: "needle",
	}}
	shell.reconcileNoteResult(ProjectNoteMutationResultMsg{
		Location: source,
		Result: app.NoteBatchResult{Results: []app.NoteResult{{
			Kind:           app.NoteMove,
			Location:       destination,
			SourceLocation: source,
			ProjectID:      destination.ProjectID,
			Filename:       filename,
		}}},
		SuccessMessage: "Moved",
	})

	if view.SearchQuery() != "needle" || len(view.loadedRows) != 1 ||
		len(view.visibleRows) != 0 || view.cursor != 0 {
		t.Fatalf(
			"source-scope reconciliation = query %q loaded %d visible %d cursor %d",
			view.SearchQuery(),
			len(view.loadedRows),
			len(view.visibleRows),
			view.cursor,
		)
	}
}

func TestNotesMoveUndoRedoRefreshesEveryAffectedOwner(t *testing.T) {
	env := newGlobalNoteTUIEnv(t, "Undoable move")
	env.view.searchApplied = "undoable"
	env.view.rederiveVisibleRows()
	source := app.GlobalNoteLocation()
	destination := app.ProjectNoteLocation(env.project.ID)

	_, _ = env.view.Update(syntheticKey('m'))
	_, pickerCmd := env.view.Update(syntheticSpecialKey(tea.KeyEnter))
	pickerResult := pickerCmd().(notesLocationPickerResultMsg)
	_, mutationCmd := env.view.Update(pickerResult)
	mutation := runProjectNoteMutation(t, func() any { return mutationCmd() })
	env.shell.handleNotesMessages(mutation)
	if env.view.SearchQuery() != "undoable" || env.view.currentRow() == nil ||
		env.view.currentRow().Location != destination {
		t.Fatalf("committed move did not refresh destination: %#v", env.view.currentRow())
	}

	undo, ok := env.view.undoDelete()().(ProjectNoteLedgerResultMsg)
	if !ok || undo.Redo {
		t.Fatalf("move undo result = %#v", undo)
	}
	env.shell.handleNotesMessages(undo)
	if env.view.SearchQuery() != "undoable" || env.view.currentRow() == nil ||
		env.view.currentRow().Location != source {
		t.Fatalf("move undo did not refresh source: %#v", env.view.currentRow())
	}

	redo, ok := env.view.redoNoteMutation()().(ProjectNoteLedgerResultMsg)
	if !ok || !redo.Redo {
		t.Fatalf("move redo result = %#v", redo)
	}
	env.shell.handleNotesMessages(redo)
	if env.view.SearchQuery() != "undoable" || env.view.currentRow() == nil ||
		env.view.currentRow().Location != destination {
		t.Fatalf("move redo did not refresh destination: %#v", env.view.currentRow())
	}
}

func TestNotesMoveFailureKeepsVisibleOwnership(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	source := app.GlobalNoteLocation()
	note := &models.Note{Title: "Stable", Filename: "stable.md", RawRevision: "stale"}
	staleErr := errors.New("artifact revision conflict")
	service := &notesMoveService{
		combinedNotesService: combinedNotesService{
			byLocation: map[app.NoteLocation][]*models.Note{source: {note}},
		},
		applyErr: staleErr,
	}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		app.GlobalNoteLocation(),
		service,
		nil,
		project,
	)
	before := view.currentRow().Identity()

	_, _ = view.Update(syntheticKey('m'))
	_, resultCmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	result := resultCmd().(notesLocationPickerResultMsg)
	_, mutationCmd := view.Update(result)
	failure, ok := mutationCmd().(ProjectNoteMutationErrorMsg)
	if !ok || failure.Location != source || !errors.Is(failure.Err, staleErr) {
		t.Fatalf("move failure = %#v, want source-qualified stale error", failure)
	}
	if view.currentRow() == nil || view.currentRow().Identity() != before ||
		len(view.loadedRows) != 1 || view.loadedRows[0].Location != source {
		t.Fatalf("failed move changed visible ownership: rows %#v current %#v", view.loadedRows, view.currentRow())
	}
}

func TestNotesMoveKeymapAndHelpStayInParity(t *testing.T) {
	view := &NotesViewModel{}
	wantKeys := []string{
		"j", "k", "down", "up", "enter", "n", "v", "s", "r", "m", "x", "u", "ctrl+r",
	}
	if got := view.HandledKeys(); !slices.Equal(got, wantKeys) {
		t.Fatalf("Notes handled keys = %v, want %v", got, wantKeys)
	}
	moveHelp := 0
	for _, item := range view.GetHelpItems() {
		if item.Key == "m" {
			moveHelp++
		}
	}
	if moveHelp != 1 {
		t.Fatalf("Notes help contains %d move rows, want one: %#v", moveHelp, view.GetHelpItems())
	}
}

package notes

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/route"
)

type notesMoveService struct {
	combinedNotesService
	result   app.NoteBatchResult
	applyErr error
}

func (s *notesMoveService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, CloneNoteBatchRequest(request))
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
				route.AllNotesScope(),
				service,
				nil,
				alpha,
				beta,
			)
			mountNotesManager(t, view)

			_, _ = view.Update(syntheticKey('%'))
			if view.LocationPicker == nil || view.LocationPicker.Purpose != NotesLocationPickerMove {
				t.Fatal("% did not open the move destination picker")
			}
			var got []app.NoteLocation
			for _, item := range view.LocationPicker.SourceItems() {
				got = append(got, item.Location)
				if item.Scope.Kind == route.NotesScopeAll {
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
			Kind:      app.NoteMove,
			Location:  destination,
			ProjectID: destination.ProjectID,
			Filename:  note.Filename,
		}}},
	}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)

	_, _ = view.Update(syntheticKey('%'))
	view.LocationPicker.SetCursorToID(NotesLocationPickerItem{
		Scope: route.ProjectNotesScope(beta.ID),
	}.ItemID())
	note.Title = "live pointer changed"
	note.Body = "live body changed"
	note.RawRevision = "new-revision"

	_, resultCmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	if resultCmd == nil || view.Confirm.IsActive() {
		t.Fatal("move Enter did not select immediately")
	}
	result, ok := resultCmd().(NotesLocationPickerResultMsg)
	if !ok || !result.Selected || result.Item.Location != destination {
		t.Fatalf("move picker result = %#v, want destination %+v", result, destination)
	}
	_, mutationCmd := view.Update(result)
	if mutationCmd == nil || view.LocationPicker != nil {
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
			route.AllNotesScope(),
			service,
			nil,
			project,
		)
		mountNotesManager(t, view)

		_, cmd := view.Update(syntheticKey('%'))
		if cmd != nil || view.LocationPicker != nil || !view.StatusIsError ||
			!strings.Contains(view.StatusMessage, "press Enter to repair") {
			t.Fatalf("malformed move state = cmd %v picker %v status %q", cmd, view.LocationPicker, view.StatusMessage)
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
			route.AllNotesScope(),
			service,
			nil,
		)
		mountNotesManager(t, view)

		_, cmd := view.Update(syntheticKey('%'))
		if cmd != nil || view.LocationPicker != nil || view.StatusIsError ||
			!strings.Contains(view.StatusMessage, "note was not moved") ||
			view.CurrentRow() == nil {
			t.Fatalf("missing-destination move state = cmd %v picker %v status %q", cmd, view.LocationPicker, view.StatusMessage)
		}
	})
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
		route.AllNotesScope(),
		service,
		nil,
		project,
	)
	mountNotesManager(t, view)
	before := view.CurrentRow().Identity()

	_, _ = view.Update(syntheticKey('%'))
	_, resultCmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	result := resultCmd().(NotesLocationPickerResultMsg)
	_, mutationCmd := view.Update(result)
	failure, ok := mutationCmd().(ProjectNoteMutationErrorMsg)
	if !ok || failure.Location != source || !errors.Is(failure.Err, staleErr) {
		t.Fatalf("move failure = %#v, want source-qualified stale error", failure)
	}
	if view.CurrentRow() == nil || view.CurrentRow().Identity() != before ||
		len(view.LoadedRows) != 1 || view.LoadedRows[0].Location != source {
		t.Fatalf("failed move changed visible ownership: rows %#v current %#v", view.LoadedRows, view.CurrentRow())
	}
}

// Move is described by both the registry and the keymap, so the merged help
// must keep the registry's row and drop the keymap's duplicate.
func TestNotesMoveKeymapAndHelpStayInParity(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {{Title: "Move me", Filename: "move.md"}},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)

	if !slices.Contains(view.HandledKeys(), "%") {
		t.Fatalf("Notes handled keys omit move: %v", view.HandledKeys())
	}
	moveHelp := 0
	for _, item := range view.GetHelpItems() {
		if item.Key == "%" {
			moveHelp++
		}
	}
	if moveHelp != 1 {
		t.Fatalf("Notes help contains %d move rows, want one: %#v", moveHelp, view.GetHelpItems())
	}
}

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

type combinedNotesService struct {
	byLocation map[app.NoteLocation][]*models.Note
	requests   []app.NoteBatchRequest
}

func (s *combinedNotesService) NotesAt(location app.NoteLocation) ([]*models.Note, error) {
	return append([]*models.Note(nil), s.byLocation[location]...), nil
}

func (s *combinedNotesService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, request)
	return app.NoteBatchResult{}, nil
}

func TestCombinedNotesLoadsGlobalAndActiveProjectWithLocations(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	global := &models.Note{Title: "Global note", Filename: "global.md", ModifiedAt: time.Unix(2, 0)}
	local := &models.Note{Title: "Project note", Filename: "project.md", ModifiedAt: time.Unix(1, 0)}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation():         {global},
		app.ProjectNoteLocation("alpha"): {local},
	}}

	view := NewCombinedNotesViewModelWithService(
		t.TempDir(),
		project,
		app.GlobalNoteLocation(),
		service,
		nil,
	)
	view.SetSize(80, 24)

	if len(view.notes) != 2 {
		t.Fatalf("notes = %d, want global plus active project", len(view.notes))
	}
	if got := view.noteLocationFor(global); got != app.GlobalNoteLocation() {
		t.Fatalf("global location = %+v", got)
	}
	if got := view.noteLocationFor(local); got != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("project location = %+v", got)
	}
	rendered := view.View().Content
	for _, want := range []string{"Notes", "Global", "Alpha", "Global note", "Project note"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("combined Notes view missing %q:\n%s", want, rendered)
		}
	}
}

func TestCombinedNotesCreationRequiresLocationConfirmation(t *testing.T) {
	t.Setenv("TT_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	view := NewCombinedNotesViewModelWithService(
		t.TempDir(),
		project,
		app.ProjectNoteLocation(project.ID),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
	)

	_, cmd := view.Update(syntheticKey('n'))
	if cmd != nil {
		t.Fatal("n started creation before location confirmation")
	}
	if !view.destinationPicker {
		t.Fatal("n did not open the destination picker")
	}
	if !view.ShouldCaptureEsc() {
		t.Fatal("destination picker did not retain Esc for explicit cancellation")
	}
	if got := view.destinations[view.destinationCursor]; got != app.ProjectNoteLocation(project.ID) {
		t.Fatalf("preselected destination = %+v, want project", got)
	}

	_, cmd = view.Update(syntheticSpecialKey(tea.KeyEnter))
	if view.destinationPicker {
		t.Fatal("Enter did not confirm the destination")
	}
	if cmd == nil {
		t.Fatal("confirmed destination did not start the external editor")
	}
}

func TestCombinedNotesCreationOffersEveryActiveProject(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	view := NewCombinedNotesViewModelWithService(
		t.TempDir(),
		nil,
		app.GlobalNoteLocation(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
		alpha,
		beta,
	)
	view.SetSize(80, 24)

	_, cmd := view.Update(syntheticKey('n'))
	if cmd != nil {
		t.Fatal("n started creation before location confirmation")
	}
	want := []app.NoteLocation{
		app.GlobalNoteLocation(),
		app.ProjectNoteLocation(alpha.ID),
		app.ProjectNoteLocation(beta.ID),
	}
	if !slices.Equal(view.destinations, want) {
		t.Fatalf("destinations = %+v, want %+v", view.destinations, want)
	}
	rendered := view.View().Content
	for _, label := range []string{"Global", alpha.Name, beta.Name} {
		if !strings.Contains(rendered, label) {
			t.Fatalf("destination picker missing %q:\n%s", label, rendered)
		}
	}
}

func TestResolveNoteEditorEnvironmentPrecedenceAndActionableFailure(t *testing.T) {
	t.Setenv("TT_EDITOR", "true --tt")
	t.Setenv("VISUAL", "sh --visual")
	t.Setenv("EDITOR", "env --editor")
	editor, err := external.ResolveNoteEditor()
	if err != nil {
		t.Fatal(err)
	}
	if editor != "true --tt" {
		t.Fatalf("editor = %q, want TT_EDITOR", editor)
	}

	t.Setenv("TT_EDITOR", "")
	editor, err = external.ResolveNoteEditor()
	if err != nil {
		t.Fatal(err)
	}
	if editor != "sh --visual" {
		t.Fatalf("editor = %q, want VISUAL", editor)
	}

	t.Setenv("VISUAL", "")
	editor, err = external.ResolveNoteEditor()
	if err != nil {
		t.Fatal(err)
	}
	if editor != "env --editor" {
		t.Fatalf("editor = %q, want EDITOR", editor)
	}

	t.Setenv("EDITOR", "")
	_, err = external.ResolveNoteEditor()
	if err == nil || !strings.Contains(err.Error(), "set TT_EDITOR, VISUAL, or EDITOR") {
		t.Fatalf("missing-editor error = %v, want actionable environment guidance", err)
	}
}

func TestCombinedNotesExposesOnlyMinimalOperations(t *testing.T) {
	descriptors := noteOperationDescriptors()
	var ids []OperationID
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	want := []OperationID{operationNoteOpen, operationNoteRename, operationNoteDelete}
	if !slices.Equal(ids, want) {
		t.Fatalf("note operations = %v, want %v", ids, want)
	}

	view := &NotesViewModel{combined: true}
	keys := view.HandledKeys()
	for _, removed := range []string{"e", "/", "p", "t", "m", "space", "ctrl+o"} {
		if slices.Contains(keys, removed) {
			t.Fatalf("combined Notes still exposes removed key %q in %v", removed, keys)
		}
	}
	model, cmd := view.Update(syntheticKey('m'))
	if model != view || cmd != nil {
		t.Fatalf("m resolved in Notes view: model=%T cmd=%v", model, cmd)
	}
}

package notes

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/route"
)

func TestNotesInteractionLayersFitMinimumLayoutWithLargeOwnerCatalog(t *testing.T) {
	const (
		width  = 60
		height = 20
	)
	projects := make([]*models.Project, 0, 160)
	byLocation := map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {{
			Title:       "Duplicate title with a very long suffix for layout coverage",
			Body:        "needle",
			Filename:    "duplicate.md",
			ModifiedAt:  time.Unix(2, 0),
			RawRevision: "global-revision",
		}},
	}
	collectionErrors := make(map[app.NoteLocation]error)
	for i := range 160 {
		project := &models.Project{
			ID:   fmt.Sprintf("project-%03d", i),
			Name: fmt.Sprintf("Project %03d with an exceptionally long owner label", i),
			Path: t.TempDir(),
		}
		projects = append(projects, project)
		location := app.ProjectNoteLocation(project.ID)
		switch {
		case i < 2:
			byLocation[location] = []*models.Note{{
				Title:       "Duplicate title with a very long suffix for layout coverage",
				Body:        "needle",
				Filename:    "duplicate.md",
				ModifiedAt:  time.Unix(int64(4-i), 0),
				RawRevision: fmt.Sprintf("revision-%d", i),
			}}
		case i%7 == 0:
			collectionErrors[location] = fmt.Errorf(
				"permission denied while reading a deliberately long collection path for owner %03d",
				i,
			)
		}
	}
	service := &notesMoveService{combinedNotesService: combinedNotesService{
		byLocation: byLocation,
		errors:     collectionErrors,
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		projects...,
	)
	mountNotesManager(t, view)
	view.SetSize(width, height)
	view.SetErrorStatus(strings.Repeat("actionable visible error summary ", 8))

	assertNotesLayoutFits(t, "flat list", view)
	if len(view.VisibleRows) != 3 {
		t.Fatalf("duplicate located rows = %d, want three", len(view.VisibleRows))
	}

	_, _ = view.Update(syntheticKey('/'))
	for _, r := range "needleae%p" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.LocationPicker != nil || view.renaming || view.Confirm.IsActive() {
		t.Fatal("search input leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "persistent search", view)
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	_, _ = view.Update(syntheticKey('p'))
	for _, r := range "project 159 ae%/" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.LocationPicker == nil || view.LocationPicker.Purpose != NotesLocationPickerScope ||
		view.searchEditing || view.renaming || view.Confirm.IsActive() {
		t.Fatal("scope picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "scope picker", view)
	cancelNotesPicker(t, view)

	_, _ = view.Update(syntheticKey('a'))
	for _, r := range "project 159 e%/p" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.LocationPicker == nil || view.LocationPicker.Purpose != NotesLocationPickerCreation ||
		view.searchEditing || view.renaming || view.Confirm.IsActive() {
		t.Fatal("creation picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "creation picker", view)
	cancelNotesPicker(t, view)

	_, _ = view.Update(syntheticKey('%'))
	for _, r := range "project 159 ae/p" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.LocationPicker == nil || view.LocationPicker.Purpose != NotesLocationPickerMove ||
		view.searchEditing || view.renaming || view.Confirm.IsActive() {
		t.Fatal("move picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "move picker", view)
	cancelNotesPicker(t, view)
}

func assertNotesLayoutFits(t *testing.T, state string, view *NotesViewModel) {
	t.Helper()
	content, auditIssues := withRenderAudit(func() string {
		return view.View().Content
	})
	_, issues := normalizeScreen(content, 60, 20)
	issues = append(issues, auditIssues...)
	if len(issues) != 0 {
		t.Fatalf("%s layout issues = %#v\n%s", state, issues, content)
	}
}

func cancelNotesPicker(t *testing.T, view *NotesViewModel) {
	t.Helper()
	_, cmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("picker Esc emitted no cancellation result")
	}
	result, ok := cmd().(NotesLocationPickerResultMsg)
	if !ok || result.Selected {
		t.Fatalf("picker cancellation result = %#v", result)
	}
	_, _ = view.Update(result)
	if view.LocationPicker != nil {
		t.Fatal("picker remained open after cancellation")
	}
}

func TestNotesHelpDocumentsTheWorkspaceManager(t *testing.T) {
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation(): {{Title: "Needle", Filename: "needle.md"}},
	}}
	view := NewNotesViewModelWithService(t.TempDir(), route.AllNotesScope(), service, nil, project)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	// The registry names every operation the Notes grammar binds; the keymap
	// supplies only the cursor, end, and ledger keys whose core descriptors answer
	// to the Outliner. Delete is absent because the Space sheet is its one path,
	// and the two end rows carry the strategy's own spellings rather than the
	// single physical key each keymap entry binds.
	want := []operation.HelpItem{
		{Key: "Enter", Description: "Open Note", Category: "Notes"},
		{Key: "a", Description: "New Note", Category: "Notes"},
		{Key: "e", Description: "Rename Note", Category: "Notes"},
		{Key: "%", Description: "Move Note", Category: "Notes"},
		{Key: "/", Description: "Search Notes", Category: "Discovery"},
		{Key: "p", Description: "Choose Notes Scope", Category: "Notes"},
		{Key: "y", Description: "Copy Note Title", Category: "Notes"},
		{Key: "Y", Description: "Copy Note Path", Category: "Notes"},
		{Key: "Space", Description: "Note Actions", Category: "Notes"},
		{Key: "j", Description: "Move cursor down", Category: "Navigation"},
		{Key: "k", Description: "Move cursor up", Category: "Navigation"},
		// The preview pane is on screen at 80 columns with a note under the cursor,
		// so its paging row is gated in; Ctrl+B is hidden because the one row names
		// both presses.
		{Key: "Ctrl+F/B", Description: "Page note preview", Category: "Navigation"},
		{Key: "gg/Home", Description: "Jump to first note", Category: "Navigation"},
		{Key: "G/End", Description: "Jump to last note", Category: "Navigation"},
		{Key: "u", Description: "Undo workspace change", Category: "Actions"},
		{Key: "ctrl+r", Description: "Redo workspace change", Category: "Actions"},
	}
	if got := view.GetHelpItems(); !reflect.DeepEqual(got, want) {
		t.Fatalf("Notes help = %#v, want %#v", got, want)
	}
}

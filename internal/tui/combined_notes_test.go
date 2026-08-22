package tui

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

type combinedNotesService struct {
	byLocation  map[app.NoteLocation][]*models.Note
	errors      map[app.NoteLocation]error
	snapshotErr error
	queryCount  int
	requests    []app.NoteBatchRequest
}

func (s *combinedNotesService) NotesAtLocations(locations []app.NoteLocation) ([]app.NoteCollectionResult, error) {
	s.queryCount++
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	results := make([]app.NoteCollectionResult, len(locations))
	for i, location := range locations {
		results[i] = app.NoteCollectionResult{
			Location: location,
			Notes:    append([]*models.Note(nil), s.byLocation[location]...),
			Err:      s.errors[location],
		}
	}
	return results, nil
}

func (s *combinedNotesService) ApplyNoteBatch(request app.NoteBatchRequest) (app.NoteBatchResult, error) {
	s.requests = append(s.requests, request)
	return app.NoteBatchResult{}, nil
}

func TestNotesManagerLoadsEveryOwnerWithStableLocatedOrdering(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	global := &models.Note{Title: "Newest", Filename: "duplicate.md", ModifiedAt: time.Unix(2, 0)}
	alphaFirst := &models.Note{Title: "First", Filename: "duplicate.md", ModifiedAt: time.Unix(1, 0)}
	alphaSecond := &models.Note{Title: "First", Filename: "z.md", ModifiedAt: time.Unix(1, 0)}
	betaNote := &models.Note{Title: "Beta note", Filename: "beta.md", ModifiedAt: time.Unix(1, 0)}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation():         {global},
		app.ProjectNoteLocation("alpha"): {alphaSecond, alphaFirst},
		app.ProjectNoteLocation("beta"):  {betaNote},
	}}

	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	if service.queryCount != 1 {
		t.Fatalf("bulk queries = %d, want one", service.queryCount)
	}
	if len(view.loadedRows) != 4 || len(view.visibleRows) != 4 {
		t.Fatalf("rows = loaded %d visible %d, want four", len(view.loadedRows), len(view.visibleRows))
	}
	wantIDs := []NoteRowIdentity{
		{Location: app.GlobalNoteLocation(), Filename: "duplicate.md"},
		{Location: app.ProjectNoteLocation("alpha"), Filename: "duplicate.md"},
		{Location: app.ProjectNoteLocation("alpha"), Filename: "z.md"},
		{Location: app.ProjectNoteLocation("beta"), Filename: "beta.md"},
	}
	gotIDs := make([]NoteRowIdentity, len(view.visibleRows))
	for i, row := range view.visibleRows {
		gotIDs[i] = row.Identity()
	}
	if !slices.Equal(gotIDs, wantIDs) {
		t.Fatalf("located ordering = %+v, want %+v", gotIDs, wantIDs)
	}
	rendered := view.View().Content
	for _, want := range []string{"Notes", "All Notes", "Global", "Alpha", "Beta", "Newest", "First", "Beta note"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("Notes manager missing %q:\n%s", want, rendered)
		}
	}
}

func TestNotesManagerRetainsPartialResultsAndOwnerErrors(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	betaErr := errors.New("permission denied")
	service := &combinedNotesService{
		byLocation: map[app.NoteLocation][]*models.Note{
			app.GlobalNoteLocation(): {{Title: "Available", Filename: "available.md"}},
		},
		errors: map[app.NoteLocation]error{
			app.ProjectNoteLocation(beta.ID): betaErr,
		},
	}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	if len(view.visibleRows) != 1 || len(view.collectionErrors) != 1 || view.snapshotError != nil {
		t.Fatalf("partial load = rows %d errors %d snapshot %v", len(view.visibleRows), len(view.collectionErrors), view.snapshotError)
	}
	rendered := view.View().Content
	for _, want := range []string{"Available", "Beta", betaErr.Error()} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("partial result missing %q:\n%s", want, rendered)
		}
	}
}

func TestNotesManagerScopeDerivationAndRefreshPreserveLocatedSelection(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	global := &models.Note{Title: "Same", Filename: "duplicate.md", ModifiedAt: time.Unix(2, 0)}
	project := &models.Note{Title: "Same", Filename: "duplicate.md", ModifiedAt: time.Unix(1, 0)}
	service := &combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
		app.GlobalNoteLocation():          {global},
		app.ProjectNoteLocation(alpha.ID): {project},
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		service,
		nil,
		alpha,
	)
	mountNotesManager(t, view)
	projectID := NoteRowIdentity{Location: app.ProjectNoteLocation(alpha.ID), Filename: "duplicate.md"}
	for i := range view.visibleRows {
		if view.visibleRows[i].Identity() == projectID {
			view.cursor = i
		}
	}

	service.byLocation[app.GlobalNoteLocation()][0].ModifiedAt = time.Unix(0, 0)
	service.byLocation[app.ProjectNoteLocation(alpha.ID)][0].ModifiedAt = time.Unix(3, 0)
	view.loadNotes()
	if row := view.currentRow(); row == nil || row.Identity() != projectID {
		t.Fatalf("refresh selection = %#v, want project duplicate", row)
	}

	view.SetScope(GlobalNotesScope())
	if len(view.visibleRows) != 1 || view.visibleRows[0].Location != app.GlobalNoteLocation() {
		t.Fatalf("global rows = %#v", view.visibleRows)
	}
	view.SetScope(ProjectNotesScope(alpha.ID))
	if len(view.visibleRows) != 1 || view.visibleRows[0].Identity() != projectID {
		t.Fatalf("project rows = %#v", view.visibleRows)
	}
	if service.queryCount != 2 {
		t.Fatalf("scope derivation queries = %d, want only the two explicit snapshot loads", service.queryCount)
	}
}

func TestNotesManagerTotalSnapshotFailureBlocksRows(t *testing.T) {
	snapshotErr := errors.New("snapshot failed")
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		&combinedNotesService{snapshotErr: snapshotErr},
		nil,
	)
	mountNotesManager(t, view)
	view.SetSize(80, 24)
	if len(view.loadedRows) != 0 || len(view.visibleRows) != 0 || !errors.Is(view.snapshotError, snapshotErr) {
		t.Fatalf("total failure = rows %d/%d error %v", len(view.loadedRows), len(view.visibleRows), view.snapshotError)
	}
	rendered := view.View().Content
	if !strings.Contains(rendered, "Workspace") || !strings.Contains(rendered, snapshotErr.Error()) {
		t.Fatalf("blocking snapshot error not rendered:\n%s", rendered)
	}
}

func TestNotesManagerMalformedEditorUsesSelectedRowsActualOwner(t *testing.T) {
	t.Setenv("TT_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: filepath.Join(t.TempDir(), "alpha")}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: filepath.Join(t.TempDir(), "beta")}
	filename := "broken.md"
	raw := []byte("missing frontmatter\n")
	if err := os.MkdirAll(storage.NotesDir(beta), storage.DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(storage.NotesDir(beta), filename), raw, storage.FilePermissions); err != nil {
		t.Fatal(err)
	}
	malformed := &models.Note{Title: "Broken", Filename: filename, ParseError: "missing frontmatter"}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{
			app.ProjectNoteLocation(beta.ID): {malformed},
		}},
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)
	for i := range view.visibleRows {
		if view.visibleRows[i].Identity() == (NoteRowIdentity{Location: app.ProjectNoteLocation(beta.ID), Filename: filename}) {
			view.cursor = i
			break
		}
	}
	row := view.currentRow()
	if row == nil || view.effectiveDir(row) != storage.NotesDir(beta) {
		t.Fatalf("malformed row directory = %q, want selected owner %q", view.effectiveDir(row), storage.NotesDir(beta))
	}
	msg := view.editCurrentNote()()
	if failure, failed := msg.(ErrorMsg); failed {
		t.Fatalf("malformed editor could not read beta-owned raw content: %v", failure.Err)
	}
}

func TestNotesManagerCreationRequiresLocationConfirmation(t *testing.T) {
	t.Setenv("TT_EDITOR", "true")
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	project := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
		project,
	)
	mountNotesManager(t, view)

	_, _ = view.Update(syntheticKey('a'))
	if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerCreation {
		t.Fatal("a did not open the destination picker")
	}
	if !view.ShouldCaptureEsc() {
		t.Fatal("destination picker did not retain Esc for explicit cancellation")
	}
	if !view.HasTextInput() {
		t.Fatal("destination picker did not focus its search input")
	}
	if got := view.locationPicker.SelectedItem().Location; got != app.GlobalNoteLocation() {
		t.Fatalf("All Notes preselected destination = %+v, want Global", got)
	}

	_, cmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter did not emit a destination result")
	}
	result, ok := cmd().(notesLocationPickerResultMsg)
	if !ok || !result.Selected {
		t.Fatalf("destination result = %#v, want selected location", result)
	}
	_, cmd = view.Update(result)
	if view.locationPicker != nil {
		t.Fatal("Enter did not confirm the destination")
	}
	if cmd == nil {
		t.Fatal("confirmed destination did not start the external editor")
	}
}

func TestNotesManagerCreationOffersEveryActiveProject(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	_, _ = view.Update(syntheticKey('a'))
	want := []app.NoteLocation{
		app.GlobalNoteLocation(),
		app.ProjectNoteLocation(alpha.ID),
		app.ProjectNoteLocation(beta.ID),
	}
	var got []app.NoteLocation
	for _, item := range view.locationPicker.SourceItems() {
		got = append(got, item.Location)
		if item.Scope.Kind == NotesScopeAll {
			t.Fatal("creation picker offered All Notes as a destination")
		}
	}
	if !slices.Equal(got, want) {
		t.Fatalf("destinations = %+v, want %+v", got, want)
	}
	rendered := view.View().Content
	for _, label := range []string{"Global", alpha.Name, beta.Name} {
		if !strings.Contains(rendered, label) {
			t.Fatalf("destination picker missing %q:\n%s", label, rendered)
		}
	}
}

func TestNotesScopePickerFiltersNavigatesAndCancelsWithoutChangingScope(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha Roadmap", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta Launch", Path: t.TempDir()}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		GlobalNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view)
	view.SetSize(80, 24)

	_, _ = view.Update(syntheticKey('p'))
	if view.locationPicker == nil || !view.HasTextInput() || !view.ShouldCaptureEsc() {
		t.Fatal("p did not open a focused scope picker that owns Esc")
	}
	if got := view.locationPicker.SelectedItem().Scope; got != GlobalNotesScope() {
		t.Fatalf("initial scope picker selection = %#v, want Global", got)
	}
	_, _ = view.Update(syntheticSpecialKey(tea.KeyDown))
	if got := view.locationPicker.SelectedItem().Scope; got != ProjectNotesScope(alpha.ID) {
		t.Fatalf("Down selection = %#v, want Alpha", got)
	}
	_, _ = view.Update(syntheticCtrlKey('j'))
	if got := view.locationPicker.SelectedItem().Scope; got != ProjectNotesScope(beta.ID) {
		t.Fatalf("Ctrl+J selection = %#v, want Beta", got)
	}
	_, _ = view.Update(syntheticCtrlKey('k'))
	if got := view.locationPicker.SelectedItem().Scope; got != ProjectNotesScope(alpha.ID) {
		t.Fatalf("Ctrl+K selection = %#v, want Alpha", got)
	}
	for _, r := range "launch" {
		_, _ = view.Update(syntheticKey(r))
	}
	filtered := view.locationPicker.FilteredItems()
	if len(filtered) != 1 || filtered[0].Scope != ProjectNotesScope(beta.ID) {
		t.Fatalf("filtered scope locations = %#v, want Beta", filtered)
	}

	_, cmd := view.Update(syntheticSpecialKey(tea.KeyEsc))
	if cmd == nil {
		t.Fatal("Esc emitted no picker cancellation")
	}
	result, ok := cmd().(notesLocationPickerResultMsg)
	if !ok || result.Selected || result.Purpose != notesLocationPickerScope {
		t.Fatalf("scope cancel result = %#v", result)
	}
	_, _ = view.Update(result)
	if view.locationPicker != nil || view.scope != GlobalNotesScope() {
		t.Fatalf("scope cancel changed manager state: picker=%v scope=%#v", view.locationPicker, view.scope)
	}
}

func TestNotesScopePickerEmptyResultsAreSafeAndResize(t *testing.T) {
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		&combinedNotesService{byLocation: map[app.NoteLocation][]*models.Note{}},
		nil,
	)
	mountNotesManager(t, view)
	_, _ = view.Update(syntheticKey('p'))
	for _, r := range "nothing-matches" {
		_, _ = view.Update(syntheticKey(r))
	}
	if len(view.locationPicker.FilteredItems()) != 0 {
		t.Fatalf("scope picker unexpectedly matched %#v", view.locationPicker.FilteredItems())
	}
	_, cmd := view.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd != nil || view.locationPicker == nil {
		t.Fatal("Enter on empty results closed the scope picker")
	}

	view.SetSize(68, 20)
	rendered := view.View().Content
	if view.locationPicker.Width() != 68 || view.locationPicker.Height() != 20 ||
		!strings.Contains(rendered, "No matching note locations") {
		t.Fatalf("resized empty picker = %dx%d:\n%s", view.locationPicker.Width(), view.locationPicker.Height(), rendered)
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

func TestNotesManagerRestoresOnlyMoveToTheMinimalOperations(t *testing.T) {
	descriptors := noteOperationDescriptors()
	var ids []OperationID
	for _, descriptor := range descriptors {
		ids = append(ids, descriptor.ID)
	}
	want := []OperationID{
		operationNoteOpen,
		operationNoteRename,
		operationNoteMove,
		operationNoteCopyTitle,
		operationNoteCopyPath,
		operationNoteDelete,
	}
	if !slices.Equal(ids, want) {
		t.Fatalf("note operations = %v, want %v", ids, want)
	}

	view := &NotesViewModel{}
	keys := view.HandledKeys()
	if !slices.Contains(keys, "%") {
		t.Fatalf("Notes manager keys omit restored move key: %v", keys)
	}
	// Delete is destructive, so it reaches the user only through the Space sheet;
	// tags and previews were never restored at all.
	for _, removed := range []string{"x", "t", "ctrl+o"} {
		if slices.Contains(keys, removed) {
			t.Fatalf("Notes manager still exposes removed key %q in %v", removed, keys)
		}
	}
}

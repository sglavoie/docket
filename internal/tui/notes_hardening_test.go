package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
)

func TestNotesWorkspaceRefreshRebuildsOwnersAndRejectsStalePickerResults(t *testing.T) {
	alpha := &models.Project{ID: "alpha", Name: "Alpha", Path: t.TempDir()}
	beta := &models.Project{ID: "beta", Name: "Beta", Path: t.TempDir()}
	global := &models.Note{
		Title: "Needle", Filename: "needle.md", Body: "needle",
		ModifiedAt: time.Unix(2, 0), RawRevision: "observed",
	}
	service := &notesMoveService{combinedNotesService: combinedNotesService{
		byLocation: map[app.NoteLocation][]*models.Note{
			app.GlobalNoteLocation():         {global},
			app.ProjectNoteLocation(beta.ID): {{Title: "Beta note", Filename: "beta.md"}},
		},
	}}
	view := NewNotesViewModelWithService(
		t.TempDir(),
		AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	view.SetSize(80, 24)
	view.searchApplied = "needle"
	view.rederiveVisibleRows()

	_, _ = view.Update(syntheticKey('m'))
	betaItem := notesLocationPickerItem{
		Label:    beta.Name,
		Scope:    ProjectNotesScope(beta.ID),
		Location: app.ProjectNoteLocation(beta.ID),
	}
	view.locationPicker.SetCursorToID(betaItem.ItemID())
	for _, r := range "beta" {
		_, _ = view.Update(syntheticKey(r))
	}

	renamedAlpha := &models.Project{
		ID: alpha.ID, Name: "Alpha Renamed", Path: alpha.Path,
	}
	if invalid := view.RefreshWorkspace(renamedAlpha); invalid {
		t.Fatal("All Notes was incorrectly reported as an invalid scope")
	}
	if view.SearchQuery() != "needle" {
		t.Fatalf("workspace refresh lost accepted search %q", view.SearchQuery())
	}
	if len(view.loadedRows) != 1 || view.loadedRows[0].Location != app.GlobalNoteLocation() {
		t.Fatalf("removed owner survived canonical rows: %#v", view.loadedRows)
	}
	if _, ok := view.owners[app.ProjectNoteLocation(beta.ID)]; ok {
		t.Fatal("removed Beta owner survived workspace refresh")
	}
	if owner := view.ownerForLocation(app.ProjectNoteLocation(alpha.ID)); owner.Label != renamedAlpha.Name {
		t.Fatalf("renamed owner label = %q, want %q", owner.Label, renamedAlpha.Name)
	}
	if view.locationPicker == nil || view.locationPicker.SearchQuery() != "beta" ||
		len(view.locationPicker.FilteredItems()) != 0 {
		t.Fatalf(
			"refreshed move picker = picker %v query %q matches %#v",
			view.locationPicker,
			view.locationPicker.SearchQuery(),
			view.locationPicker.FilteredItems(),
		)
	}

	_, cmd := view.Update(notesLocationPickerResultMsg{
		Purpose:  notesLocationPickerMove,
		Item:     betaItem,
		Selected: true,
	})
	if cmd != nil || view.locationPicker != nil || !view.statusIsError ||
		!strings.Contains(view.statusMessage, "no longer available") {
		t.Fatalf(
			"stale picker result = cmd %v picker %v error %t status %q",
			cmd,
			view.locationPicker,
			view.statusIsError,
			view.statusMessage,
		)
	}
	if len(service.requests) != 0 {
		t.Fatalf("stale picker result emitted mutations: %#v", service.requests)
	}
}

func TestNotesWorkspaceRefreshNormalizesInvalidScopeAndTypedRouteImmediately(t *testing.T) {
	tests := []struct {
		name     string
		projects func(*models.Project) []*models.Project
	}{
		{
			name: "removed",
			projects: func(*models.Project) []*models.Project {
				return nil
			},
		},
		{
			name: "archived",
			projects: func(project *models.Project) []*models.Project {
				archived := *project
				archived.Archived = true
				return []*models.Project{&archived}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			shell := typedTestApp(t)
			project := shell.currentProject
			_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{
				Scope: ProjectNotesScope(project.ID),
			})
			notes := shell.getActiveNotesView()
			notes.searchApplied = "needle"
			notes.rederiveVisibleRows()
			_, _ = notes.Update(syntheticKey('n'))

			setTestAppProjects(shell, tc.projects(project)...)
			handled, _ := shell.handleGlobalRefreshMessage(NotesRefreshMsg{})
			if !handled {
				t.Fatal("Notes workspace refresh was not handled")
			}

			params := shell.CurrentRoute().Params().(ManageParams)
			notes = shell.getActiveNotesView()
			if params.NotesScope != AllNotesScope() || notes.scope != AllNotesScope() {
				t.Fatalf("invalid scope survived refresh: route %#v model %#v", params.NotesScope, notes.scope)
			}
			if notes.locationPicker != nil {
				t.Fatal("invalid project creation destination survived route normalization")
			}
			if notes.SearchQuery() != "needle" {
				t.Fatalf("scope fallback lost accepted search %q", notes.SearchQuery())
			}
			if len(notes.owners) != 1 || !notes.locationAvailable(app.GlobalNoteLocation()) {
				t.Fatalf("scope fallback owners = %#v, want Global only", notes.owners)
			}
			if shell.currentProject != nil {
				t.Fatalf("invalid current project survived refresh: %#v", shell.currentProject)
			}
			if !shell.statusIsInfo || !strings.Contains(shell.statusMessage, "showing All Notes") ||
				!strings.Contains(notes.statusMessage, "showing All Notes") {
				t.Fatalf(
					"scope fallback status = shell %q info=%t notes %q",
					shell.statusMessage,
					shell.statusIsInfo,
					notes.statusMessage,
				)
			}
		})
	}
}

func TestNotesWorkspaceRefreshUpdatesOpenPickerLabelsAndQuery(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	beta := &models.Project{ID: "beta", Name: "Old Destination", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, alpha, beta)
	_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{Scope: AllNotesScope()})
	notes := shell.getActiveNotesView()
	_, _ = notes.Update(syntheticKey('n'))
	for _, r := range "renamed" {
		_, _ = notes.Update(syntheticKey(r))
	}
	if len(notes.locationPicker.FilteredItems()) != 0 {
		t.Fatalf("pre-rename picker unexpectedly matched %#v", notes.locationPicker.FilteredItems())
	}

	renamed := *beta
	renamed.Name = "Renamed Destination"
	setTestAppProjects(shell, alpha, &renamed)
	handled, _ := shell.handleGlobalRefreshMessage(NotesRefreshMsg{})
	if !handled {
		t.Fatal("Notes workspace refresh was not handled")
	}

	notes = shell.getActiveNotesView()
	filtered := notes.locationPicker.FilteredItems()
	if notes.locationPicker.SearchQuery() != "renamed" ||
		len(filtered) != 1 || filtered[0].Location != app.ProjectNoteLocation(beta.ID) ||
		filtered[0].Label != renamed.Name {
		t.Fatalf(
			"renamed picker state = query %q filtered %#v",
			notes.locationPicker.SearchQuery(),
			filtered,
		)
	}
	if owner := notes.ownerForLocation(app.ProjectNoteLocation(beta.ID)); owner.Label != renamed.Name {
		t.Fatalf("renamed owner label = %q, want %q", owner.Label, renamed.Name)
	}
}

func TestNotesManagerReopenRebuildsActiveOwnerCatalog(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{Scope: AllNotesScope()})
	notes := shell.getActiveNotesView()

	renamedAlpha := *alpha
	renamedAlpha.Name = "Alpha Renamed"
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, &renamedAlpha, beta)
	_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{Scope: AllNotesScope()})

	if shell.getActiveNotesView() != notes {
		t.Fatal("reopening Notes rebuilt the manager instead of refreshing it")
	}
	if owner := notes.ownerForLocation(app.ProjectNoteLocation(alpha.ID)); owner.Label != renamedAlpha.Name {
		t.Fatalf("reopened Alpha label = %q, want %q", owner.Label, renamedAlpha.Name)
	}
	if !notes.locationAvailable(app.ProjectNoteLocation(beta.ID)) {
		t.Fatal("reopening Notes did not add the new active owner")
	}
}

func TestNotesMutationRefreshPreservesProjectScopeAndAcceptedSearch(t *testing.T) {
	kinds := []app.NoteMutationKind{
		app.NoteCreate,
		app.NoteUpdate,
		app.NoteRename,
		app.NoteDelete,
		app.NoteMove,
	}
	for _, kind := range kinds {
		t.Run(string(kind), func(t *testing.T) {
			shell := typedTestApp(t)
			project := shell.currentProject
			location := app.ProjectNoteLocation(project.ID)
			service := &notesMoveService{combinedNotesService: combinedNotesService{
				byLocation: map[app.NoteLocation][]*models.Note{
					location: {{
						Title: "Needle before", Filename: "before.md", Body: "needle",
					}},
				},
			}}
			_, _ = shell.handleNotesMessages(ShowNotesManagerMsg{
				Scope: ProjectNotesScope(project.ID),
			})
			notes := shell.getActiveNotesView()
			notes.noteService = service
			notes.searchApplied = "needle"
			notes.loadNotes()

			service.byLocation[location] = []*models.Note{{
				Title: "Needle after", Filename: "after.md", Body: "needle",
			}}
			shell.reconcileNoteResult(ProjectNoteMutationResultMsg{
				Location: location,
				Result: app.NoteBatchResult{Results: []app.NoteResult{{
					Kind:             kind,
					Location:         location,
					ProjectID:        project.ID,
					Filename:         "after.md",
					PreviousFilename: "before.md",
				}}},
				SuccessMessage: "Changed",
			})

			if notes.scope != ProjectNotesScope(project.ID) ||
				notes.SearchQuery() != "needle" ||
				len(notes.visibleRows) != 1 ||
				notes.visibleRows[0].Identity() != (NoteRowIdentity{
					Location: location,
					Filename: "after.md",
				}) {
				t.Fatalf(
					"mutation refresh = scope %#v query %q rows %#v",
					notes.scope,
					notes.SearchQuery(),
					notes.visibleRows,
				)
			}
		})
	}
}

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
		AllNotesScope(),
		service,
		nil,
		projects...,
	)
	view.SetSize(width, height)
	view.setErrorStatus(strings.Repeat("actionable visible error summary ", 8))

	assertNotesLayoutFits(t, "flat list", view, width, height)
	if len(view.visibleRows) != 3 {
		t.Fatalf("duplicate located rows = %d, want three", len(view.visibleRows))
	}

	_, _ = view.Update(syntheticKey('s'))
	for _, r := range "needlemvx" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.locationPicker != nil || view.renaming || view.confirm.IsActive() {
		t.Fatal("search input leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "persistent search", view, width, height)
	_, _ = view.Update(syntheticSpecialKey(tea.KeyEsc))

	_, _ = view.Update(syntheticKey('v'))
	for _, r := range "project 159 smnx" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerScope ||
		view.searchEditing || view.renaming || view.confirm.IsActive() {
		t.Fatal("scope picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "scope picker", view, width, height)
	cancelNotesPicker(t, view)

	_, _ = view.Update(syntheticKey('n'))
	for _, r := range "project 159 smvx" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerCreation ||
		view.searchEditing || view.renaming || view.confirm.IsActive() {
		t.Fatal("creation picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "creation picker", view, width, height)
	cancelNotesPicker(t, view)

	_, _ = view.Update(syntheticKey('m'))
	for _, r := range "project 159 snvx" {
		_, _ = view.Update(syntheticKey(r))
	}
	if view.locationPicker == nil || view.locationPicker.purpose != notesLocationPickerMove ||
		view.searchEditing || view.renaming || view.confirm.IsActive() {
		t.Fatal("move picker leaked typed operation keys into the manager")
	}
	assertNotesLayoutFits(t, "move picker", view, width, height)
	cancelNotesPicker(t, view)
}

func assertNotesLayoutFits(t *testing.T, state string, view *NotesViewModel, width, height int) {
	t.Helper()
	content, auditIssues := withRenderAudit(func() string {
		return view.View().Content
	})
	_, issues := normalizeScreen(content, width, height)
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
	result, ok := cmd().(notesLocationPickerResultMsg)
	if !ok || result.Selected {
		t.Fatalf("picker cancellation result = %#v", result)
	}
	_, _ = view.Update(result)
	if view.locationPicker != nil {
		t.Fatal("picker remained open after cancellation")
	}
}

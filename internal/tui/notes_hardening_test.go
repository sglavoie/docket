package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/route"
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
	view := notes.NewNotesViewModelWithService(
		t.TempDir(),
		route.AllNotesScope(),
		service,
		nil,
		alpha,
		beta,
	)
	mountNotesManager(t, view, route.AllNotesScope())
	view.SetSize(80, 24)
	view.SearchApplied = "needle"
	view.RederiveVisibleRows()

	_, _ = view.Update(syntheticKey('%'))
	betaItem := notes.NotesLocationPickerItem{
		Label:    beta.Name,
		Scope:    route.ProjectNotesScope(beta.ID),
		Location: app.ProjectNoteLocation(beta.ID),
	}
	view.LocationPicker.SetCursorToID(betaItem.ItemID())
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
	if len(view.LoadedRows) != 1 || view.LoadedRows[0].Location != app.GlobalNoteLocation() {
		t.Fatalf("removed owner survived canonical rows: %#v", view.LoadedRows)
	}
	if _, ok := view.Owners[app.ProjectNoteLocation(beta.ID)]; ok {
		t.Fatal("removed Beta owner survived workspace refresh")
	}
	if owner := view.OwnerForLocation(app.ProjectNoteLocation(alpha.ID)); owner.Label != renamedAlpha.Name {
		t.Fatalf("renamed owner label = %q, want %q", owner.Label, renamedAlpha.Name)
	}
	if view.LocationPicker == nil || view.LocationPicker.SearchQuery() != "beta" ||
		len(view.LocationPicker.FilteredItems()) != 0 {
		t.Fatalf(
			"refreshed move picker = picker %v query %q matches %#v",
			view.LocationPicker,
			view.LocationPicker.SearchQuery(),
			view.LocationPicker.FilteredItems(),
		)
	}

	_, cmd := view.Update(notes.NotesLocationPickerResultMsg{
		Purpose:  notes.NotesLocationPickerMove,
		Item:     betaItem,
		Selected: true,
	})
	if cmd != nil || view.LocationPicker != nil || !view.StatusIsError ||
		!strings.Contains(view.StatusMessage, "no longer available") {
		t.Fatalf(
			"stale picker result = cmd %v picker %v error %t status %q",
			cmd,
			view.LocationPicker,
			view.StatusIsError,
			view.StatusMessage,
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
			_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{
				Scope: route.ProjectNotesScope(project.ID),
			})
			manager := shell.getActiveNotesView()
			manager.SearchApplied = "needle"
			manager.RederiveVisibleRows()
			_, _ = manager.Update(syntheticKey('a'))

			setTestAppProjects(shell, tc.projects(project)...)
			_ = shell.refreshActiveNotesManager()

			params := shell.CurrentRoute().Params().(route.ManageParams)
			manager = shell.getActiveNotesView()
			if params.NotesScope != route.AllNotesScope() || manager.Scope != route.AllNotesScope() {
				t.Fatalf("invalid scope survived refresh: route %#v model %#v", params.NotesScope, manager.Scope)
			}
			if manager.LocationPicker != nil {
				t.Fatal("invalid project creation destination survived route normalization")
			}
			if manager.SearchQuery() != "needle" {
				t.Fatalf("scope fallback lost accepted search %q", manager.SearchQuery())
			}
			if len(manager.Owners) != 1 || !manager.LocationAvailable(app.GlobalNoteLocation()) {
				t.Fatalf("scope fallback owners = %#v, want Global only", manager.Owners)
			}
			if shell.currentProject != nil {
				t.Fatalf("invalid current project survived refresh: %#v", shell.currentProject)
			}
			if !shell.feedback.info || !strings.Contains(shell.feedback.message, "showing All Notes") ||
				!strings.Contains(manager.StatusMessage, "showing All Notes") {
				t.Fatalf(
					"scope fallback status = shell %q info=%t manager %q",
					shell.feedback.message,
					shell.feedback.info,
					manager.StatusMessage,
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
	_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.AllNotesScope()})
	manager := shell.getActiveNotesView()
	_, _ = manager.Update(syntheticKey('a'))
	for _, r := range "renamed" {
		_, _ = manager.Update(syntheticKey(r))
	}
	if len(manager.LocationPicker.FilteredItems()) != 0 {
		t.Fatalf("pre-rename picker unexpectedly matched %#v", manager.LocationPicker.FilteredItems())
	}

	renamed := *beta
	renamed.Name = "Renamed Destination"
	setTestAppProjects(shell, alpha, &renamed)
	_ = shell.refreshActiveNotesManager()

	manager = shell.getActiveNotesView()
	filtered := manager.LocationPicker.FilteredItems()
	if manager.LocationPicker.SearchQuery() != "renamed" ||
		len(filtered) != 1 || filtered[0].Location != app.ProjectNoteLocation(beta.ID) ||
		filtered[0].Label != renamed.Name {
		t.Fatalf(
			"renamed picker state = query %q filtered %#v",
			manager.LocationPicker.SearchQuery(),
			filtered,
		)
	}
	if owner := manager.OwnerForLocation(app.ProjectNoteLocation(beta.ID)); owner.Label != renamed.Name {
		t.Fatalf("renamed owner label = %q, want %q", owner.Label, renamed.Name)
	}
}

func TestNotesManagerReopenRebuildsActiveOwnerCatalog(t *testing.T) {
	shell := typedTestApp(t)
	alpha := shell.currentProject
	_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.AllNotesScope()})
	manager := shell.getActiveNotesView()

	renamedAlpha := *alpha
	renamedAlpha.Name = "Alpha Renamed"
	beta := &models.Project{ID: "beta", Name: "Beta", Slug: "beta", Path: t.TempDir()}
	setTestAppProjects(shell, &renamedAlpha, beta)
	_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.AllNotesScope()})

	if shell.getActiveNotesView() != manager {
		t.Fatal("reopening Notes rebuilt the manager instead of refreshing it")
	}
	if owner := manager.OwnerForLocation(app.ProjectNoteLocation(alpha.ID)); owner.Label != renamedAlpha.Name {
		t.Fatalf("reopened Alpha label = %q, want %q", owner.Label, renamedAlpha.Name)
	}
	if !manager.LocationAvailable(app.ProjectNoteLocation(beta.ID)) {
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
			_, _ = shell.handleNotesMessages(notes.ShowNotesManagerMsg{
				Scope: route.ProjectNotesScope(project.ID),
			})
			manager := shell.getActiveNotesView()
			manager.NoteService = service
			manager.SearchApplied = "needle"
			manager.LoadNotes()

			service.byLocation[location] = []*models.Note{{
				Title: "Needle after", Filename: "after.md", Body: "needle",
			}}
			shell.reconcileNoteResult(notes.ProjectNoteMutationResultMsg{
				Location: location,
				Result: app.NoteBatchResult{Results: []app.NoteResult{{
					Kind:      kind,
					Location:  location,
					ProjectID: project.ID,
					Filename:  "after.md",
				}}},
				SuccessMessage: "Changed",
			})

			if manager.Scope != route.ProjectNotesScope(project.ID) ||
				manager.SearchQuery() != "needle" ||
				len(manager.VisibleRows) != 1 ||
				manager.VisibleRows[0].Identity() != (notes.NoteRowIdentity{
					Location: location,
					Filename: "after.md",
				}) {
				t.Fatalf(
					"mutation refresh = scope %#v query %q rows %#v",
					manager.Scope,
					manager.SearchQuery(),
					manager.VisibleRows,
				)
			}
		})
	}
}

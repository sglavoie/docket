package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	apppkg "github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/listsurface"
	"github.com/sglavoie/docket/internal/tui/notes"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/route"
)

type manageHubNotesPort struct {
	apppkg.WorkspaceNotePort
	byLocation map[apppkg.NoteLocation][]*models.Note
}

func (p *manageHubNotesPort) Notes(
	location apppkg.NoteLocation,
	_ *models.Project,
) ([]*models.Note, error) {
	return append([]*models.Note(nil), p.byLocation[location]...), nil
}

func TestManageHubDeepLinksUseOneTypedRouteAndDirectEsc(t *testing.T) {
	tests := []struct {
		name   string
		open   func(*AppModel, *models.Project) (bool, tea.Cmd)
		params route.ManageParams
	}{
		{name: "project notes", open: func(app *AppModel, project *models.Project) (bool, tea.Cmd) {
			return app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)})
		}, params: route.ManageParams{Module: route.ManageNotes, NotesScope: route.ProjectNotesScope("project")}},
		{name: "global notes", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.GlobalNotesScope()})
		}, params: route.ManageParams{Module: route.ManageNotes, NotesScope: route.GlobalNotesScope()}},
		{name: "all notes", open: func(app *AppModel, _ *models.Project) (bool, tea.Cmd) {
			return app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.AllNotesScope()})
		}, params: route.ManageParams{Module: route.ManageNotes, NotesScope: route.AllNotesScope()}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := typedTestApp(t)
			project := app.currentProject
			want := tc.params
			if want.NotesScope.Kind == route.NotesScopeProject {
				want.NotesScope.ProjectID = project.ID
			}
			handled, _ := tc.open(app, project)
			if !handled {
				t.Fatal("deep link was not handled")
			}

			current := app.CurrentRoute()
			params, ok := current.Params().(route.ManageParams)
			if current.ID() != route.SecondaryTool || !ok || params != want {
				t.Fatalf("deep link route = %s/%#v, want Manage/%#v", current.ID(), current.Params(), want)
			}
			if _, legacy := current.Presentation(); legacy {
				t.Fatal("live Manage module retained an independent legacy presentation")
			}
			if app.activeView() != app.manageHub || app.manageHub.Params() != want {
				t.Fatalf("active Manage renderer/params = %T/%#v", app.activeView(), app.manageHub.Params())
			}

			// A single Esc dismisses the child tool directly back to its launching
			// destination — the sole Outliner root — with no intermediate landing.
			app.handleGlobalKeyPress(syntheticSpecialKey(27))
			if app.CurrentRoute().ID() != route.TaskWorkspace {
				t.Fatalf("Esc route = %s, want TaskWorkspace (Outliner root)", app.CurrentRoute().ID())
			}
		})
	}
}

func TestManageHubScopeUsesRouteParams(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.GlobalNotesScope()})
	globalNotes := app.manageHub.ActiveModuleView().(*notes.NotesViewModel)
	if params := app.CurrentRoute().Params().(route.ManageParams); params.NotesScope != route.GlobalNotesScope() ||
		globalNotes.Scope != route.GlobalNotesScope() ||
		app.manageHub.StatusBarLabel() != "Notes · Global" {
		t.Fatalf("initial Global scope = %#v/%#v/%q", params.NotesScope, globalNotes.Scope, app.manageHub.StatusBarLabel())
	}
	_, _ = app.manageHub.Update(syntheticKey('p'))
	if globalNotes.LocationPicker == nil || globalNotes.LocationPicker.Purpose != notes.NotesLocationPickerScope {
		t.Fatal("Manage Hub did not forward p to the Notes scope picker")
	}
	globalNotes.LocationPicker.SetCursorToID(notes.NotesLocationPickerItem{
		Scope: route.ProjectNotesScope(project.ID),
	}.ItemID())
	_, cmd := app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("project scope selection emitted no picker result")
	}
	_, cmd = app.manageHub.Update(cmd())
	if cmd == nil {
		t.Fatal("project scope result emitted no route change")
	}
	app.handleManageHubMessages(cmd())
	params := app.CurrentRoute().Params().(route.ManageParams)
	manager, ok := app.manageHub.ActiveModuleView().(*notes.NotesViewModel)
	var resolvedProject *models.Project
	if ok {
		resolvedProject = manager.ProjectByID(project.ID)
	}
	if params.NotesScope != route.ProjectNotesScope(project.ID) || !ok ||
		manager.Scope != route.ProjectNotesScope(project.ID) ||
		resolvedProject == nil || resolvedProject.ID != project.ID {
		t.Fatalf("project scope = %#v/%T, want %q/%p", params.NotesScope, app.manageHub.ActiveModuleView(), project.ID, project)
	}
	if manager != globalNotes {
		t.Fatal("scope selection rebuilt the Notes manager")
	}
	_, _ = app.manageHub.Update(syntheticKey('a'))
	if manager.LocationPicker == nil || manager.LocationPicker.Purpose != notes.NotesLocationPickerCreation ||
		manager.LocationPicker.SelectedItem().Location != apppkg.ProjectNoteLocation(project.ID) {
		t.Fatalf("project scope creation picker = %#v, want project %q selected", manager.LocationPicker, project.ID)
	}
	if app.viewNav.navigation.Primary.Depth() != 2 {
		t.Fatalf("scope replacement depth = %d, want root plus one child", app.viewNav.navigation.Primary.Depth())
	}
}

func TestManageHubNotesScopeMapsCommandContextAndPresentation(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	other := &models.Project{ID: "other", Name: "Other", Slug: "other"}
	setTestAppProjects(app, project, other)

	tests := []struct {
		name              string
		scope             route.NotesScope
		wantProjectIDs    []string
		wantStatus        string
		wantPresentation  presentation.PresentationID
		wantAppCapability bool
	}{
		{
			name: "all", scope: route.AllNotesScope(),
			wantProjectIDs: operation.NormalizedProjectIDs([]string{other.ID, project.ID}),
			wantStatus:     "Notes · All", wantPresentation: presentation.ViewGlobalNotes, wantAppCapability: true,
		},
		{
			name: "global", scope: route.GlobalNotesScope(),
			wantProjectIDs: operation.NormalizedProjectIDs([]string{other.ID, project.ID}),
			wantStatus:     "Notes · Global", wantPresentation: presentation.ViewGlobalNotes, wantAppCapability: true,
		},
		{
			name: "project", scope: route.ProjectNotesScope(project.ID),
			wantProjectIDs: []string{project.ID},
			wantStatus:     "Notes · Project", wantPresentation: presentation.ViewNotes,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: test.scope})
			ctx := app.manageHub.CommandContext()
			presentation, ok := app.manageHub.PresentationView()
			if !slices.Equal(ctx.ScopeProjectIDs, test.wantProjectIDs) ||
				app.manageHub.StatusBarLabel() != test.wantStatus ||
				!ok || presentation != test.wantPresentation {
				t.Fatalf(
					"scope mapping = context %#v status %q presentation %s/%t",
					ctx,
					app.manageHub.StatusBarLabel(),
					presentation,
					ok,
				)
			}
			if ctx.Capabilities.Has(operation.CapabilityAppScope) != test.wantAppCapability ||
				ctx.Capabilities.Has(operation.CapabilityProjectScope) == test.wantAppCapability {
				t.Fatalf("scope capabilities = %#v, want app=%t", ctx.Capabilities, test.wantAppCapability)
			}
		})
	}
}

func TestNotesManagerMessageFocusIsLocationQualified(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	filename := "duplicate.md"
	service := apppkg.NewServiceWithArtifacts(
		newTestAppRepo(project),
		nil,
		nil,
		&manageHubNotesPort{
			WorkspaceNotePort: apppkg.NewStorageWorkspaceNotePort(app.domain.runtime.DataDir()),
			byLocation: map[apppkg.NoteLocation][]*models.Note{
				apppkg.GlobalNoteLocation(): {
					{Title: "Global duplicate", Filename: filename},
				},
				apppkg.ProjectNoteLocation(project.ID): {
					{Title: "Project duplicate", Filename: filename},
				},
			},
		},
	)
	app.domain.service = service
	app.domain.ledger = nil

	focus := notes.NoteRowIdentity{
		Location: apppkg.ProjectNoteLocation(project.ID),
		Filename: filename,
	}
	handled, _ := app.handleNotesMessages(notes.ShowNotesManagerMsg{
		Scope: route.AllNotesScope(),
		Focus: focus,
	})
	manager := app.manageHub.ActiveModuleView().(*notes.NotesViewModel)
	if !handled || len(manager.VisibleRows) != 2 || manager.CurrentRow() == nil ||
		manager.CurrentRow().Identity() != focus {
		t.Fatalf("focused manager = handled %t rows %d current %+v, want %+v", handled, len(manager.VisibleRows), manager.CurrentRow(), focus)
	}
}

func TestManageHubCollectionGrammarAndProjectPickerFallback(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	other := &models.Project{ID: "other", Name: "Other Project", Slug: "other"}
	setTestAppProjects(app, project, other)

	app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)})
	manager := app.manageHub.ActiveModuleView().(*notes.NotesViewModel)
	manager.VisibleRows = []notes.LocatedNoteRow{{
		Note:       &models.Note{Filename: "one.md", Title: "One"},
		Location:   apppkg.GlobalNoteLocation(),
		OwnerLabel: "Global",
	}}
	// Notes has no multi-select, so Space is one note's action sheet.
	_, spaceCmd := app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeySpace})
	sheet := app.operations.ActionSheet
	if spaceCmd != nil || len(manager.VisibleRows) != 1 || sheet == nil ||
		sheet.Config.Bulk || sheet.Config.Targets != 1 {
		t.Fatalf("Space did not open a single-note action sheet: %#v", sheet)
	}
	if got := len(sheet.Surface.Items()); got != 1 ||
		!sheet.Surface.SetCursorToID(listsurface.ListItemID("note-action:"+string(notes.OperationNoteDelete))) {
		t.Fatalf("note action sheet holds %d rows, want Delete alone", got)
	}
	app.operations.ActionSheet = nil
	_, _ = app.manageHub.Update(syntheticKey('a'))
	if manager.LocationPicker == nil || manager.LocationPicker.Purpose != notes.NotesLocationPickerCreation {
		t.Fatal("Notes a did not open the destination picker")
	}
	if manager.LocationPicker.SelectedItem().Location != apppkg.ProjectNoteLocation(project.ID) {
		t.Fatalf("project Notes preselection = %+v, want project", manager.LocationPicker.SelectedItem().Location)
	}
	var destinations []apppkg.NoteLocation
	for _, item := range manager.LocationPicker.SourceItems() {
		destinations = append(destinations, item.Location)
	}
	if !slices.Contains(destinations, apppkg.ProjectNoteLocation(other.ID)) {
		t.Fatalf("Notes creation destinations omitted active project %q: %+v", other.ID, destinations)
	}

	wantHelp := []string{"↑/↓", "Ctrl+J/K", "type", "Enter", "Esc"}
	help := app.manageHub.GetHelpItems()
	gotHelp := make([]string, len(help))
	for i := range help {
		gotHelp[i] = help[i].Key
	}
	if !slices.Equal(gotHelp, wantHelp) {
		t.Fatalf("Manage collection grammar = %v, want %v", gotHelp, wantHelp)
	}

	app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope("missing")})
	params := app.CurrentRoute().Params().(route.ManageParams)
	if app.CurrentRoute().ID() != route.SecondaryTool || params.NotesScope != route.AllNotesScope() ||
		app.manageHub.ActiveModuleView().(*notes.NotesViewModel).LocationPicker != nil {
		t.Fatalf("missing project fallback = %s/%#v, want All Notes without a picker", app.CurrentRoute().ID(), params)
	}
}

func TestManageHubNotesDeleteKeyDeletesGlobalAndProjectNotes(t *testing.T) {
	tests := []struct {
		name     string
		location apppkg.NoteLocation
		open     func(*AppModel, *models.Project)
	}{
		{
			name:     "global",
			location: apppkg.GlobalNoteLocation(),
			open: func(app *AppModel, _ *models.Project) {
				app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.GlobalNotesScope()})
			},
		},
		{
			name:     "project",
			location: apppkg.ProjectNoteLocation("project"),
			open: func(app *AppModel, project *models.Project) {
				app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)})
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := typedTestApp(t)
			project := app.currentProject
			location := test.location
			if location.Scope == apppkg.NoteScopeProject {
				location.ProjectID = project.ID
			}
			test.open(app, project)

			note := &models.Note{Filename: test.name + ".md", Title: test.name + " note"}
			service := &combinedNotesService{byLocation: map[apppkg.NoteLocation][]*models.Note{}}
			manager := app.manageHub.ActiveModuleView().(*notes.NotesViewModel)
			manager.NoteService = service
			manager.VisibleRows = []notes.LocatedNoteRow{{
				Note:       note,
				Location:   location,
				OwnerLabel: manager.LocationLabel(location),
			}}

			_, cmd := app.manageHub.Update(tea.KeyPressMsg{Code: tea.KeySpace})
			if cmd != nil || app.operations.ActionSheet == nil {
				t.Fatalf("Space did not open the note action sheet: cmd=%v", cmd)
			}
			handled, sheetCmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter})
			if !handled || sheetCmd == nil {
				t.Fatalf("sheet Enter = handled %t cmd %v", handled, sheetCmd)
			}
			if handled, _ := app.handleOperationOverlayMessage(sheetCmd()); !handled {
				t.Fatal("the sheet's Delete row was not executed")
			}
			if app.operations.ActionSheet != nil || !manager.Confirm.IsActive() {
				t.Fatalf("sheet Delete did not hand the confirmation back: active=%t", manager.Confirm.IsActive())
			}
			if overlay := app.renderConfirmDialogOverlay(); !strings.Contains(overlay, note.Title) {
				t.Fatalf("delete confirmation overlay missing note title %q: %q", note.Title, overlay)
			}

			_, cmd = app.manageHub.Update(syntheticKey('y'))
			if cmd == nil {
				t.Fatal("confirmed delete emitted no mutation command")
			}
			result, ok := cmd().(notes.ProjectNoteMutationResultMsg)
			if !ok || result.Location != location {
				t.Fatalf("delete result = %#v, want location %+v", result, location)
			}
			if len(service.requests) != 1 || len(service.requests[0].Operations) != 1 {
				t.Fatalf("delete requests = %#v, want one operation", service.requests)
			}
			operation := service.requests[0].Operations[0]
			if operation.Kind != apppkg.NoteDelete || operation.Location != location ||
				operation.Note.Filename != note.Filename {
				t.Fatalf("delete operation = %#v, want %s at %+v", operation, note.Filename, location)
			}
		})
	}
}

func TestManageHubRemovedProjectFallsBackToAllNotes(t *testing.T) {
	app := typedTestApp(t)
	project := app.currentProject
	app.handleNotesMessages(notes.ShowNotesManagerMsg{Scope: route.ProjectNotesScope(project.ID)})
	setTestAppProjects(app)

	handled, _ := app.handleGlobalKeyPress(syntheticKey('j'))
	if !handled {
		t.Fatal("removed Manage project scope was not normalized")
	}
	params := app.CurrentRoute().Params().(route.ManageParams)
	if app.CurrentRoute().ID() != route.SecondaryTool || params.Module != route.ManageNotes || params.NotesScope != route.AllNotesScope() {
		t.Fatalf("removed project fallback = %s/%#v, want All Notes", app.CurrentRoute().ID(), params)
	}
	if app.currentProject != nil {
		t.Fatal("removed project pointer remained selected")
	}
}

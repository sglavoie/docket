package tui

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func openProjectHub(t *testing.T, app *AppModel) *ProjectHubModel {
	t.Helper()
	handled, cmd := app.handleContextMessages(ShowProjectHubMsg{})
	if !handled {
		t.Fatal("ShowProjectHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*ProjectHubModel)
	if !ok {
		t.Fatalf("active Manage module view = %T, want *ProjectHubModel", app.manageHub.ActiveModuleView())
	}
	hub.SetSize(120, 30)
	return hub
}

func focusProjectHubItem(t *testing.T, hub *ProjectHubModel, id ListItemID) {
	t.Helper()
	if hub.surface == nil || !hub.surface.SetCursorToID(id) {
		t.Fatalf("project hub omitted item %q", id)
	}
}

func projectHubActionIDs(hub *ProjectHubModel) []OperationID {
	if hub.actions == nil || hub.actions.surface == nil {
		return nil
	}
	ids := make([]OperationID, 0)
	for _, item := range hub.actions.surface.Items() {
		if descriptor, ok := item.Data.(OperationDescriptor); ok {
			ids = append(ids, descriptor.ID)
		}
	}
	return ids
}

func TestProjectHubListsEveryProjectByLifecycle(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Slug = "archived"
	archived.Context = "work"
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(app, archived)

	hub := openProjectHub(t, app)
	wantCategories := []ListCategory{
		{ID: projectHubCategoryActive, Label: "Active Projects"},
		{ID: projectHubCategoryArchived, Label: "Archived Projects"},
	}
	if got := hub.surface.Categories(); !reflect.DeepEqual(got, wantCategories) {
		t.Fatalf("project categories = %#v, want %#v", got, wantCategories)
	}
	for _, id := range []ListItemID{
		"project-hub:project:alpha",
		"project-hub:project:beta",
		"project-hub:archived:archived",
	} {
		if !hub.surface.SetCursorToID(id) {
			t.Errorf("Project Hub omitted %q", id)
		}
	}
	for _, item := range hub.surface.Items() {
		if _, ok := item.Data.(scopeHubEntry); ok {
			t.Fatalf("Project Hub retained a Scope Hub row: %#v", item.Data)
		}
	}
}

func TestProjectHubStartsInSearchAndEscReturnsToBrowse(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	if !hub.HasActiveSearch() || !hub.HasTextInput() || !hub.ShouldCaptureEsc() {
		t.Fatalf(
			"initial search state active=%v input=%v capture=%v",
			hub.HasActiveSearch(),
			hub.HasTextInput(),
			hub.ShouldCaptureEsc(),
		)
	}
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	if hub.SearchQuery() != "bet" || len(hub.surface.Matches()) != 1 {
		t.Fatalf("query=%q matches=%d, want beta only", hub.SearchQuery(), len(hub.surface.Matches()))
	}

	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.SearchQuery() != "" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewProjectHub {
		t.Fatalf("first Esc closed Project Hub: %s/%s", app.CurrentRoute().ID(), app.currentView())
	}
	if len(hub.surface.Matches()) != len(hub.surface.Items()) {
		t.Fatalf("Esc restored %d/%d project rows", len(hub.surface.Matches()), len(hub.surface.Items()))
	}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("clean browse Esc handled=%v route=%s, want Outliner", handled, app.CurrentRoute().ID())
	}
}

func TestProjectHubSearchEnterSelectsImmediately(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("search Enter emitted no selection")
	}
	selected, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("search Enter emitted %#v, want beta", cmd())
	}
	app.handleViewActionMessages(selected)
	if app.crossOutliner.scopedProjectID != "beta" || app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf(
			"selection landed project=%q route=%s",
			app.crossOutliner.scopedProjectID,
			app.CurrentRoute().ID(),
		)
	}
}

func TestProjectHubSearchNavigationSelectsHighlightedProject(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('a'))

	assertCurrent := func(want ListItemID) {
		t.Helper()
		current, ok := hub.surface.CurrentItem()
		if !ok || current.ID != want {
			t.Fatalf("current project = %q (present=%v), want %q", current.ID, ok, want)
		}
		if query := hub.SearchQuery(); query != "a" {
			t.Fatalf("search navigation changed query to %q, want %q", query, "a")
		}
		if !hub.HasActiveSearch() {
			t.Fatal("search navigation deactivated search")
		}
	}

	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticCtrlKey('j'))
	assertCurrent("project-hub:project:beta")
	hub.Update(syntheticCtrlKey('k'))
	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticSpecialKey(tea.KeyDown))
	assertCurrent("project-hub:project:beta")
	hub.Update(syntheticSpecialKey(tea.KeyUp))
	assertCurrent("project-hub:project:alpha")
	hub.Update(syntheticSpecialKey(tea.KeyDown))

	_, cmd := hub.Update(syntheticSpecialKey(tea.KeyEnter))
	if cmd == nil {
		t.Fatal("Enter after search navigation emitted no selection")
	}
	selected, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("Enter after search navigation emitted %#v, want beta", cmd())
	}
	if help := hub.footerHelp(120); !strings.Contains(help, "↑↓/Ctrl+j/k move") {
		t.Fatalf("search footer does not advertise navigation: %q", help)
	}
}

func TestProjectHubSearchPlainJAndKRemainQueryInput(t *testing.T) {
	hub := openProjectHub(t, contextHubApp(t))
	hub.Update(syntheticKey('j'))
	hub.Update(syntheticKey('k'))

	if query := hub.SearchQuery(); query != "jk" {
		t.Fatalf("plain j/k query = %q, want %q", query, "jk")
	}
	if !hub.HasActiveSearch() {
		t.Fatal("plain j/k deactivated search")
	}
}

func TestProjectHubArchivedEnterKeepsHubOpenWithNotice(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:archived:archived")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("archived Enter emitted %#v", cmd())
	}
	if !strings.Contains(hub.notice, "unarchived") || app.currentView() != ViewProjectHub {
		t.Fatalf("archived Enter notice=%q view=%s", hub.notice, app.currentView())
	}
}

func TestProjectHubBrowseNCreatesProject(t *testing.T) {
	hub := openProjectHub(t, contextHubApp(t))
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})

	_, cmd := hub.Update(syntheticKey('n'))
	if cmd == nil {
		t.Fatal("browse n emitted no project creation request")
	}
	if _, ok := cmd().(CreateProjectMsg); !ok {
		t.Fatalf("browse n emitted %T, want CreateProjectMsg", cmd())
	}
}

func TestProjectHubReopensWithFreshSearch(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() {
		t.Fatal("fixture did not leave initial search")
	}
	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled {
		t.Fatal("browse Esc did not close Project Hub")
	}

	reopened := openProjectHub(t, app)
	if !reopened.HasActiveSearch() || reopened == hub {
		t.Fatalf("reopened hub active=%v reused=%v", reopened.HasActiveSearch(), reopened == hub)
	}
}

func TestProjectHubStaleProjectActionUsesProductionExecutorError(t *testing.T) {
	app := contextHubApp(t)
	hub := openProjectHub(t, app)
	if hub.registry != app.operationRegistry || hub.executor != app.operationExecutor {
		t.Fatal("Project Hub did not receive the production operation system")
	}

	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}
	if !hub.actions.surface.SetCursorToID(ListItemID("project-hub-action:" + string(operationScopeRenameProject))) {
		t.Fatal("rename action was not selectable")
	}

	alpha := app.projectByID("alpha")
	setTestAppProjects(app, alpha)
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("stale project action still emitted %#v", cmd())
	}
	if hub.notice != "project beta is no longer available" {
		t.Fatalf("stale project action notice = %q, want production executor error", hub.notice)
	}
}

func TestSimplifiedUppercaseCOpensSearchFirstProjectHub(t *testing.T) {
	app := contextHubApp(t)
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	for name, register := range map[string]func() error{
		"core":        app.registerCoreInputOperations,
		"scope hub":   app.registerScopeHubOperations,
		"project hub": app.registerProjectHubOperations,
		"project":     app.registerProjectOperations,
		"task":        app.registerTaskActionOperations,
		"filter":      app.registerFilterOperations,
	} {
		if err := register(); err != nil {
			t.Fatalf("register %s operations: %v", name, err)
		}
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input strategy: %v", err)
	}

	handled, cmd := app.handleSimplifiedInputKey(syntheticKey('C'))
	if !handled || cmd == nil {
		t.Fatal("simplified C did not dispatch Select Project")
	}
	msg := cmd()
	handled, activate := app.handleContextMessages(msg)
	if !handled {
		t.Fatalf("project operation emitted unhandled %T", msg)
	}
	if activate != nil {
		activate()
	}
	hub, ok := app.manageHub.ActiveModuleView().(*ProjectHubModel)
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewProjectHub || !ok {
		t.Fatalf("C landed on %s/%s (%T)", app.CurrentRoute().ID(), app.currentView(), app.manageHub.ActiveModuleView())
	}
	if !hub.HasActiveSearch() {
		t.Fatal("C did not prime project search")
	}
}

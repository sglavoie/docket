package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

func focusScopeHubItem(t *testing.T, hub *ContextHubModel, id ListItemID) {
	t.Helper()
	if hub.surface == nil || !hub.surface.SetCursorToID(id) {
		t.Fatalf("scope hub omitted item %q", id)
	}
}

func scopeHubActionIDs(hub *ContextHubModel) []OperationID {
	if hub.actions == nil || hub.actions.surface == nil {
		return nil
	}
	ids := make([]OperationID, 0)
	for _, item := range hub.actions.surface.Items() {
		descriptor, ok := item.Data.(OperationDescriptor)
		if ok {
			ids = append(ids, descriptor.ID)
		}
	}
	return ids
}

func TestScopeHubUsesTheFourCanonicalListSurfaceCategories(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Slug = "archived"
	archived.Context = "work"
	archived.Archived = true
	archived.Path = t.TempDir()
	addTestAppProject(app, archived)

	hub := openContextHub(t, app)
	got := hub.surface.Categories()
	want := []ListCategory{
		{ID: scopeHubCategoryBase, Label: "Base Scopes"},
		{ID: scopeHubCategoryActive, Label: "Active Projects"},
		{ID: scopeHubCategoryArchived, Label: "Archived Projects"},
		{ID: scopeHubCategoryTrash, Label: "Trash"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scope categories = %#v, want %#v", got, want)
	}

	kinds := make(map[scopeHubEntryKind]int)
	for _, item := range hub.surface.Items() {
		entry, ok := item.Data.(scopeHubEntry)
		if ok {
			kinds[entry.Kind]++
		}
	}
	for _, kind := range []scopeHubEntryKind{scopeHubEntryBase, scopeHubEntryActiveProject, scopeHubEntryArchivedProject} {
		if kinds[kind] == 0 {
			t.Fatalf("scope hub omitted lifecycle kind %d: %#v", kind, kinds)
		}
	}
}

func TestScopeHubEnterActivatesOnlyBaseScopesAndActiveProjects(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	focusScopeHubItem(t, hub, "scope-hub:active:beta")
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("active project Enter emitted no focus request")
	}
	focus, ok := cmd().(ApplyProjectScopeMsg)
	if !ok || focus.ProjectID != "beta" {
		t.Fatalf("active project Enter emitted %#v, want beta focus", cmd())
	}
	app.handleViewActionMessages(focus)
	if app.crossOutliner.scopedProjectID != "beta" {
		t.Fatalf("project focus = %q, want beta", app.crossOutliner.scopedProjectID)
	}
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("project focus left route %s, want Outliner root", app.CurrentRoute().ID())
	}

	hub = openContextHub(t, app)
	focusScopeHubItem(t, hub, contextScopeListItemID(RegularContextScope("work")))
	_, cmd = hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := cmd().(ContextScopeSelectedMsg)
	if !ok {
		t.Fatalf("base scope Enter emitted %T, want ContextScopeSelectedMsg", cmd())
	}
	app.handleContextMessages(selected)
	if app.crossOutliner.scopedProjectID != "" {
		t.Fatalf("context activation retained project focus %q", app.crossOutliner.scopedProjectID)
	}
	if app.crossOutliner.activeContext != RegularContextScope("work") {
		t.Fatalf("active context = %#v, want work", app.crossOutliner.activeContext)
	}
}

func TestScopeHubSpaceOpensRegistryBackedContextActions(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))
	focusScopeHubItem(t, hub, contextScopeListItemID(CustomContextScope("focus")))

	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not open the contextual action sheet")
	}
	if !hub.HasTextInput() {
		t.Fatal("action sheet did not suppress shell and Manage Hub key interception")
	}
	got := scopeHubActionIDs(hub)
	for _, want := range []OperationID{
		operationScopeCreateContext,
		operationScopeEditContext,
		operationScopeRenameContext,
		operationScopeDeleteContext,
	} {
		if !containsOperationID(got, want) {
			t.Fatalf("context actions %v omit %s", got, want)
		}
	}

	if !hub.actions.surface.SetCursorToID(ListItemID("scope-hub-action:" + string(operationScopeRenameContext))) {
		t.Fatal("rename action was not selectable")
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if hub.actions != nil {
		t.Fatal("executed action sheet remained open")
	}
	if hub.editor == nil || hub.editor.originalName != "focus" {
		t.Fatalf("rename action opened editor %#v, want focus", hub.editor)
	}
}

func TestScopeHubTrashActionsPreserveRestoreAndPurgeSafety(t *testing.T) {
	app := contextHubApp(t)
	app.runtime = testRuntimeAt(t.TempDir(), app.runtime)
	app.crossOutliner.runtime = app.runtime
	deleted := &models.Project{
		ID:   "deleted-id",
		Slug: "deleted",
		Name: "Deleted",
		Path: t.TempDir(),
	}
	entryID, err := storage.MoveProjectToTrash(deleted, app.runtime.DataDir(), "scope hub test", nil)
	if err != nil {
		t.Fatalf("MoveProjectToTrash: %v", err)
	}

	hub := openContextHub(t, app)
	focusScopeHubItem(t, hub, ListItemID("scope-hub:trash:"+entryID))
	if _, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter}); cmd != nil {
		t.Fatalf("trash Enter emitted %T; recovery must live under Space", cmd())
	}
	if hub.notice == "" {
		t.Fatal("trash Enter gave no recovery guidance")
	}

	hub.Update(syntheticKey(' '))
	got := scopeHubActionIDs(hub)
	if !containsOperationID(got, operationScopeRestoreTrash) ||
		!containsOperationID(got, operationScopePurgeTrash) {
		t.Fatalf("trash actions = %v, want restore and purge", got)
	}
	hub.actions.surface.SetCursorToID(ListItemID("scope-hub-action:" + string(operationScopePurgeTrash)))
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !hub.confirm.IsActive() || hub.confirm.Action() != scopeHubConfirmPurge {
		t.Fatal("purge did not arm the explicit irreversible confirmation")
	}
}

func TestScopeHubStaleProjectActionUsesProductionExecutorError(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	if hub.registry != app.operationRegistry || hub.executor != app.operationExecutor {
		t.Fatal("Scope Hub did not receive the production operation system")
	}

	focusScopeHubItem(t, hub, "scope-hub:active:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}
	if !hub.actions.surface.SetCursorToID(ListItemID("scope-hub-action:" + string(operationScopeRenameProject))) {
		t.Fatal("rename action was not selectable")
	}

	alpha := app.projectByID("alpha")
	setTestAppProjects(app, alpha)
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("stale project action emitted no error message")
	}
	msg, ok := cmd().(ErrorMsg)
	if !ok || msg.Err == nil || msg.Err.Error() != "scope action target is no longer available" {
		t.Fatalf("stale project action emitted %#v, want the production ErrorMsg", cmd())
	}
}

func TestSimplifiedCOpensOnlyTheScopeHub(t *testing.T) {
	app := contextHubApp(t)
	app.operationRegistry = NewOperationRegistry()
	app.operationExecutor = NewOperationExecutor(app.operationRegistry)
	if err := app.registerCoreInputOperations(); err != nil {
		t.Fatalf("register core input operations: %v", err)
	}
	if err := app.registerScopeHubOperations(); err != nil {
		t.Fatalf("register scope hub operations: %v", err)
	}
	if err := app.operationRegistry.RegisterInputStrategy(SimplifiedInputStrategy()); err != nil {
		t.Fatalf("register simplified input strategy: %v", err)
	}
	handled, cmd := app.handleSimplifiedInputKey(syntheticKey('c'))
	if !handled || cmd == nil {
		t.Fatal("simplified c did not dispatch the scope operation")
	}
	msg := cmd()
	handled, activate := app.handleContextMessages(msg)
	if !handled {
		t.Fatalf("scope operation emitted unhandled %T", msg)
	}
	if activate != nil {
		activate()
	}
	if app.CurrentRoute().ID() != SecondaryTool || app.currentView() != ViewContextHub {
		t.Fatalf("c landed on %s/%s, want the routed Scope Hub", app.CurrentRoute().ID(), app.currentView())
	}
}

func TestProjectFocusReplacementNeverStacksAndEscReturnsToContextBase(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "1")
	beta.Context = "work"
	out := scopeTestOutliner(alpha, beta)
	out.runtime = testConfigWithProjects(alpha, beta)
	out.applyContextScope(RegularContextScope("work"))

	out.pushProjectScope("alpha")
	out.pushProjectScope("beta")
	if out.scopedProjectID != "beta" || len(out.scopeStack) != 1 {
		t.Fatalf("replacement focus = %q with %d frames, want beta with one base frame", out.scopedProjectID, len(out.scopeStack))
	}
	if !out.popProjectScope() {
		t.Fatal("Esc-equivalent pop did not leave the project focus")
	}
	if out.scopedProjectID != "" || out.activeContext != RegularContextScope("work") {
		t.Fatalf("focus pop returned to project=%q context=%#v, want work base", out.scopedProjectID, out.activeContext)
	}
}

func containsOperationID(ids []OperationID, want OperationID) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

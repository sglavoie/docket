package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
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

func TestScopeHubUsesTheLeanCanonicalListSurfaceCategories(t *testing.T) {
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
		{ID: scopeHubCategoryScopes, Label: "Scopes"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scope categories = %#v, want %#v", got, want)
	}

	scopes := make(map[ContextScope]ListCategoryID)
	for _, item := range hub.surface.Items() {
		entry, ok := item.Data.(scopeHubEntry)
		if ok {
			scopes[entry.Scope] = item.CategoryID
		}
	}
	if scopes[AllContextScope()] != scopeHubCategoryScopes ||
		scopes[RegularContextScope("work")] != scopeHubCategoryScopes ||
		scopes[RegularContextScope("personal")] != scopeHubCategoryScopes {
		t.Fatalf("scope categories by row = %#v", scopes)
	}
	for _, item := range hub.surface.Items() {
		if entry, ok := item.Data.(scopeHubEntry); !ok || entry.Kind != scopeHubEntryBase {
			t.Fatalf("Scope Hub retained a non-scope row: %#v", item.Data)
		}
	}
}

func TestScopeHubEnterActivatesBaseScopes(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	focusScopeHubItem(t, hub, contextScopeListItemID(RegularContextScope("work")))
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
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
	focusScopeHubItem(t, hub, contextScopeListItemID(RegularContextScope("work")))

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
	if hub.editor == nil || hub.editor.originalName != "work" {
		t.Fatalf("rename action opened editor %#v, want work", hub.editor)
	}
}

func TestScopeHubHelpDocumentsEveryBrowseKey(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))
	keys := make(map[string]bool)
	for _, item := range hub.GetHelpItems() {
		keys[item.Key] = true
	}
	for _, key := range []string{"n", "e", "r", "d"} {
		if !keys[key] {
			t.Fatalf("Scope Hub help omits browse key %q: %#v", key, hub.GetHelpItems())
		}
	}
}

func TestScopeHubCreatesRegularContextWithSharedEditor(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))

	hub.Update(syntheticKey('n'))
	if hub.editor == nil || !hub.editor.isNew() {
		t.Fatalf("n opened editor %#v, want a new regular context", hub.editor)
	}
	typeIntoContextHub(hub, "focus")
	checkEditorProject(t, hub, "alpha")

	_, cmd := hub.Update(contextHubSaveKey)
	if cmd == nil {
		t.Fatalf("new context save emitted no message (editor error: %q)", hub.editor.err)
	}
	saved, ok := cmd().(RegularContextSavedMsg)
	if !ok || !saved.IsNew || saved.Name != "focus" || saved.Assigned != 1 {
		t.Fatalf("new context save = %#v, want one-project focus creation", saved)
	}
}

func TestScopeHubPersonalContextProtectsNameAndDeleteButAllowsMembership(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))
	focusScopeHubItem(t, hub, contextScopeListItemID(RegularContextScope("personal")))

	hub.Update(syntheticKey(' '))
	got := scopeHubActionIDs(hub)
	if !containsOperationID(got, operationScopeCreateContext) || !containsOperationID(got, operationScopeEditContext) {
		t.Fatalf("personal actions %v omit creation or membership editing", got)
	}
	if containsOperationID(got, operationScopeRenameContext) || containsOperationID(got, operationScopeDeleteContext) {
		t.Fatalf("personal actions %v expose rename or delete", got)
	}
	hub.actions = nil
	for _, key := range []rune{'r', 'd'} {
		hub.Update(syntheticKey(key))
		if hub.editor != nil || hub.confirm.IsActive() {
			t.Fatalf("%q opened a protected personal-context mutation", string(key))
		}
	}
	hub.Update(syntheticKey('e'))
	if hub.editor == nil || !hub.editor.nameLocked {
		t.Fatalf("personal membership edit = %#v, want a name-locked editor", hub.editor)
	}
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.editor != nil {
		t.Fatal("Esc could not close the unchanged name-locked personal editor")
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
	if err := app.registerTaskActionOperations(); err != nil {
		t.Fatalf("register task action operations: %v", err)
	}
	if err := app.registerFilterOperations(); err != nil {
		t.Fatalf("register filter operations: %v", err)
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

func TestContextScopeChangePreservesRunViewAndClearsNavigationState(t *testing.T) {
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Context = "work"
	beta := scopeTestProject("beta", "Beta", "3")
	beta.Context = "home"
	out := scopeTestOutliner(alpha, beta)

	out.primaryMode = ModeDue
	out.SetSearchQuery("needle")
	out.filter.Priority = 3
	out.filter.Tags = map[string]bool{"focus": true}
	out.filter.DueDate = DueDateThisWeek
	out.filter.Snoozed = true
	out.pushProjectScope("alpha")
	out.crossZoomStack = []CrossProjectZoomEntry{{
		ProjectID:   "alpha",
		TaskID:      "1",
		CursorIndex: 2,
	}}
	out.cursor = 2
	out.multiSelectMode = true
	out.SelectTaskIdentity(TaskIdentity{ProjectID: "alpha", TaskID: "1"})
	// A mutation reload may already have frozen a cursor identity before the
	// Scope Hub selection arrives. It must not survive the new project set.
	out.crossReloadProjectID = "alpha"
	out.crossReloadTaskID = "1"

	out.applyContextScope(RegularContextScope("home"))

	if out.primaryMode != ModeDue || out.SearchQuery() != "needle" {
		t.Fatalf("scope switch changed mode/search: mode=%s search=%q", out.primaryMode, out.SearchQuery())
	}
	if out.filter.Priority != 3 ||
		!out.filter.Tags["focus"] ||
		out.filter.DueDate != DueDateThisWeek ||
		!out.filter.Snoozed {
		t.Fatalf("scope switch changed facets: %+v", out.filter.CommonFilterState)
	}
	if out.scopedProjectID != "" || len(out.scopeStack) != 0 || len(out.crossZoomStack) != 0 {
		t.Fatalf(
			"scope switch retained project frames: project=%q stack=%d zoom=%d",
			out.scopedProjectID,
			len(out.scopeStack),
			len(out.crossZoomStack),
		)
	}
	if out.cursor != 0 || !out.multiSelectMode || out.SelectedCount() != 1 {
		t.Fatalf(
			"scope switch selection persistence mismatch: cursor=%d active=%v selected=%d",
			out.cursor,
			out.multiSelectMode,
			out.SelectedCount(),
		)
	}
	counts := out.selectionCounts()
	if counts.Total != 1 || counts.Visible != 0 || counts.Hidden != 1 {
		t.Fatalf("scope selection counts = %+v, want total=1 visible=0 hidden=1", counts)
	}
	if out.crossReloadProjectID != "" || out.crossReloadTaskID != "" || !out.crossReloadFromTop {
		t.Fatalf(
			"scope switch retained reload cursor: project=%q task=%q from-top=%v",
			out.crossReloadProjectID,
			out.crossReloadTaskID,
			out.crossReloadFromTop,
		)
	}

	_ = out.reloadCrossProjects()
	if out.crossReloadProjectID != "" || out.crossReloadTaskID != "" || out.crossReloadFromTop {
		t.Fatalf(
			"first scope reload restored a cursor: project=%q task=%q from-top=%v",
			out.crossReloadProjectID,
			out.crossReloadTaskID,
			out.crossReloadFromTop,
		)
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

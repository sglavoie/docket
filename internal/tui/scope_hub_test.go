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
		{ID: scopeHubCategoryBase, Label: "Base Scopes"},
		{ID: scopeHubCategoryCustom, Label: "Custom Contexts"},
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
	if scopes[AllContextScope()] != scopeHubCategoryBase ||
		scopes[RegularContextScope("work")] != scopeHubCategoryBase ||
		scopes[CustomContextScope("focus")] != scopeHubCategoryCustom {
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

func TestScopeHubBrowseKeysExecuteRegistryOperationsWithFrozenTargets(t *testing.T) {
	tests := []struct {
		name      string
		key       rune
		operation OperationID
		assert    func(*testing.T, *ContextHubModel)
	}{
		{
			name:      "new",
			key:       'n',
			operation: operationScopeCreateContext,
			assert: func(t *testing.T, hub *ContextHubModel) {
				t.Helper()
				if hub.editor == nil || !hub.editor.isNew() {
					t.Fatalf("new operation opened editor %#v", hub.editor)
				}
			},
		},
		{
			name:      "edit",
			key:       'e',
			operation: operationScopeEditContext,
			assert: func(t *testing.T, hub *ContextHubModel) {
				t.Helper()
				if hub.editor == nil || hub.editor.originalName != "focus" {
					t.Fatalf("edit operation opened editor %#v", hub.editor)
				}
			},
		},
		{
			name:      "rename",
			key:       'r',
			operation: operationScopeRenameContext,
			assert: func(t *testing.T, hub *ContextHubModel) {
				t.Helper()
				if hub.editor == nil || hub.editor.originalName != "focus" {
					t.Fatalf("rename operation opened editor %#v", hub.editor)
				}
			},
		},
		{
			name:      "delete",
			key:       'd',
			operation: operationScopeDeleteContext,
			assert: func(t *testing.T, hub *ContextHubModel) {
				t.Helper()
				if !hub.confirm.IsActive() || hub.pendingDelete != CustomContextScope("focus") {
					t.Fatalf("delete operation armed confirm=%v target=%#v", hub.confirm.IsActive(), hub.pendingDelete)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hub := openContextHub(t, contextHubApp(t))
			focusScopeHubItem(t, hub, contextScopeListItemID(CustomContextScope("focus")))

			registry := NewOperationRegistry()
			descriptor := OperationDescriptor{}
			for _, candidate := range scopeHubOperationDescriptors() {
				if candidate.ID == tc.operation {
					descriptor = candidate
					break
				}
			}
			if descriptor.ID == "" {
				t.Fatalf("scope operation %s has no descriptor", tc.operation)
			}
			if err := registry.Register(descriptor); err != nil {
				t.Fatalf("register scope operation: %v", err)
			}

			var invocation OperationInvocation
			executor := NewOperationExecutor(registry)
			if err := executor.Register(OperationBinding{
				OperationID:          tc.operation,
				RequiredCapabilities: NewCapabilitySet(CapabilityScopeHub),
				Handler: func(got OperationInvocation) OperationHandlerResult {
					invocation = got
					focusScopeHubItem(t, hub, contextScopeListItemID(RegularContextScope("home")))
					target, _ := got.Value.(scopeHubActionTarget)
					return OperationHandlerResult{
						Message: scopeHubInlineActionMsg{OperationID: tc.operation, Target: target},
					}
				},
				DelegateTargetResolution: true,
				DelegateValueProvider:    true,
				DelegateConfirmation:     true,
				DelegateTransaction:      true,
			}); err != nil {
				t.Fatalf("bind scope operation: %v", err)
			}
			hub.SetOperationSystem(registry, executor)

			hub.Update(syntheticKey(tc.key))

			if invocation.Descriptor.ID != tc.operation {
				t.Fatalf("browse key invoked %s, want %s", invocation.Descriptor.ID, tc.operation)
			}
			if invocation.Source != OperationSourceDirectKey {
				t.Fatalf("invocation source = %q, want %q", invocation.Source, OperationSourceDirectKey)
			}
			target, ok := invocation.Value.(scopeHubActionTarget)
			if !ok || !invocation.HasValue || target.Scope != CustomContextScope("focus") {
				t.Fatalf("frozen invocation target = %#v/%v, want custom context focus", invocation.Value, invocation.HasValue)
			}
			tc.assert(t, hub)
			if tc.operation != operationScopeCreateContext {
				entry, ok := hub.selectedScopeHubEntry()
				if !ok || entry.Kind != scopeHubEntryBase || entry.Scope != CustomContextScope("focus") {
					t.Fatalf("inline action followed the moved cursor to %#v/%v", entry, ok)
				}
			}
		})
	}
}

func TestScopeHubNewContextBrowseKeyOpensWithoutVisibleRows(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))

	registry := NewOperationRegistry()
	var descriptor OperationDescriptor
	for _, candidate := range scopeHubOperationDescriptors() {
		if candidate.ID == operationScopeCreateContext {
			descriptor = candidate
			break
		}
	}
	if descriptor.ID == "" {
		t.Fatal("create-context operation has no descriptor")
	}
	if err := registry.Register(descriptor); err != nil {
		t.Fatalf("register create-context operation: %v", err)
	}

	var invocation OperationInvocation
	executor := NewOperationExecutor(registry)
	if err := executor.Register(OperationBinding{
		OperationID:          operationScopeCreateContext,
		RequiredCapabilities: NewCapabilitySet(CapabilityScopeHub),
		Handler: func(got OperationInvocation) OperationHandlerResult {
			invocation = got
			target, _ := got.Value.(scopeHubActionTarget)
			return OperationHandlerResult{
				Message: scopeHubInlineActionMsg{
					OperationID: operationScopeCreateContext,
					Target:      target,
				},
			}
		},
		DelegateTargetResolution: true,
		DelegateValueProvider:    true,
		DelegateConfirmation:     true,
		DelegateTransaction:      true,
	}); err != nil {
		t.Fatalf("bind create-context operation: %v", err)
	}
	hub.SetOperationSystem(registry, executor)

	hub.Update(syntheticKey('/'))
	typeIntoContextHub(hub, "nothing-matches")
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if hub.HasTextInput() || hub.SearchQuery() != "nothing-matches" {
		t.Fatalf("search was not confirmed with its query retained: input=%v query=%q", hub.HasTextInput(), hub.SearchQuery())
	}
	if matches := hub.surface.Matches(); len(matches) != 0 {
		t.Fatalf("confirmed no-match search kept %d visible rows", len(matches))
	}

	hub.Update(syntheticKey('n'))

	if invocation.Descriptor.ID != operationScopeCreateContext {
		t.Fatalf("n invoked %s, want %s", invocation.Descriptor.ID, operationScopeCreateContext)
	}
	if invocation.Source != OperationSourceDirectKey {
		t.Fatalf("invocation source = %q, want %q", invocation.Source, OperationSourceDirectKey)
	}
	target, ok := invocation.Value.(scopeHubActionTarget)
	if !ok || !invocation.HasValue || target != (scopeHubActionTarget{}) {
		t.Fatalf("no-row invocation target = %#v/%v, want empty scope target", invocation.Value, invocation.HasValue)
	}
	if hub.editor == nil || !hub.editor.isNew() {
		t.Fatalf("n with no visible rows opened editor %#v", hub.editor)
	}
	if hub.notice != "" {
		t.Fatalf("n with no visible rows produced notice %q", hub.notice)
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
	out.filter.Blocked = true
	out.filter.Snoozed = true
	out.pushProjectScope("alpha")
	out.crossZoomStack = []CrossProjectZoomEntry{{
		ProjectID:   "alpha",
		TaskID:      "1",
		CursorIndex: 2,
	}}
	out.cursor = 2
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
		!out.filter.Blocked ||
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
	if out.cursor != 0 || out.multiSelectMode || out.SelectedCount() != 0 {
		t.Fatalf(
			"scope switch retained cursor/selection: cursor=%d active=%v selected=%d",
			out.cursor,
			out.multiSelectMode,
			out.SelectedCount(),
		)
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

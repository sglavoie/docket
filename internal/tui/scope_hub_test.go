package tui

import (
	"reflect"
	"strings"
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
					focusScopeHubItem(t, hub, "scope-hub:active:beta")
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
			if !ok || !invocation.HasValue || target.Scope != CustomContextScope("focus") ||
				target.ProjectID != "" || target.TrashEntryID != "" {
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

func TestScopeHubContextBrowseKeysRejectNonContextRowsWithVisibleNotice(t *testing.T) {
	for _, row := range []struct {
		name string
		id   func(string) ListItemID
	}{
		{name: "active project", id: func(string) ListItemID { return "scope-hub:active:beta" }},
		{name: "archived project", id: func(string) ListItemID { return "scope-hub:archived:archived" }},
		{name: "trash", id: func(entryID string) ListItemID { return ListItemID("scope-hub:trash:" + entryID) }},
	} {
		for _, key := range []rune{'e', 'r', 'd'} {
			t.Run(row.name+"/"+string(key), func(t *testing.T) {
				app := contextHubApp(t)
				app.runtime = testRuntimeAt(t.TempDir(), app.runtime)
				app.crossOutliner.runtime = app.runtime
				archived := scopeTestProject("archived", "Archived", "1")
				archived.Slug = "archived"
				archived.Context = "work"
				archived.Archived = true
				archived.Path = t.TempDir()
				addTestAppProject(app, archived)

				deleted := &models.Project{
					ID:   "deleted-id",
					Slug: "deleted",
					Name: "Deleted",
					Path: t.TempDir(),
				}
				entryID, err := storage.MoveProjectToTrash(
					deleted,
					app.runtime.DataDir(),
					"scope hub browse key test",
					nil,
				)
				if err != nil {
					t.Fatalf("MoveProjectToTrash: %v", err)
				}

				hub := openContextHub(t, app)
				itemID := row.id(entryID)
				focusScopeHubItem(t, hub, itemID)
				hub.Update(syntheticKey(key))

				if hub.notice == "" {
					t.Fatalf("%q on %s produced no notice", string(key), row.name)
				}
				if hub.editor != nil || hub.confirm.IsActive() {
					t.Fatalf("%q on %s opened editor=%v confirm=%v", string(key), row.name, hub.editor != nil, hub.confirm.IsActive())
				}
				item, ok := hub.surface.CurrentItem()
				if !ok || item.ID != itemID {
					t.Fatalf("%q on %s moved cursor to %#v/%v", string(key), row.name, item, ok)
				}
				if rendered := hub.View().Content; !strings.Contains(rendered, hub.notice) {
					t.Fatalf("notice %q is not visible in the rendered hub:\n%s", hub.notice, rendered)
				}
			})
		}
	}
}

func TestScopeHubNewContextBrowseKeyOpensFromEveryNonContextRow(t *testing.T) {
	for _, row := range []struct {
		name string
		id   func(string) ListItemID
	}{
		{name: "active project", id: func(string) ListItemID { return "scope-hub:active:beta" }},
		{name: "archived project", id: func(string) ListItemID { return "scope-hub:archived:archived" }},
		{name: "trash", id: func(entryID string) ListItemID { return ListItemID("scope-hub:trash:" + entryID) }},
	} {
		t.Run(row.name, func(t *testing.T) {
			app := contextHubApp(t)
			app.runtime = testRuntimeAt(t.TempDir(), app.runtime)
			app.crossOutliner.runtime = app.runtime
			archived := scopeTestProject("archived", "Archived", "1")
			archived.Slug = "archived"
			archived.Context = "work"
			archived.Archived = true
			archived.Path = t.TempDir()
			addTestAppProject(app, archived)

			deleted := &models.Project{
				ID:   "deleted-id",
				Slug: "deleted",
				Name: "Deleted",
				Path: t.TempDir(),
			}
			entryID, err := storage.MoveProjectToTrash(
				deleted,
				app.runtime.DataDir(),
				"scope hub browse key test",
				nil,
			)
			if err != nil {
				t.Fatalf("MoveProjectToTrash: %v", err)
			}

			hub := openContextHub(t, app)
			itemID := row.id(entryID)
			focusScopeHubItem(t, hub, itemID)
			hub.Update(syntheticKey('n'))

			if hub.editor == nil || !hub.editor.isNew() {
				t.Fatalf("n on %s opened editor %#v", row.name, hub.editor)
			}
			if hub.notice != "" {
				t.Fatalf("n on %s produced notice %q", row.name, hub.notice)
			}
			item, ok := hub.surface.CurrentItem()
			if !ok || item.ID != itemID {
				t.Fatalf("n on %s moved cursor to %#v/%v", row.name, item, ok)
			}
		})
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

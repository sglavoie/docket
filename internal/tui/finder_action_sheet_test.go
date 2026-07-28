package tui

import (
	"path/filepath"
	"slices"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// finderRowFixture is one Finder row built by the production provider that owns
// its kind, so a test acts on the same manifest a ranked result really carries.
type finderRowFixture struct {
	name string
	item ListItem
}

// openFinderForActions opens the Finder over a populated workspace and returns
// the context the Finder froze when it was ranked, which is exactly what Tab
// carries into the contextual sheet.
func openFinderForActions(t *testing.T) (*AppModel, CommandContext) {
	t.Helper()
	app, discovery := openFixtureFinder(t, "")
	return app, cloneCommandContext(discovery.context)
}

func finderRowFixtures(t *testing.T, app *AppModel, ctx CommandContext) []finderRowFixture {
	t.Helper()
	project := &models.Project{ID: "alpha", Name: "Alpha", Slug: "alpha", Context: "personal"}
	note := models.NewNote("Release checklist")

	first := func(name string, provider FinderProvider) finderRowFixture {
		t.Helper()
		items, err := provider.FinderItems()
		if err != nil {
			t.Fatalf("%s FinderItems: %v", name, err)
		}
		if len(items) == 0 {
			t.Fatalf("the %s provider produced no rows", name)
		}
		return finderRowFixture{name: name, item: items[0]}
	}

	tools := globalToolCatalog(app.GetOperationRegistry(), ctx)
	if len(tools) == 0 {
		t.Fatal("the tool catalog is empty")
	}
	modes := OutlinerModeCatalog()
	if len(modes) == 0 {
		t.Fatal("the mode catalog is empty")
	}

	return []finderRowFixture{
		first("operation", FinderOperationProvider{Registry: app.GetOperationRegistry(), Context: ctx}),
		// Task actions resolve against live workspace state, so this row comes
		// from the workspace instead of a synthetic project.
		finderFirstTaskRow(t, app, ctx),
		first("project", FinderProjectProvider{Projects: []*models.Project{project}}),
		first("project note", FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: project}}}),
		first("global note", FinderNoteProvider{Notes: []FinderNote{{Note: note}}}),
		first("tool", FinderToolProvider{Tools: tools[:1]}),
		first("mode", FinderModeProvider{Modes: modes[:1]}),
		first("scope", FinderScopeProvider{Scopes: []ContextScopeSummary{{Scope: AllContextScope()}}}),
		first("custom scope", FinderScopeProvider{
			Scopes: []ContextScopeSummary{{Scope: CustomContextScope("Focus")}},
		}),
		first("tag", FinderTagProvider{Tags: []TagInfo{{Name: "urgent", TaskCount: 2}}}),
		first("saved filter", FinderSavedFilterProvider{
			Filters: []config.SavedFilter{{ID: "weekly", Name: "Weekly", Mode: "snapshot"}},
		}),
	}
}

func finderFirstTaskRow(t *testing.T, app *AppModel, ctx CommandContext) finderRowFixture {
	t.Helper()
	finder, err := NewRuntimeFinder(app.runtime, app.GetOperationRegistry(), ctx)
	if err != nil {
		t.Fatalf("NewRuntimeFinder: %v", err)
	}
	for _, item := range finder.Items() {
		if target, ok := item.Data.(FinderTarget); ok && target.Kind == FinderItemTask {
			return finderRowFixture{name: "task", item: item}
		}
	}
	t.Fatal("the fixture workspace ranked no task rows")
	return finderRowFixture{}
}

func TestFinderContextActionsCoverEveryResultKind(t *testing.T) {
	app, ctx := openFinderForActions(t)
	for _, row := range finderRowFixtures(t, app, ctx) {
		t.Run(row.name, func(t *testing.T) {
			if len(row.item.OperationIDs) == 0 {
				t.Fatalf("row %s advertises no actions", row.item.ID)
			}
			handled, _ := app.handleSearchMessages(ShowFinderContextActionsMsg{
				Item:    row.item,
				Context: ctx,
			})
			if !handled {
				t.Fatal("ShowFinderContextActionsMsg was not handled")
			}
			sheet := app.contextActionSheet
			if sheet == nil {
				t.Fatalf("row %s offered no contextual actions", row.item.ID)
			}
			if sheet.config.Host != ContextActionHostFinder {
				t.Fatalf("sheet host = %q, want %q", sheet.config.Host, ContextActionHostFinder)
			}
			if sheet.config.Completion != ContextActionCompleteCloseHost {
				t.Fatalf("sheet completion = %q, want %q", sheet.config.Completion, ContextActionCompleteCloseHost)
			}
			if sheet.config.Targets != 1 || sheet.config.Bulk {
				t.Fatalf("sheet targets = %d bulk = %v, want one frozen row",
					sheet.config.Targets, sheet.config.Bulk)
			}
			advertised := make(map[OperationID]bool, len(row.item.OperationIDs))
			for _, id := range row.item.OperationIDs {
				advertised[id] = true
			}
			offered := sheet.operationIDs()
			if len(offered) == 0 {
				t.Fatalf("row %s opened an empty sheet", row.item.ID)
			}
			for _, id := range offered {
				if !advertised[id] {
					t.Errorf("sheet offers %s, which is not on the row manifest", id)
				}
			}

			consumed, _ := app.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEsc})
			if !consumed {
				t.Fatal("the open sheet did not consume Esc")
			}
			if app.contextActionSheet != nil {
				t.Fatal("Esc left the contextual sheet open")
			}
			if app.currentView() != ViewFinder {
				t.Fatalf("current view = %v, want the Finder still open after Esc", app.currentView())
			}
		})
	}
}

func TestFinderScopeRowsOfferActivationAndOnlyCustomMutations(t *testing.T) {
	app, ctx := openFinderForActions(t)
	cases := []struct {
		name  string
		scope ContextScope
		want  []OperationID
	}{
		{"all", AllContextScope(), []OperationID{operationScopeActivateContext}},
		{"regular", RegularContextScope("personal"), []OperationID{operationScopeActivateContext}},
		{"custom", CustomContextScope("Focus"), []OperationID{
			operationScopeActivateContext,
			operationScopeEditContext,
			operationScopeRenameContext,
			operationScopeDeleteContext,
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			handled, _ := app.handleSearchMessages(ShowFinderContextActionsMsg{
				Item:    finderScopeItem(t, testCase.scope),
				Context: ctx,
			})
			if !handled || app.contextActionSheet == nil {
				t.Fatal("the scope row offered no contextual actions")
			}
			got := app.contextActionSheet.operationIDs()
			if len(got) != len(testCase.want) {
				t.Fatalf("scope actions = %v, want %v", got, testCase.want)
			}
			for index, id := range testCase.want {
				if got[index] != id {
					t.Fatalf("scope action %d = %s, want %s", index, got[index], id)
				}
			}
			app.contextActionSheet = nil
		})
	}
}

func TestFinderScopeActivationAppliesTheFrozenScope(t *testing.T) {
	app, ctx := openFinderForActions(t)
	scope := RegularContextScope("personal")
	sheet := openFinderScopeSheet(t, app, ctx, scope)

	result := finderSheetActivate(t, sheet, operationScopeActivateContext)
	if result.Result.Err != nil {
		t.Fatalf("activation failed: %v", result.Result.Err)
	}
	handled, cmd := app.handleOperationOverlayMessage(result)
	if !handled {
		t.Fatal("the shell did not route the sheet result")
	}
	completion, ok := cmd().(ContextActionSheetCompletedMsg)
	if !ok {
		t.Fatalf("activation produced %T, want ContextActionSheetCompletedMsg", cmd())
	}

	handled, cmd = app.handleSearchMessages(completion)
	if !handled {
		t.Fatal("ContextActionSheetCompletedMsg was not handled")
	}
	if app.currentView() == ViewFinder {
		t.Fatal("a completed scope action left the Finder open")
	}
	selected := false
	for _, msg := range collectFinderCmdMsgs(cmd) {
		scopeMsg, isScope := msg.(ContextScopeSelectedMsg)
		if !isScope {
			continue
		}
		selected = true
		if scopeMsg.SelectedContext != scope.Name || scopeMsg.IsCustomContext {
			t.Fatalf("activation selected %+v, want the frozen %v", scopeMsg, scope)
		}
		if handled, _ := app.handleContextMessages(scopeMsg); !handled {
			t.Fatal("ContextScopeSelectedMsg was not handled")
		}
	}
	if !selected {
		t.Fatal("activation released no scope selection")
	}
	if got := app.crossOutliner.activeContext; got != scope {
		t.Fatalf("Outliner scope = %v, want %v", got, scope)
	}
}

func TestFinderCustomScopeMutationsOpenScopeHubOnTheFrozenScope(t *testing.T) {
	cases := []struct {
		name    string
		id      OperationID
		inspect func(*testing.T, *ContextHubModel, ContextScope)
	}{
		{
			name: "rename",
			id:   operationScopeRenameContext,
			inspect: func(t *testing.T, hub *ContextHubModel, scope ContextScope) {
				t.Helper()
				if hub.editor == nil {
					t.Fatal("rename opened no membership editor")
				}
				if !hub.HasTextInput() {
					t.Fatal("the hub does not report the rename text input")
				}
			},
		},
		{
			name: "edit membership",
			id:   operationScopeEditContext,
			inspect: func(t *testing.T, hub *ContextHubModel, scope ContextScope) {
				t.Helper()
				if hub.editor == nil {
					t.Fatal("edit membership opened no editor")
				}
			},
		},
		{
			name: "delete",
			id:   operationScopeDeleteContext,
			inspect: func(t *testing.T, hub *ContextHubModel, scope ContextScope) {
				t.Helper()
				if !hub.confirm.IsActive() {
					t.Fatal("delete armed no confirmation")
				}
				if hub.pendingDelete != scope {
					t.Fatalf("pending delete = %v, want the frozen %v", hub.pendingDelete, scope)
				}
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			app, ctx := openFinderForActions(t)
			scope := seedCustomContext(t, app, "Focus")
			sheet := openFinderScopeSheet(t, app, ctx, scope)

			result := finderSheetActivate(t, sheet, testCase.id)
			if result.Result.Err != nil {
				t.Fatalf("%s failed: %v", testCase.id, result.Result.Err)
			}
			handled, cmd := app.handleOperationOverlayMessage(result)
			if !handled {
				t.Fatal("the shell did not route the sheet result")
			}
			completion, ok := cmd().(ContextActionSheetCompletedMsg)
			if !ok {
				t.Fatalf("%s produced %T, want ContextActionSheetCompletedMsg", testCase.id, cmd())
			}
			if handled, _ = app.handleSearchMessages(completion); !handled {
				t.Fatal("ContextActionSheetCompletedMsg was not handled")
			}
			for _, msg := range collectFinderCmdMsgs(completion.Cmd) {
				inline, isInline := msg.(scopeHubInlineActionMsg)
				if !isInline {
					continue
				}
				if handled, _ = app.handleContextMessages(inline); !handled {
					t.Fatal("scopeHubInlineActionMsg was not handled")
				}
			}

			if app.manageHub == nil {
				t.Fatal("the custom-scope action did not open the Scope Hub")
			}
			hub, ok := app.manageHub.ActiveModuleView().(*ContextHubModel)
			if !ok {
				t.Fatalf("active module = %T, want *ContextHubModel", app.manageHub.ActiveModuleView())
			}
			if item, found := hub.surface.CurrentItem(); !found ||
				item.ID != contextScopeListItemID(scope) {
				t.Fatalf("hub cursor = %v/%v, want the frozen %v", item.ID, found, scope)
			}
			testCase.inspect(t, hub, scope)
		})
	}
}

// finderChildRoutes are the globally supported routes a user can open the
// Finder from without reaching the Outliner first. Each one freezes a different
// presentation into the Finder context, which is exactly what a row sheet must
// not inherit when the row names its own entity.
func finderChildRoutes(t *testing.T) []struct {
	name string
	open func(*UITestDriver) tea.Msg
} {
	t.Helper()
	return []struct {
		name string
		open func(*UITestDriver) tea.Msg
	}{
		{
			name: "daily review",
			open: func(*UITestDriver) tea.Msg { return ShowReviewHubMsg{Module: ReviewDaily} },
		},
		{
			name: "settings",
			open: func(*UITestDriver) tea.Msg {
				return ShowManageHubMsg{Params: ManageParams{Module: ManageSettings, Scope: AppEntityScope()}}
			},
		},
		{
			name: "task detail",
			open: func(driver *UITestDriver) tea.Msg {
				t.Helper()
				app := driver.App()
				for _, row := range app.crossOutliner.rows {
					if !row.isTaskRow() {
						continue
					}
					project := testProjectByID(t, app.runtime.ActiveProjects(), row.ProjectID)
					return ShowTaskDetailMsg{Task: row.task, Project: project}
				}
				t.Fatal("the fixture Outliner has no task row to open in detail")
				return nil
			},
		},
	}
}

// openFinderFromRoute reaches one route and then opens the Finder over it, so
// the returned context is the one a real Tab press would carry into a sheet.
func openFinderFromRoute(t *testing.T, open func(*UITestDriver) tea.Msg) (*UITestDriver, *AppModel, CommandContext) {
	t.Helper()
	driver, workspace, err := NewFixtureDriver(FixtureRich, 100, 35)
	if err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })
	app := driver.App()
	if open != nil {
		driver.dispatch(open(driver))
		driver.drain()
	}
	driver.dispatch(ShowFinderMsg{})
	driver.drain()
	discovery, ok := app.views.Get(ViewFinder).(*DiscoveryListModel)
	if !ok {
		t.Fatalf("Finder view = %T, want *DiscoveryListModel", app.views.Get(ViewFinder))
	}
	return driver, app, cloneCommandContext(discovery.context)
}

func finderRowSheetOperations(t *testing.T, app *AppModel, item ListItem, ctx CommandContext) []OperationID {
	t.Helper()
	app.contextActionSheet = nil
	handled, _ := app.handleSearchMessages(ShowFinderContextActionsMsg{Item: item, Context: ctx})
	if !handled {
		t.Fatal("ShowFinderContextActionsMsg was not handled")
	}
	if app.contextActionSheet == nil {
		return nil
	}
	return app.contextActionSheet.operationIDs()
}

// TestFinderTaskActionsMatchOutlinerFromDailyReviewSettingsAndDetailRoutes pins
// the rule that a task row's actions depend on the task, never on the route the
// Finder happened to be opened from.
func TestFinderTaskActionsMatchOutlinerFromDailyReviewSettingsAndDetailRoutes(t *testing.T) {
	_, rootApp, rootCtx := openFinderFromRoute(t, nil)
	rootRow := finderFirstTaskRow(t, rootApp, rootCtx)
	want := finderRowSheetOperations(t, rootApp, rootRow.item, rootCtx)
	if len(want) == 0 {
		t.Fatal("the Outliner route offered no task actions")
	}

	for _, route := range finderChildRoutes(t) {
		t.Run(route.name, func(t *testing.T) {
			_, app, ctx := openFinderFromRoute(t, route.open)
			if ctx.View == ViewCrossProjectOutliner {
				t.Fatalf("route %s froze the Outliner presentation, so it cannot prove the repair", route.name)
			}
			row := finderFirstTaskRow(t, app, ctx)
			got := finderRowSheetOperations(t, app, row.item, ctx)
			if len(got) != len(want) {
				t.Fatalf("task actions from %s = %v, want the Outliner set %v", route.name, got, want)
			}
			for index, id := range want {
				if got[index] != id {
					t.Fatalf("task action %d from %s = %s, want %s", index, route.name, got[index], id)
				}
			}
			if view := app.contextActionSheet.context.View; view != ViewCrossProjectOutliner {
				t.Fatalf("frozen sheet view = %v, want the Outliner presentation", view)
			}
			if !app.contextActionSheet.context.Capabilities.Has(CapabilitySimplifiedInput) {
				t.Fatal("the frozen task sheet lost the simplified input capability")
			}
		})
	}
}

// TestFinderTagFilterIsWithheldOnDailyReviewAndSettingsRoutes proves the tag
// filter is advertised only where it lands, instead of executing against a
// destination that owns no tag filter.
func TestFinderTagFilterIsWithheldOnDailyReviewAndSettingsRoutes(t *testing.T) {
	tagRow := finderTagRowItem(t)

	_, rootApp, rootCtx := openFinderFromRoute(t, nil)
	if !slices.Contains(finderRowSheetOperations(t, rootApp, tagRow, rootCtx), operationTagFilter) {
		t.Fatal("the Outliner route withheld the tag filter, which it supports")
	}

	for _, route := range finderChildRoutes(t) {
		t.Run(route.name, func(t *testing.T) {
			_, app, ctx := openFinderFromRoute(t, route.open)
			if tagFilterAppliesTo(ctx.View) {
				t.Fatalf("route %s owns a tag filter, so it cannot prove the repair", route.name)
			}
			got := finderRowSheetOperations(t, app, tagRow, ctx)
			if slices.Contains(got, operationTagFilter) {
				t.Fatalf("tag actions from %s advertise a filter that cannot land: %v", route.name, got)
			}
			if len(got) == 0 {
				t.Fatalf("route %s dropped every tag action, want the view-independent ones", route.name)
			}
		})
	}
}

// TestFinderTagFilterFromOutlinerRouteAppliesToTheOutliner covers the other half
// of the same rule: where the filter is advertised, it must really apply.
func TestFinderTagFilterFromOutlinerRouteAppliesToTheOutliner(t *testing.T) {
	_, app, ctx := openFinderFromRoute(t, nil)
	tagRow := finderTagRowItem(t)
	if finderRowSheetOperations(t, app, tagRow, ctx) == nil {
		t.Fatal("the tag row offered no actions")
	}
	sheet := app.contextActionSheet

	result := finderSheetActivate(t, sheet, operationTagFilter)
	if result.Result.Err != nil {
		t.Fatalf("tag filter failed: %v", result.Result.Err)
	}
	handled, cmd := app.handleOperationOverlayMessage(result)
	if !handled {
		t.Fatal("the shell did not route the sheet result")
	}
	completion, ok := cmd().(ContextActionSheetCompletedMsg)
	if !ok {
		t.Fatalf("tag filter produced %T, want ContextActionSheetCompletedMsg", cmd())
	}
	if handled, _ = app.handleSearchMessages(completion); !handled {
		t.Fatal("ContextActionSheetCompletedMsg was not handled")
	}
	applied := false
	for _, msg := range collectFinderCmdMsgs(completion.Cmd) {
		filter, isFilter := msg.(ApplyTagFilterMsg)
		if !isFilter {
			continue
		}
		applied = true
		if filter.Source != ViewCrossProjectOutliner {
			t.Fatalf("tag filter source = %v, want a view that owns a tag filter", filter.Source)
		}
		if handled, _ = app.handleTagMessages(filter); !handled {
			t.Fatal("ApplyTagFilterMsg was not handled")
		}
	}
	if !applied {
		t.Fatal("the tag filter released no ApplyTagFilterMsg")
	}
	if !app.crossOutliner.filter.Tags["urgent"] {
		t.Fatalf("Outliner tag filter = %v, want the frozen tag applied", app.crossOutliner.filter.Tags)
	}
}

// TestFinderTaskSheetCancelFromSettingsPreservesQueryAndRoute keeps the sheet a
// pure overlay: cancelling it must return the user to the same Finder.
func TestFinderTaskSheetCancelFromSettingsPreservesQueryAndRoute(t *testing.T) {
	driver, app, ctx := openFinderFromRoute(t, func(*UITestDriver) tea.Msg {
		return ShowManageHubMsg{Params: ManageParams{Module: ManageSettings, Scope: AppEntityScope()}}
	})
	discovery, ok := app.views.Get(ViewFinder).(*DiscoveryListModel)
	if !ok {
		t.Fatalf("Finder view = %T, want *DiscoveryListModel", app.views.Get(ViewFinder))
	}
	for _, key := range "task" {
		discovery.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	}
	wantQuery := discovery.searchInput.Value()
	if wantQuery == "" {
		t.Fatal("the Finder recorded no query to preserve")
	}
	wantStack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	row := finderFirstTaskRow(t, app, ctx)
	if finderRowSheetOperations(t, app, row.item, ctx) == nil {
		t.Fatal("the task row offered no actions to cancel")
	}
	consumed, _ := app.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !consumed {
		t.Fatal("the open sheet did not consume Esc")
	}
	if app.contextActionSheet != nil {
		t.Fatal("Esc left the contextual sheet open")
	}
	if got := discovery.searchInput.Value(); got != wantQuery {
		t.Fatalf("Finder query after cancel = %q, want %q", got, wantQuery)
	}
	after := driver.Observe(ObserveOptions{})
	if after.CurrentView != ViewFinder.String() {
		t.Fatalf("current view after cancel = %q, want the Finder", after.CurrentView)
	}
	if !slices.Equal(after.ViewStack, wantStack) {
		t.Fatalf("view stack after cancel = %v, want %v", after.ViewStack, wantStack)
	}
}

// finderFixtureProject returns the project the fixture workspace seeded, which
// owns the notes and tags the rename tests act on.
func finderFixtureProject(t *testing.T, model *AppModel) *models.Project {
	t.Helper()
	project := model.projectByID(model.runtime.Machine().CurrentProject)
	if project == nil {
		t.Fatal("the fixture workspace has no current project")
	}
	return project
}

// finderNoteRow writes one real note into the fixture project and returns the
// row a ranked note result carries, so a rename acts on a note that exists.
func finderNoteRow(t *testing.T, model *AppModel, title string) (*models.Project, ListItem) {
	t.Helper()
	project := finderFixtureProject(t, model)
	note := models.NewNote(title)
	note.Body = title + " body"
	if err := storage.SaveNote(project, note); err != nil {
		t.Fatalf("seed note %q: %v", title, err)
	}
	items, err := (FinderNoteProvider{Notes: []FinderNote{{Note: note, Project: project}}}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("note items = %d, want 1", len(items))
	}
	return project, items[0]
}

// finderGlobalNoteRow writes one real workspace scratch note and returns the row
// the same provider builds for a note that names no project.
func finderGlobalNoteRow(t *testing.T, model *AppModel, title string) ListItem {
	t.Helper()
	note := models.NewNote(title)
	note.Body = title + " body"
	if err := storage.SaveNoteTo(storage.GlobalNotesDir(model.runtime.DataDir()), note); err != nil {
		t.Fatalf("seed global note %q: %v", title, err)
	}
	items, err := (FinderNoteProvider{Notes: []FinderNote{{Note: note}}}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("global note items = %d, want 1", len(items))
	}
	return items[0]
}

func finderNoteTitles(t *testing.T, project *models.Project) []string {
	t.Helper()
	notes, err := storage.ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	titles := make([]string, 0, len(notes))
	for _, note := range notes {
		titles = append(titles, note.Title)
	}
	return titles
}

// openFinderRenamePrompt opens one row's sheet, activates the named rename, and
// routes the pending value step the way the shell does.
func openFinderRenamePrompt(
	t *testing.T,
	model *AppModel,
	item ListItem,
	ctx CommandContext,
	id OperationID,
) *OperationTextPromptModel {
	t.Helper()
	if !slices.Contains(finderRowSheetOperations(t, model, item, ctx), id) {
		t.Fatalf("the row withheld %s", id)
	}
	result := finderSheetActivate(t, model.contextActionSheet, id)
	if result.Result.ValueRequired == nil {
		t.Fatalf("%s produced %#v, want a pending value step", id, result.Result)
	}
	if handled, _ := model.handleOperationOverlayMessage(result); !handled {
		t.Fatal("the shell did not route the sheet result")
	}
	if model.contextActionSheet == nil {
		t.Fatalf("%s closed the sheet its prompt must return to", id)
	}
	prompt := model.operationTextPrompt
	if prompt == nil {
		t.Fatalf("%s opened no text prompt", id)
	}
	return prompt
}

func clearOperationTextPrompt(t *testing.T, model *AppModel) {
	t.Helper()
	prompt := model.operationTextPrompt
	if prompt == nil {
		t.Fatal("no text prompt is open")
	}
	for range []rune(prompt.input.Value()) {
		if consumed, _ := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyBackspace}); !consumed {
			t.Fatal("the text prompt did not consume Backspace")
		}
	}
	if got := prompt.input.Value(); got != "" {
		t.Fatalf("prompt value after clearing = %q, want it empty", got)
	}
}

// submitOperationTextPrompt replaces the prefilled name and confirms it through
// the shell's overlay dispatch, returning the messages the prompt released.
func submitOperationTextPrompt(t *testing.T, model *AppModel, value string) []tea.Msg {
	t.Helper()
	clearOperationTextPrompt(t, model)
	for _, char := range value {
		consumed, _ := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: char, Text: string(char)})
		if !consumed {
			t.Fatalf("the text prompt did not consume %q", char)
		}
	}
	if got := model.operationTextPrompt.input.Value(); got != value {
		t.Fatalf("prompt value = %q, want %q", got, value)
	}
	consumed, cmd := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !consumed {
		t.Fatal("the text prompt did not consume Enter")
	}
	if model.operationTextPrompt != nil {
		t.Fatal("Enter left the text prompt open")
	}
	return collectFinderCmdMsgs(cmd)
}

// resumeRenameFromPrompt confirms the prompt and hands the typed value back to
// the shell, returning whatever the resumed operation produced.
func resumeRenameFromPrompt(t *testing.T, model *AppModel, value string) tea.Cmd {
	t.Helper()
	msgs := submitOperationTextPrompt(t, model, value)
	if len(msgs) != 1 {
		t.Fatalf("the prompt released %v, want one selected value", msgs)
	}
	selected, ok := msgs[0].(OperationValueSelectedMsg)
	if !ok {
		t.Fatalf("the prompt released %T, want OperationValueSelectedMsg", msgs[0])
	}
	if selected.Value != value {
		t.Fatalf("selected value = %v, want %q", selected.Value, value)
	}
	handled, cmd := model.handleOperationOverlayMessage(selected)
	if !handled {
		t.Fatal("the shell did not route the selected value")
	}
	return cmd
}

// TestFinderRenameNotePromptsForATitleAndAppliesIt covers the whole repaired
// path: the advertised rename opens a prompt, the typed title reaches the
// mutation, and the completion closes the Finder exactly once.
func TestFinderRenameNotePromptsForATitleAndAppliesIt(t *testing.T) {
	driver, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")
	stack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	prompt := openFinderRenamePrompt(t, model, row, ctx, operationNoteRename)
	if prompt.input.Value() != "Release checklist" {
		t.Fatalf("prompt prefill = %q, want the current title", prompt.input.Value())
	}

	cmd := resumeRenameFromPrompt(t, model, "Release checklist v2")
	if model.contextActionSheet != nil {
		t.Fatal("a completed rename left the sheet open")
	}
	completion, ok := cmd().(ContextActionSheetCompletedMsg)
	if !ok {
		t.Fatalf("the resumed rename produced %T, want ContextActionSheetCompletedMsg", cmd())
	}
	if handled, _ := model.handleSearchMessages(completion); !handled {
		t.Fatal("ContextActionSheetCompletedMsg was not handled")
	}

	after := driver.Observe(ObserveOptions{}).ViewStack
	if !slices.Equal(after, stack[:len(stack)-1]) {
		t.Fatalf("view stack after rename = %v, want the Finder popped once from %v", after, stack)
	}

	committed := false
	for _, msg := range collectFinderCmdMsgs(completion.Cmd) {
		switch typed := msg.(type) {
		case ProjectNoteMutationResultMsg:
			committed = true
		case ProjectNoteMutationErrorMsg:
			t.Fatalf("note rename failed: %v", typed.Err)
		}
	}
	if !committed {
		t.Fatal("the rename released no note mutation")
	}
	if titles := finderNoteTitles(t, project); !slices.Contains(titles, "Release checklist v2") {
		t.Fatalf("note titles after rename = %v, want the new title", titles)
	}
}

// TestFinderRenameNoteRejectsABlankTitle keeps the prompt the place a name is
// validated, instead of letting an empty value reach the handler.
func TestFinderRenameNoteRejectsABlankTitle(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")

	openFinderRenamePrompt(t, model, row, ctx, operationNoteRename)
	clearOperationTextPrompt(t, model)
	consumed, cmd := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !consumed {
		t.Fatal("the text prompt did not consume Enter")
	}
	if msgs := collectFinderCmdMsgs(cmd); len(msgs) != 0 {
		t.Fatalf("a blank title released %v, want nothing", msgs)
	}
	if model.operationTextPrompt == nil {
		t.Fatal("a blank title closed the prompt")
	}
	if model.operationTextPrompt.notice == "" {
		t.Fatal("a blank title reported nothing to the user")
	}
	if titles := finderNoteTitles(t, project); !slices.Contains(titles, "Release checklist") {
		t.Fatalf("note titles = %v, want the original title untouched", titles)
	}
}

// TestFinderRenameNoteCancelReturnsToTheSameSheet keeps the prompt a pure nested
// step: Esc must return to the row's sheet over the same Finder.
func TestFinderRenameNoteCancelReturnsToTheSameSheet(t *testing.T) {
	driver, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")
	stack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	openFinderRenamePrompt(t, model, row, ctx, operationNoteRename)
	consumed, cmd := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEsc})
	if !consumed {
		t.Fatal("the text prompt did not consume Esc")
	}
	msgs := collectFinderCmdMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("Esc released %v, want one cancellation", msgs)
	}
	if _, ok := msgs[0].(OperationValueCanceledMsg); !ok {
		t.Fatalf("Esc released %T, want OperationValueCanceledMsg", msgs[0])
	}
	if handled, _ := model.handleOperationOverlayMessage(msgs[0]); !handled {
		t.Fatal("OperationValueCanceledMsg was not handled")
	}

	if model.operationTextPrompt != nil {
		t.Fatal("Esc left the text prompt open")
	}
	if model.contextActionSheet == nil {
		t.Fatal("cancelling the prompt also closed the row sheet")
	}
	if !slices.Contains(model.contextActionSheet.operationIDs(), operationNoteRename) {
		t.Fatal("the sheet the prompt returned to no longer offers the rename")
	}
	if after := driver.Observe(ObserveOptions{}).ViewStack; !slices.Equal(after, stack) {
		t.Fatalf("view stack after cancel = %v, want %v", after, stack)
	}
	if titles := finderNoteTitles(t, project); !slices.Contains(titles, "Release checklist") {
		t.Fatalf("note titles = %v, want the original title untouched", titles)
	}
}

// TestFinderRenameNoteFailsOnAStaleTarget proves the frozen identity is
// revalidated after the prompt, not trusted from when the row was built.
func TestFinderRenameNoteFailsOnAStaleTarget(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")

	openFinderRenamePrompt(t, model, row, ctx, operationNoteRename)
	notes, err := storage.ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	for _, note := range notes {
		if note.Title == "Release checklist" {
			if err := storage.DeleteNote(project, note.Filename); err != nil {
				t.Fatalf("DeleteNote: %v", err)
			}
		}
	}

	if cmd := resumeRenameFromPrompt(t, model, "Release checklist v2"); cmd != nil {
		t.Fatalf("a stale rename released %#v, want no completion", cmd())
	}
	if model.contextActionSheet == nil {
		t.Fatal("a failed rename closed the sheet that reports it")
	}
	if model.contextActionSheet.notice == "" {
		t.Fatal("a stale rename reported nothing to the user")
	}
	if titles := finderNoteTitles(t, project); slices.Contains(titles, "Release checklist v2") {
		t.Fatalf("note titles = %v, want no note written for a stale target", titles)
	}
}

// TestFinderGlobalNoteSheetWithholdsTheTaskLinkProjectNotesKeep separates the two
// note scopes a Finder note row can name. A link names a task inside the note's
// own project, so a workspace scratch note must not advertise it while still
// offering everything a scratch note can really do.
func TestFinderGlobalNoteSheetWithholdsTheTaskLinkProjectNotesKeep(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	_, projectRow := finderNoteRow(t, model, "Release checklist")
	globalRow := finderGlobalNoteRow(t, model, "Scratch pad")

	projectActions := finderRowSheetOperations(t, model, projectRow, ctx)
	if !slices.Contains(projectActions, operationNoteLink) {
		t.Fatalf("project note actions = %v, want the task link", projectActions)
	}
	if slices.Contains(projectActions, operationNoteMove) {
		t.Fatalf("project note actions = %v, want no move-to-project", projectActions)
	}

	globalActions := finderRowSheetOperations(t, model, globalRow, ctx)
	if slices.Contains(globalActions, operationNoteLink) {
		t.Fatalf("global note actions = %v, want no task link", globalActions)
	}
	if !slices.Contains(globalActions, operationNoteMove) {
		t.Fatalf("global note actions = %v, want the move-to-project action", globalActions)
	}
	for _, shared := range []OperationID{operationNoteEdit, operationNoteTags, operationNoteDelete} {
		if !slices.Contains(globalActions, shared) {
			t.Fatalf("global note actions = %v, want the shared action %s", globalActions, shared)
		}
	}
}

// TestFinderGlobalNoteLinkFailsVisiblyWhenForced proves the withheld action is
// also refused at execution, and that the linker route itself no longer accepts
// a projectless request by quietly doing nothing.
func TestFinderGlobalNoteLinkFailsVisiblyWhenForced(t *testing.T) {
	driver, model, ctx := openFinderFromRoute(t, nil)
	globalRow := finderGlobalNoteRow(t, model, "Scratch pad")
	stack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	target, ok := globalRow.Data.(FinderTarget)
	if !ok {
		t.Fatalf("global note row data = %T, want FinderTarget", globalRow.Data)
	}
	frozen, ok := finderContextActionTarget(ctx, target)
	if !ok {
		t.Fatal("the global note row froze no context")
	}
	forced := model.operationExecutor.Execute(OperationExecutionRequest{
		ID:      operationNoteLink,
		Context: frozen.context,
		Source:  OperationSourceActionSheet,
	})
	if forced.Err == nil {
		t.Fatalf("a forced global-note link produced %#v, want a visible error", forced)
	}

	handled, cmd := model.handleNotesMessages(ShowNoteTaskLinkerMsg{Note: target.Note})
	if !handled {
		t.Fatal("ShowNoteTaskLinkerMsg was not handled")
	}
	msgs := collectFinderCmdMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("a projectless linker request released %v, want one error", msgs)
	}
	if _, isError := msgs[0].(ErrorMsg); !isError {
		t.Fatalf("a projectless linker request released %T, want ErrorMsg", msgs[0])
	}
	if after := driver.Observe(ObserveOptions{}).ViewStack; !slices.Equal(after, stack) {
		t.Fatalf("view stack = %v, want the Finder untouched at %v", after, stack)
	}
}

// TestFinderProjectNoteLinkOpensTheLinkerOnTheFrozenRow keeps the working half
// working: the row's own note and project reach the project-backed linker.
func TestFinderProjectNoteLinkOpensTheLinkerOnTheFrozenRow(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")

	if !slices.Contains(finderRowSheetOperations(t, model, row, ctx), operationNoteLink) {
		t.Fatal("the project note row withheld the task link")
	}
	result := finderSheetActivate(t, model.contextActionSheet, operationNoteLink)
	linker, ok := result.Result.Message.(ShowNoteTaskLinkerMsg)
	if !ok {
		t.Fatalf("the task link produced %#v, want ShowNoteTaskLinkerMsg", result.Result)
	}
	if linker.Project == nil || linker.Project.ID != project.ID {
		t.Fatalf("linker project = %#v, want %s", linker.Project, project.ID)
	}
	if linker.Note == nil || linker.Note.Title != "Release checklist" {
		t.Fatalf("linker note = %#v, want the frozen row's note", linker.Note)
	}

	if handled, _ := model.handleNotesMessages(linker); !handled {
		t.Fatal("ShowNoteTaskLinkerMsg was not handled")
	}
	if view := model.currentView(); view != ViewNoteTaskLinker {
		t.Fatalf("current view = %s, want %s", view, ViewNoteTaskLinker)
	}
	opened, ok := model.views.Get(ViewNoteTaskLinker).(*NoteTaskLinkerModel)
	if !ok {
		t.Fatalf("linker view = %T, want *NoteTaskLinkerModel", model.views.Get(ViewNoteTaskLinker))
	}
	if opened.project == nil || opened.project.ID != project.ID || opened.note.Title != "Release checklist" {
		t.Fatalf("linker opened on %#v / %#v, want the frozen row", opened.project, opened.note)
	}
}

// TestFinderProjectNoteLinkFailsOnAStaleNote proves the link revalidates the
// frozen identity and reports the failure on its own sheet instead of unwinding
// the Finder onto an unrelated route.
func TestFinderProjectNoteLinkFailsOnAStaleNote(t *testing.T) {
	driver, model, ctx := openFinderFromRoute(t, nil)
	project, row := finderNoteRow(t, model, "Release checklist")
	stack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

	if !slices.Contains(finderRowSheetOperations(t, model, row, ctx), operationNoteLink) {
		t.Fatal("the project note row withheld the task link")
	}
	notes, err := storage.ListNotes(project)
	if err != nil {
		t.Fatalf("ListNotes: %v", err)
	}
	for _, note := range notes {
		if note.Title == "Release checklist" {
			if err := storage.DeleteNote(project, note.Filename); err != nil {
				t.Fatalf("DeleteNote: %v", err)
			}
		}
	}

	result := finderSheetActivate(t, model.contextActionSheet, operationNoteLink)
	if result.Result.Err == nil {
		t.Fatalf("a stale link produced %#v, want a visible error", result.Result)
	}
	if handled, _ := model.handleOperationOverlayMessage(result); !handled {
		t.Fatal("the shell did not route the sheet result")
	}
	if model.contextActionSheet == nil {
		t.Fatal("a failed link closed the sheet that reports it")
	}
	if model.contextActionSheet.notice == "" {
		t.Fatal("a stale link reported nothing to the user")
	}
	if after := driver.Observe(ObserveOptions{}).ViewStack; !slices.Equal(after, stack) {
		t.Fatalf("view stack = %v, want the Finder untouched at %v", after, stack)
	}
}

// stubNoteClipboardAndReference replaces the clipboard and external-editor
// seams the note actions use, so a test can observe what they report without
// writing to the real clipboard or launching an editor.
func stubNoteClipboardAndReference(t *testing.T) {
	t.Helper()
	title, body, both := copyNoteTitleToClipboard, copyNoteBodyToClipboard, copyNoteTitleAndBodyToClipboard
	create, open := createReferenceSnapshotFile, openReferenceFile
	t.Cleanup(func() {
		copyNoteTitleToClipboard, copyNoteBodyToClipboard, copyNoteTitleAndBodyToClipboard = title, body, both
		createReferenceSnapshotFile, openReferenceFile = create, open
	})
	copied := func(note *models.Note) (string, error) { return note.Title, nil }
	copyNoteTitleToClipboard = copied
	copyNoteBodyToClipboard = copied
	copyNoteTitleAndBodyToClipboard = copied
	snapshot := filepath.Join(t.TempDir(), "reference.md")
	createReferenceSnapshotFile = func(string, string, string) (string, error) { return snapshot, nil }
	openReferenceFile = func(editor string, path string) (external.ReferenceOpenResult, error) {
		return external.ReferenceOpenResult{
			Status: external.ReferenceOpenStatusOpened,
			Editor: editor,
			Path:   path,
		}, nil
	}
}

// finderSheetActivateThroughShell runs one sheet row through the shell's overlay
// key path, so the sheet's own dismissal is observed the way a real keypress
// causes it rather than assumed.
func finderSheetActivateThroughShell(t *testing.T, model *AppModel, id OperationID) ContextActionSheetResultMsg {
	t.Helper()
	if model.contextActionSheet == nil {
		t.Fatalf("no sheet is open for %s", id)
	}
	if !model.contextActionSheet.surface.SetCursorToID(ListItemID("finder-action:" + string(id))) {
		t.Fatalf("the sheet has no row for %s", id)
	}
	consumed, cmd := model.handleActiveKeyOverlay(tea.KeyPressMsg{Code: tea.KeyEnter})
	if !consumed {
		t.Fatalf("the sheet did not consume Enter on %s", id)
	}
	msgs := collectFinderCmdMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("Enter on %s released %v, want one sheet result", id, msgs)
	}
	result, ok := msgs[0].(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("Enter on %s released %T, want ContextActionSheetResultMsg", id, msgs[0])
	}
	return result
}

// TestFinderNoteCopyAndReferenceReportSuccessAndCloseTheFinder covers the note
// actions whose whole outcome is a status. Each one must still tell the user it
// worked, exactly once, while the Finder unwinds the way every completed row
// action unwinds it.
func TestFinderNoteCopyAndReferenceReportSuccessAndCloseTheFinder(t *testing.T) {
	cases := []struct {
		id     OperationID
		status string
	}{
		{operationNoteCopyTitle, "Copied title to clipboard"},
		{operationNoteCopyBody, "Copied body to clipboard"},
		{operationNoteCopyTitleBody, "Copied title and body to clipboard"},
		{operationNoteReference, "Opened note reference copy"},
	}
	for _, tc := range cases {
		t.Run(string(tc.id), func(t *testing.T) {
			stubNoteClipboardAndReference(t)
			driver, model, ctx := openFinderFromRoute(t, nil)
			_, row := finderNoteRow(t, model, "Release checklist")
			stack := slices.Clone(driver.Observe(ObserveOptions{}).ViewStack)

			if !slices.Contains(finderRowSheetOperations(t, model, row, ctx), tc.id) {
				t.Fatalf("the note row withheld %s", tc.id)
			}
			result := finderSheetActivateThroughShell(t, model, tc.id)
			if result.Result.Err != nil {
				t.Fatalf("%s failed: %v", tc.id, result.Result.Err)
			}
			if result.Result.Message != nil {
				t.Fatalf("%s produced %#v, want a status-only outcome", tc.id, result.Result.Message)
			}
			if result.Result.Status != tc.status {
				t.Fatalf("%s status = %q, want %q", tc.id, result.Result.Status, tc.status)
			}
			if model.contextActionSheet != nil {
				t.Fatalf("a completed %s left the sheet open", tc.id)
			}

			handled, cmd := model.handleOperationOverlayMessage(result)
			if !handled {
				t.Fatal("the shell did not route the sheet result")
			}
			msgs := collectFinderCmdMsgs(cmd)
			if len(msgs) != 1 {
				t.Fatalf("%s released %v, want one host completion", tc.id, msgs)
			}
			completion, ok := msgs[0].(ContextActionSheetCompletedMsg)
			if !ok {
				t.Fatalf("%s released %T, want ContextActionSheetCompletedMsg", tc.id, msgs[0])
			}
			handled, closing := model.handleSearchMessages(completion)
			if !handled {
				t.Fatal("ContextActionSheetCompletedMsg was not handled")
			}

			after := driver.Observe(ObserveOptions{}).ViewStack
			if !slices.Equal(after, stack[:len(stack)-1]) {
				t.Fatalf("view stack after %s = %v, want the Finder popped once from %v", tc.id, after, stack)
			}
			reported := make([]string, 0, 1)
			for _, msg := range collectFinderCmdMsgs(closing) {
				switch typed := msg.(type) {
				case StatusMsg:
					reported = append(reported, string(typed))
				case InfoStatusMsg:
					reported = append(reported, string(typed))
				case ErrorMsg:
					t.Fatalf("%s reported an error: %v", tc.id, typed.Err)
				}
			}
			if len(reported) != 1 || reported[0] != tc.status {
				t.Fatalf("%s reported %v, want exactly one %q", tc.id, reported, tc.status)
			}
		})
	}
}

// TestFinderRenameWorkspaceTagPromptsAndRenamesEverywhere covers the aggregate
// tag scope a Finder tag row names.
func TestFinderRenameWorkspaceTagPromptsAndRenamesEverywhere(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	prompt := openFinderRenamePrompt(t, model, finderTagRowItem(t), ctx, operationTagRenameWorkspace)
	if prompt.input.Value() != "urgent" {
		t.Fatalf("prompt prefill = %q, want the current tag name", prompt.input.Value())
	}

	cmd := resumeRenameFromPrompt(t, model, "critical")
	completion, ok := cmd().(ContextActionSheetCompletedMsg)
	if !ok {
		t.Fatalf("the resumed rename produced %T, want ContextActionSheetCompletedMsg", cmd())
	}
	if handled, _ := model.handleSearchMessages(completion); !handled {
		t.Fatal("ContextActionSheetCompletedMsg was not handled")
	}
	renamed := false
	for _, msg := range collectFinderCmdMsgs(completion.Cmd) {
		result, isRename := msg.(GlobalTagRenamedMsg)
		if !isRename {
			continue
		}
		renamed = true
		if result.OldName != "urgent" || result.NewName != "critical" {
			t.Fatalf("workspace rename = %+v, want urgent -> critical", result)
		}
		if result.ProjectCount == 0 || result.TaskCount == 0 {
			t.Fatalf("workspace rename touched %+v, want at least one project and task", result)
		}
	}
	if !renamed {
		t.Fatal("the workspace tag rename released no result")
	}
}

// TestFinderRenameProjectTagPromptsAndRenamesInsideItsProject covers the
// project-scoped tag identity. No production row emits one today, so the test
// drives the same executor seam a project-scoped tag row would reach.
func TestFinderRenameProjectTagPromptsAndRenamesInsideItsProject(t *testing.T) {
	_, model, _ := openFinderFromRoute(t, nil)
	project := finderFixtureProject(t, model)
	ctx := PresentationCommandContext(ViewCrossProjectOutliner)
	ctx.CurrentEntity = ProjectTagEntity(project.ID, "urgent")

	model.executeOperationOverlayRequest(OperationExecutionRequest{
		ID:      operationTagRenameProject,
		Context: ctx,
		Source:  OperationSourceFinder,
	})
	if model.operationTextPrompt == nil {
		t.Fatal("the project tag rename opened no text prompt")
	}
	if got := model.operationTextPrompt.input.Value(); got != "urgent" {
		t.Fatalf("prompt prefill = %q, want the current tag name", got)
	}

	cmd := resumeRenameFromPrompt(t, model, "critical")
	if cmd == nil {
		t.Fatal("the resumed project tag rename released no command")
	}
	result, ok := cmd().(TagRenamedMsg)
	if !ok {
		t.Fatalf("the resumed rename produced %T, want TagRenamedMsg", cmd())
	}
	if result.OldName != "urgent" || result.NewName != "critical" {
		t.Fatalf("project rename = %+v, want urgent -> critical", result)
	}
}

// TestFinderRenameTagFailsOnAStaleTarget proves a tag that stopped existing
// while the prompt was open fails instead of renaming something else.
func TestFinderRenameTagFailsOnAStaleTarget(t *testing.T) {
	_, model, ctx := openFinderFromRoute(t, nil)
	openFinderRenamePrompt(t, model, finderTagRowItem(t), ctx, operationTagRenameWorkspace)

	for _, project := range model.activeProjects() {
		project.Tags = nil
		for _, task := range project.Tasks {
			task.Tags = nil
		}
	}

	if cmd := resumeRenameFromPrompt(t, model, "critical"); cmd != nil {
		t.Fatalf("a stale rename released %#v, want no completion", cmd())
	}
	if model.contextActionSheet == nil {
		t.Fatal("a failed rename closed the sheet that reports it")
	}
	if model.contextActionSheet.notice == "" {
		t.Fatal("a stale rename reported nothing to the user")
	}
}

// finderProjectRow builds the row the production provider gives one project, so
// a lifecycle test reads the same manifest a ranked Finder result carries.
func finderProjectRow(t *testing.T, project *models.Project) ListItem {
	t.Helper()
	items, err := (FinderProjectProvider{Projects: []*models.Project{project}}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("project items = %d, want 1", len(items))
	}
	if len(items[0].OperationIDs) != len(projectOperationDescriptors()) {
		t.Fatalf("the row advertises %d actions, want the whole project catalog",
			len(items[0].OperationIDs))
	}
	return items[0]
}

// finderProjectLifecycleApp seeds one project for every lifecycle and
// protection state a Finder project row can name.
func finderProjectLifecycleApp(t *testing.T) *AppModel {
	t.Helper()
	app := contextHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	sandbox := scopeTestProject("sandbox", "Sandbox", "1")
	sandbox.Sandbox = true
	addTestAppProject(app, archived)
	addTestAppProject(app, sandbox)
	addTestAppProject(app, scopeTestProject("inbox", "Inbox", "1"))

	catalog := cloneWorkspaceCatalog(app.runtime.Catalog())
	catalog.SystemInboxProjectID = "inbox"
	app.runtime.SetCatalogInMemory(catalog)
	return app
}

// TestFinderProjectSheetsMatchEachLifecycleAndProtectionState pins the exact
// action set every project state offers. An action that could only fail is a
// lie the sheet must not tell, whichever host opened it.
func TestFinderProjectSheetsMatchEachLifecycleAndProtectionState(t *testing.T) {
	app := finderProjectLifecycleApp(t)
	ctx := app.operationCommandContext()
	cases := []struct {
		name      string
		projectID string
		want      []OperationID
	}{
		{
			name:      "active",
			projectID: "beta",
			want: []OperationID{
				operationProjectOpen,
				operationScopeRenameProject,
				operationScopeStatusProject,
				operationScopePinProject,
				operationScopeArchiveProject,
				operationScopeOpenProjectNote,
				operationProjectMetrics,
				operationProjectExport,
				operationProjectOpenFolder,
				operationProjectArchiveCompleted,
			},
		},
		{
			name:      "archived",
			projectID: "archived",
			want: []OperationID{
				operationScopeRenameProject,
				operationScopeStatusProject,
				operationScopeRestoreProject,
				operationScopeOpenProjectNote,
				operationProjectMetrics,
				operationProjectExport,
				operationProjectOpenFolder,
			},
		},
		{
			name:      "sandbox",
			projectID: "sandbox",
			want: []OperationID{
				operationProjectOpen,
				operationScopeRenameProject,
				operationScopeStatusProject,
				operationScopePinProject,
				operationScopeOpenProjectNote,
				operationProjectMetrics,
				operationProjectExport,
				operationProjectOpenFolder,
				operationProjectArchiveCompleted,
			},
		},
		{
			name:      "system inbox",
			projectID: "inbox",
			want: []OperationID{
				operationProjectOpen,
				operationScopeStatusProject,
				operationScopePinProject,
				operationScopeOpenProjectNote,
				operationProjectMetrics,
				operationProjectExport,
				operationProjectOpenFolder,
				operationProjectArchiveCompleted,
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			project := app.projectByID(testCase.projectID)
			if project == nil {
				t.Fatalf("the workspace has no project %s", testCase.projectID)
			}
			got := finderRowSheetOperations(t, app, finderProjectRow(t, project), ctx)
			if !slices.Equal(got, testCase.want) {
				t.Fatalf("%s project actions = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

// TestFinderProjectSheetRereadsAProjectArchivedAfterItsRowWasBuilt proves the
// sheet derives its actions from the live project instead of the metadata the
// ranked row froze.
func TestFinderProjectSheetRereadsAProjectArchivedAfterItsRowWasBuilt(t *testing.T) {
	app := finderProjectLifecycleApp(t)
	ctx := app.operationCommandContext()
	live := app.projectByID("beta")
	stale := *live
	row := finderProjectRow(t, &stale)

	live.Archived = true
	got := finderRowSheetOperations(t, app, row, ctx)
	if slices.Contains(got, operationProjectOpen) {
		t.Fatalf("a row built while beta was active still offers Open: %v", got)
	}
	if !slices.Contains(got, operationScopeRestoreProject) {
		t.Fatalf("actions = %v, want the unarchive the live project now needs", got)
	}
}

// TestFinderProjectActionRefusedWhenArchivedBetweenSheetAndExecution keeps a
// state change during an open sheet safe: the destructive action is refused
// before the executor asks the user to confirm it.
func TestFinderProjectActionRefusedWhenArchivedBetweenSheetAndExecution(t *testing.T) {
	app := finderProjectLifecycleApp(t)
	ctx := app.operationCommandContext()
	project := app.projectByID("beta")
	if !slices.Contains(
		finderRowSheetOperations(t, app, finderProjectRow(t, project), ctx),
		operationProjectArchiveCompleted,
	) {
		t.Fatal("the active project withheld the archive-completed action")
	}
	sheet := app.contextActionSheet
	if armed := finderSheetActivate(t, sheet, operationProjectArchiveCompleted); armed.Result.ConfirmationRequired == nil {
		t.Fatalf("archive completed produced %#v, want a confirmation on an active project", armed.Result)
	}

	project.Archived = true
	stale := finderSheetActivate(t, sheet, operationProjectArchiveCompleted)
	if stale.Result.ConfirmationRequired != nil {
		t.Fatal("a stale archive-completed asked to confirm before reporting the lifecycle rejection")
	}
	if stale.Result.Err == nil {
		t.Fatalf("a stale archive-completed produced %#v, want a visible error", stale.Result)
	}
	if stale.Result.Message != nil {
		t.Fatalf("a stale archive-completed emitted %#v", stale.Result.Message)
	}
	if app.contextActionSheet == nil {
		t.Fatal("the refused action closed the sheet that reports it")
	}
}

func finderTagRowItem(t *testing.T) ListItem {
	t.Helper()
	items, err := (FinderTagProvider{Tags: []TagInfo{{Name: "urgent", TaskCount: 2}}}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("tag items = %d, want 1", len(items))
	}
	return items[0]
}

// seedCustomContext gives the workspace one custom context so the Scope Hub can
// focus the exact row a Finder scope action froze.
func seedCustomContext(t *testing.T, app *AppModel, name string) ContextScope {
	t.Helper()
	projects := app.runtime.ActiveProjects()
	if len(projects) == 0 {
		t.Fatal("the fixture workspace has no projects")
	}
	err := app.commitMachineMutation(func(machine *config.MachineState) {
		machine.SetCustomContext(name, []string{projects[0].ID})
	})
	if err != nil {
		t.Fatalf("seed custom context: %v", err)
	}
	return CustomContextScope(name)
}

func finderScopeItem(t *testing.T, scope ContextScope) ListItem {
	t.Helper()
	items, err := (FinderScopeProvider{
		Scopes: []ContextScopeSummary{{Scope: scope, ActiveProjectCount: 1}},
	}).FinderItems()
	if err != nil {
		t.Fatalf("FinderItems: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("scope items = %d, want 1", len(items))
	}
	return items[0]
}

func openFinderScopeSheet(
	t *testing.T,
	app *AppModel,
	ctx CommandContext,
	scope ContextScope,
) *ContextActionSheetModel {
	t.Helper()
	handled, _ := app.handleSearchMessages(ShowFinderContextActionsMsg{
		Item:    finderScopeItem(t, scope),
		Context: ctx,
	})
	if !handled || app.contextActionSheet == nil {
		t.Fatalf("scope %v offered no contextual actions", scope)
	}
	return app.contextActionSheet
}

// finderSheetActivate runs one named action the way Enter on its row would.
func finderSheetActivate(
	t *testing.T,
	sheet *ContextActionSheetModel,
	id OperationID,
) ContextActionSheetResultMsg {
	t.Helper()
	if !sheet.surface.SetCursorToID(ListItemID("finder-action:" + string(id))) {
		t.Fatalf("the sheet has no row for %s", id)
	}
	_, _, raw := sheet.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
	result, ok := raw.(ContextActionSheetResultMsg)
	if !ok {
		t.Fatalf("Enter on %s returned %T, want ContextActionSheetResultMsg", id, raw)
	}
	return result
}

// collectFinderCmdMsgs flattens one command into the messages it releases,
// without feeding them back through the shell.
func collectFinderCmdMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		if msg == nil {
			return nil
		}
		return []tea.Msg{msg}
	}
	msgs := make([]tea.Msg, 0, len(batch))
	for _, child := range batch {
		msgs = append(msgs, collectFinderCmdMsgs(child)...)
	}
	return msgs
}

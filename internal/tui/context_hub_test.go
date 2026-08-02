package tui

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// contextHubApp builds a typed shell whose catalog carries two project contexts
// and a live cross-project Outliner so an applied scope has somewhere to land.
// Alpha carries two tasks and Beta one so the hub's preview counts distinguish
// them.
func contextHubApp(t *testing.T) *AppModel {
	t.Helper()
	app := typedTestApp(t)
	alpha := scopeTestProject("alpha", "Alpha", "1", "2")
	alpha.Slug = "alpha"
	alpha.Context = "work"
	alpha.Path = t.TempDir()
	beta := scopeTestProject("beta", "Beta", "1")
	beta.Slug = "beta"
	beta.Context = "home"
	beta.Path = t.TempDir()
	setTestAppProjects(app, alpha, beta)
	app.currentProject = nil
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.SetSize(120, 40)
	app.crossOutliner.loadedProjects = []*models.Project{alpha, beta}
	app.crossOutliner.crossForceRebuild = true
	app.crossOutliner.rebuildRows()
	return app
}

// openContextHub opens the Manage Contexts route and returns the hosted hub
// sized wide enough for the two-pane layout.
func openContextHub(t *testing.T, app *AppModel) *ContextHubModel {
	t.Helper()
	app.handleContextMessages(ShowContextHubMsg{})
	hub := contextHubModuleView(t, app)
	hub.SetSize(120, 30)
	return hub
}

func contextHubRender(hub *ContextHubModel) string {
	return strings.Join(strings.Fields(ansi.Strip(hub.View().Content)), " ")
}

func contextHubScopes(hub *ContextHubModel) []ContextScope {
	scopes := make([]ContextScope, 0)
	if hub == nil || hub.surface == nil {
		return scopes
	}
	for _, match := range hub.surface.Matches() {
		entry, ok := match.Item.Data.(scopeHubEntry)
		if ok && entry.Kind == scopeHubEntryBase {
			scopes = append(scopes, entry.Scope)
		}
	}
	return scopes
}

func currentContextHubScope(hub *ContextHubModel) (ContextScope, bool) {
	if hub == nil || hub.surface == nil {
		return ContextScope{}, false
	}
	item, ok := hub.surface.CurrentItem()
	if !ok {
		return ContextScope{}, false
	}
	entry, ok := item.Data.(scopeHubEntry)
	if !ok || entry.Kind != scopeHubEntryBase {
		return ContextScope{}, false
	}
	return entry.Scope, true
}

func seekContextHubScope(t *testing.T, hub *ContextHubModel, target ContextScope) {
	t.Helper()
	if hub == nil || hub.surface == nil {
		t.Fatalf("cannot seek context scope %#v without a List Surface", target)
	}

	matchCount := len(hub.surface.Matches())
	for range matchCount {
		if scope, ok := currentContextHubScope(hub); ok && scope == target {
			return
		}

		before, beforeOK := hub.surface.CurrentItem()
		hub.Update(syntheticKey('j'))
		after, afterOK := hub.surface.CurrentItem()
		if beforeOK == afterOK && (!beforeOK || before.ID == after.ID) {
			break
		}
	}

	scope, scopeOK := currentContextHubScope(hub)
	item, itemOK := hub.surface.CurrentItem()
	t.Fatalf(
		"context scope %#v not found after scanning at most %d List Surface matches; cursor stopped at item %q (present=%t, scope=%#v, scope row=%t)",
		target,
		matchCount,
		item.ID,
		itemOK,
		scope,
		scopeOK,
	)
}

func typeIntoContextHub(hub *ContextHubModel, text string) {
	for _, r := range text {
		hub.Update(syntheticKey(r))
	}
}

// contextHubSaveKey is the editor's commit key. Ctrl+S never reaches the shell
// interceptors because an open editor reports HasTextInput.
var contextHubSaveKey = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}

func focusContextHubScope(t *testing.T, hub *ContextHubModel, scope ContextScope) {
	t.Helper()
	if hub.surface != nil && hub.surface.SetCursorToID(contextScopeListItemID(scope)) {
		return
	}
	t.Fatalf("context hub omitted %#v from List Surface matches %#v", scope, contextHubScopes(hub))
}

// checkEditorProject moves the editor's project cursor onto id and toggles it.
func checkEditorProject(t *testing.T, hub *ContextHubModel, id string) {
	t.Helper()
	if hub.editor == nil {
		t.Fatal("no inline editor is open")
	}
	if !hub.editor.listFocus {
		hub.Update(syntheticSpecialKey(tea.KeyTab))
	}
	for i, project := range hub.editor.filtered {
		if project.ID == id {
			hub.editor.cursor = i
			hub.Update(syntheticKey(' '))
			return
		}
	}
	t.Fatalf("editor omitted project %q", id)
}

func contextHubModuleView(t *testing.T, app *AppModel) *ContextHubModel {
	t.Helper()
	hub, ok := app.manageHub.ActiveModuleView().(*ContextHubModel)
	if !ok {
		t.Fatalf("active Manage module view = %T, want *ContextHubModel", app.manageHub.ActiveModuleView())
	}
	return hub
}

// TestManageHubHostsContextsAsOneTypedRoute locks session 4's core promise: the
// `c` key resolves to the Manage Contexts route rather than a standalone picker
// overlay, and a single Esc returns to the Outliner root.
func TestManageHubHostsContextsAsOneTypedRoute(t *testing.T) {
	app := contextHubApp(t)

	handled, cmd := app.handleContextMessages(ShowContextHubMsg{})
	if !handled {
		t.Fatal("ShowContextHubMsg was not handled")
	}
	if cmd != nil {
		cmd()
	}

	route := app.CurrentRoute()
	params, ok := route.Params().(ManageParams)
	want := ManageParams{Module: ManageContexts}
	if route.ID() != SecondaryTool || !ok || params != want {
		t.Fatalf("context route = %s/%#v, want Manage/%#v", route.ID(), route.Params(), want)
	}
	if app.activeView() != app.manageHub {
		t.Fatalf("active renderer = %T, want the Manage hub", app.activeView())
	}
	contextHubModuleView(t, app)
	if got := app.manageHub.StatusBarLabel(); got != "Contexts" {
		t.Fatalf("status bar label = %q, want %q", got, "Contexts")
	}
	if presentation, ok := app.manageHub.PresentationView(); !ok || presentation != ViewContextHub {
		t.Fatalf("presentation = %s/%v, want ViewContextHub", presentation, ok)
	}

	app.handleGlobalKeyPress(syntheticSpecialKey(27))
	if app.CurrentRoute().ID() != TaskWorkspace {
		t.Fatalf("Esc route = %s, want TaskWorkspace", app.CurrentRoute().ID())
	}
}

// TestManageHubContextSelectionAppliesOutlinerScope asserts the hub reports a
// choice through the existing ContextScopeSelectedMsg seam, so applying a context
// still narrows the Outliner and pops back to it.
func TestManageHubContextSelectionAppliesOutlinerScope(t *testing.T) {
	for _, tc := range []struct {
		name string
		want ContextScope
	}{
		{name: "all", want: AllContextScope()},
		{name: "regular", want: RegularContextScope("work")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := contextHubApp(t)
			app.handleContextMessages(ShowContextHubMsg{})
			hub := contextHubModuleView(t, app)

			focusContextHubScope(t, hub, tc.want)

			_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Enter emitted no selection command")
			}
			done, ok := cmd().(ContextScopeSelectedMsg)
			if !ok {
				t.Fatalf("Enter emitted %T, want ContextScopeSelectedMsg", cmd())
			}
			app.handleContextMessages(done)

			if got := app.crossOutliner.activeContext; got != tc.want {
				t.Fatalf("applied context = %#v, want %#v", got, tc.want)
			}
			if app.CurrentRoute().ID() != TaskWorkspace {
				t.Fatalf("apply route = %s, want TaskWorkspace", app.CurrentRoute().ID())
			}
		})
	}
}

// TestManageHubContextsRendersEveryScopeWithCounts asserts the hub renders the
// shared summary list rather than a second, independently derived catalog.
func TestManageHubContextsRendersEveryScopeWithCounts(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	app.handleContextMessages(ShowContextHubMsg{})

	hub := contextHubModuleView(t, app)
	hub.SetSize(100, 24)
	if hub.active != RegularContextScope("work") {
		t.Fatalf("hub active context = %#v, want the Outliner's live scope", hub.active)
	}
	if scope, ok := currentContextHubScope(hub); !ok || scope != RegularContextScope("work") {
		t.Fatalf("cursor started on %#v/%v, want the active context", scope, ok)
	}

	rendered := strings.Join(strings.Fields(ansi.Strip(hub.View().Content)), " ")
	for _, want := range []string{"All Projects", "work (active)", "home", "personal"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("context hub render missing %q:\n%s", want, rendered)
		}
	}

	// Reopening must rebuild from live state: a cached instance would still show
	// the previous active context and miss contexts created since.
	app.handleGlobalKeyPress(syntheticSpecialKey(27))
	app.crossOutliner.applyContextScope(RegularContextScope("work"))
	catalog := cloneWorkspaceCatalog(app.runtime.Catalog())
	catalog.Projects[1].Context = "later"
	app.runtime.SetCatalogInMemory(catalog)
	app.handleContextMessages(ShowContextHubMsg{})

	reopened := contextHubModuleView(t, app)
	if reopened.active != RegularContextScope("work") {
		t.Fatalf("reopened hub active context = %#v, want the current Outliner scope", reopened.active)
	}
	hasLater := false
	for _, summary := range reopened.summaries {
		if summary.Scope == RegularContextScope("later") {
			hasLater = true
		}
	}
	if !hasLater {
		t.Fatalf("reopened hub missed a context created since the last visit: %#v", reopened.summaries)
	}
}

// TestRouteValidatesManageContexts locks the sealed-enum extension: Contexts is
// an accepted Manage module, Notes-only scope is canonicalized away, and
// Projects is also accepted and the module after it remains rejected.
func TestRouteValidatesManageContexts(t *testing.T) {
	route, err := NewRoute(SecondaryTool, ManageParams{
		Module:     ManageContexts,
		NotesScope: ProjectNotesScope("alpha"),
	})
	if err != nil {
		t.Fatalf("NewRoute(ManageContexts): %v", err)
	}
	want := ManageParams{Module: ManageContexts}
	if got := route.Params().(ManageParams); got != want {
		t.Fatalf("canonical Contexts params = %#v, want %#v", got, want)
	}
	if got := ManageContexts.String(); got != "Contexts" {
		t.Fatalf("ManageContexts.String() = %q, want %q", got, "Contexts")
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{Module: ManageProjects + 1}); err == nil {
		t.Fatal("module past Projects must remain invalid")
	}
}

// TestContextHubGroupsScopesAndPreviewsMembers covers the browse surface: All
// Projects and regular contexts share one ordered list, and the preview describes
// the highlighted scope with its member projects and counts.
func TestContextHubGroupsScopesAndPreviewsMembers(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))

	rendered := contextHubRender(hub)
	for _, want := range []string{
		"SCOPES",
		"Every active project", "2 active project(s) · 3 task(s)",
		"Projects", "Alpha 2 task(s)", "Beta 1 task(s)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("All-scope hub render missing %q:\n%s", want, rendered)
		}
	}

	// Moving onto a regular context repoints the preview to exactly its projects.
	seekContextHubScope(t, hub, RegularContextScope("work"))
	rendered = contextHubRender(hub)
	for _, want := range []string{
		"Project-label context",
		"Alpha 2 task(s)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("regular-scope preview missing %q:\n%s", want, rendered)
		}
	}
	preview, ok := hub.surface.CurrentPreview()
	if !ok {
		t.Fatal("regular-scope preview is unavailable")
	}
	if normalized := strings.Join(strings.Fields(preview.Body), " "); !strings.Contains(
		normalized,
		"1 active project(s) · 2 task(s)",
	) {
		t.Fatalf("regular-scope preview counts missing from %q", normalized)
	}
	if strings.Contains(preview.Body, "Beta") {
		t.Fatalf("regular-scope preview leaked a non-member project:\n%s", preview.Body)
	}
}

// TestContextHubSearchMatchesContextAndProjectNames asserts search spans both
// axes the session names, and that clearing it restores the full list.
func TestContextHubSearchMatchesContextAndProjectNames(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))
	all := len(contextHubScopes(hub))

	hub.Update(syntheticKey('/'))
	if !hub.HasTextInput() {
		t.Fatal("/ did not open the hub search input")
	}
	typeIntoContextHub(hub, "wor")
	if got := contextHubScopes(hub); len(got) != 1 || got[0] != RegularContextScope("work") {
		t.Fatalf("context-name search = %#v, want [work]", got)
	}

	// A project name the user knows but whose context they do not: Beta belongs
	// to the "home" label, and All always contains it.
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	hub.Update(syntheticKey('/'))
	typeIntoContextHub(hub, "beta")
	want := []ContextScope{AllContextScope(), RegularContextScope("home")}
	if got := contextHubScopes(hub); !slices.Equal(got, want) {
		t.Fatalf("project-name search = %#v, want %#v", got, want)
	}

	// Enter keeps the filter; Esc then clears it and restores every scope.
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if hub.HasTextInput() || hub.SearchQuery() != "beta" {
		t.Fatalf("Enter must keep the query: input=%v query=%q", hub.HasTextInput(), hub.SearchQuery())
	}
	if !hub.ShouldCaptureEsc() {
		t.Fatal("a live query must keep Esc inside the hub so it clears the search first")
	}
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.SearchQuery() != "" || len(contextHubScopes(hub)) != all {
		t.Fatalf("Esc left query=%q with %d of %d scopes", hub.SearchQuery(), len(contextHubScopes(hub)), all)
	}
}

// TestContextHubRendersEmptyStates covers the no-crash requirement for an empty
// personal context, a search with no matches, and a hub built over an empty
// catalog.
func TestContextHubRendersEmptyStates(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	seekContextHubScope(t, hub, RegularContextScope("personal"))
	if got := contextHubRender(hub); !strings.Contains(got, "No active projects in this context.") {
		t.Fatalf("empty personal context preview missing its empty state:\n%s", got)
	}

	hub.Update(syntheticKey('/'))
	typeIntoContextHub(hub, "nothing-matches")
	if matches := hub.surface.Matches(); len(matches) != 0 {
		t.Fatalf("unmatched search kept %d List Surface matches", len(matches))
	}
	if got := contextHubRender(hub); !strings.Contains(got, "No contexts match") {
		t.Fatalf("unmatched search missing its empty state:\n%s", got)
	}

	bare := NewContextHubModel(nil, nil, nil, AllContextScope())
	bare.SetSize(120, 30)
	if got := contextHubRender(bare); !strings.Contains(got, "No active projects in this context.") {
		t.Fatalf("empty catalog hub render:\n%s", got)
	}
}

// TestContextHubApplyUpdatesOutlinerRows is the session's "done when": choosing
// All or a regular label re-scopes the cross-project Outliner's rows through the
// shell path.
func TestContextHubApplyUpdatesOutlinerRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed ContextScope
		want ContextScope
		rows []string
	}{
		{name: "all", seed: RegularContextScope("home"), want: AllContextScope(), rows: []string{"alpha", "beta"}},
		{name: "regular", seed: RegularContextScope("work"), want: RegularContextScope("home"), rows: []string{"beta"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := contextHubApp(t)
			app.crossOutliner.applyContextScope(tc.seed)
			hub := openContextHub(t, app)

			focusContextHubScope(t, hub, tc.want)

			loaded := app.crossOutliner.loadedProjects
			_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("Enter emitted no selection command")
			}
			app.handleContextMessages(cmd())
			// Returning to the Outliner also kicks off a disk reload, which drops
			// the in-memory projects until its result lands. Settle that load with
			// the same active set so rows reflect the newly applied context.
			app.crossOutliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
				projects:   loaded,
				generation: app.crossOutliner.crossLoadGen,
			})

			if got := app.crossOutliner.activeContext; got != tc.want {
				t.Fatalf("applied context = %#v, want %#v", got, tc.want)
			}
			visible := scopeVisibleProjectIDs(app.crossOutliner)
			if len(visible) != len(tc.rows) {
				t.Fatalf("Outliner rows span %v, want %v", visible, tc.rows)
			}
			for _, id := range tc.rows {
				if !visible[id] {
					t.Fatalf("Outliner rows span %v, want %v", visible, tc.rows)
				}
			}
		})
	}
}

// TestContextHubEditActionsRejectAllScope keeps the editing verbs off All, which
// is derived from the whole catalog and has no membership of its own.
func TestContextHubEditActionsRejectAllScope(t *testing.T) {
	for _, key := range []rune{'e', 'r', 'd'} {
		app := contextHubApp(t)
		hub := openContextHub(t, app)
		focusContextHubScope(t, hub, AllContextScope())

		hub.Update(syntheticKey(key))
		if hub.editor != nil {
			t.Fatalf("%q on All opened the membership editor", string(key))
		}
		if hub.confirm.IsActive() {
			t.Fatalf("%q on All armed a delete confirmation", string(key))
		}
		if hub.notice == "" {
			t.Fatalf("%q on All gave the user no explanation", string(key))
		}
	}
}

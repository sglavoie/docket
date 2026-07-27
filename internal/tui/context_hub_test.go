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
// and one custom context, with a live cross-project Outliner so an applied scope
// has somewhere to land. Alpha carries two tasks and Beta one so the hub's
// preview counts distinguish them.
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
	app.runtime.Machine().SetCustomContext("focus", []string{"alpha"})

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

func currentContextHubSummary(hub *ContextHubModel) (ContextScopeSummary, bool) {
	scope, ok := currentContextHubScope(hub)
	if !ok {
		return ContextScopeSummary{}, false
	}
	for _, summary := range hub.summaries {
		if summary.Scope == scope {
			return summary, true
		}
	}
	return ContextScopeSummary{}, false
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
// `C` key resolves to the Manage Contexts route rather than a standalone picker
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
	want := ManageParams{Module: ManageContexts, Scope: AppEntityScope()}
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
		{name: "regular", want: RegularContextScope("work")},
		{name: "custom", want: CustomContextScope("focus")},
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
	for _, want := range []string{"All", "focus", "custom · 1 project(s)", "work (active)", "home"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("context hub render missing %q:\n%s", want, rendered)
		}
	}

	// Reopening must rebuild from live state: a cached instance would still show
	// the previous active context and miss contexts created since.
	app.handleGlobalKeyPress(syntheticSpecialKey(27))
	app.crossOutliner.applyContextScope(CustomContextScope("focus"))
	app.runtime.Machine().SetCustomContext("later", []string{"beta"})
	app.handleContextMessages(ShowContextHubMsg{})

	reopened := contextHubModuleView(t, app)
	if reopened.active != CustomContextScope("focus") {
		t.Fatalf("reopened hub active context = %#v, want the current Outliner scope", reopened.active)
	}
	hasLater := false
	for _, summary := range reopened.summaries {
		if summary.Scope == CustomContextScope("later") {
			hasLater = true
		}
	}
	if !hasLater {
		t.Fatalf("reopened hub missed a context created since the last visit: %#v", reopened.summaries)
	}
}

// TestRouteValidatesManageContexts locks the sealed-enum extension: Contexts is
// an accepted app-scoped Manage module, project scope is canonicalized away, and
// the module after it is still rejected.
func TestRouteValidatesManageContexts(t *testing.T) {
	route, err := NewRoute(SecondaryTool, ManageParams{Module: ManageContexts, Scope: ProjectEntityScope("alpha")})
	if err != nil {
		t.Fatalf("NewRoute(ManageContexts): %v", err)
	}
	want := ManageParams{Module: ManageContexts, Scope: AppEntityScope()}
	if got := route.Params().(ManageParams); got != want {
		t.Fatalf("canonical Contexts params = %#v, want %#v", got, want)
	}
	if got := ManageContexts.String(); got != "Contexts" {
		t.Fatalf("ManageContexts.String() = %q, want %q", got, "Contexts")
	}
	if _, err := NewRoute(SecondaryTool, ManageParams{Module: ManageContexts + 1}); err == nil {
		t.Fatal("module past Contexts must remain invalid")
	}
}

// TestContextHubGroupsScopesAndPreviewsMembers covers the browse surface: the
// left pane is grouped All / Custom Contexts / Regular Contexts and the right
// pane describes the highlighted scope with its member projects and counts.
func TestContextHubGroupsScopesAndPreviewsMembers(t *testing.T) {
	hub := openContextHub(t, contextHubApp(t))

	rendered := contextHubRender(hub)
	for _, want := range []string{
		"BASE SCOPES", "ACTIVE PROJECTS",
		"Every active project", "2 active project(s) · 3 task(s)",
		"Projects", "Alpha 2 task(s)", "Beta 1 task(s)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("All-scope hub render missing %q:\n%s", want, rendered)
		}
	}

	// Moving onto the custom context must repoint the preview and surface the
	// archived/stale counts only custom scopes can have.
	seekContextHubScope(t, hub, CustomContextScope("focus"))
	rendered = contextHubRender(hub)
	for _, want := range []string{
		"Custom context · saved project group",
		"Alpha 2 task(s)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("custom-scope preview missing %q:\n%s", want, rendered)
		}
	}
	preview, ok := hub.surface.CurrentPreview()
	if !ok {
		t.Fatal("custom-scope preview is unavailable")
	}
	if normalized := strings.Join(strings.Fields(preview.Body), " "); !strings.Contains(
		normalized,
		"1 active project(s) · 0 archived · 0 missing · 2 task(s)",
	) {
		t.Fatalf("custom-scope preview counts missing from %q", normalized)
	}
	if strings.Contains(preview.Body, "Beta") {
		t.Fatalf("custom-scope preview leaked a non-member project:\n%s", preview.Body)
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
	typeIntoContextHub(hub, "foc")
	if got := contextHubScopes(hub); len(got) != 1 || got[0] != CustomContextScope("focus") {
		t.Fatalf("context-name search = %#v, want [focus]", got)
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

// TestContextHubRendersEmptyStates covers the session's no-crash requirement for
// empty project sets: an empty custom context, a search with no matches, and a
// hub built over an empty catalog.
func TestContextHubRendersEmptyStates(t *testing.T) {
	app := contextHubApp(t)
	app.runtime.Machine().SetCustomContext("empty", nil)
	hub := openContextHub(t, app)

	seekContextHubScope(t, hub, CustomContextScope("empty"))
	if got := contextHubRender(hub); !strings.Contains(got, "No active projects in this context.") {
		t.Fatalf("empty custom context preview missing its empty state:\n%s", got)
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
// All, a regular label, and a custom context each re-scope the cross-project
// Outliner's rows through the shell path.
func TestContextHubApplyUpdatesOutlinerRows(t *testing.T) {
	for _, tc := range []struct {
		name string
		seed ContextScope
		want ContextScope
		rows []string
	}{
		{name: "all", seed: RegularContextScope("home"), want: AllContextScope(), rows: []string{"alpha", "beta"}},
		{name: "regular", seed: RegularContextScope("work"), want: RegularContextScope("home"), rows: []string{"beta"}},
		{name: "custom", seed: RegularContextScope("home"), want: CustomContextScope("focus"), rows: []string{"alpha"}},
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

// TestContextHubCreatesCustomContext covers the `n` path end to end: the inline
// editor takes a name and a membership selection, Ctrl+S commits through
// ContextMutator, and the browse list refreshes onto the new scope.
func TestContextHubCreatesCustomContext(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	hub.Update(syntheticKey('n'))
	if hub.editor == nil {
		t.Fatal("n did not open the inline custom context editor")
	}
	if !hub.HasTextInput() || !hub.ShouldCaptureEsc() {
		t.Fatal("an open editor must claim text input and Esc so shell interceptors stay out")
	}
	// The name field must receive these verbatim rather than losing them to the
	// shell's queue/today/undo/clipboard/palette shortcuts.
	for _, key := range []rune{'q', 'w', 'u', 'y', '#', '|', ':'} {
		if handled, _ := app.handleGlobalKeyPress(syntheticKey(key)); handled {
			t.Fatalf("the global %q shortcut swallowed an editor keystroke", string(key))
		}
	}
	typeIntoContextHub(hub, "ship")
	checkEditorProject(t, hub, "beta")

	_, cmd := hub.Update(contextHubSaveKey)
	if cmd == nil {
		t.Fatal("ctrl+s emitted no save command")
	}
	saved, ok := cmd().(CustomContextSavedMsg)
	if !ok {
		t.Fatalf("save emitted %T, want CustomContextSavedMsg", cmd())
	}
	if !saved.IsNew || saved.Renamed || saved.Name != "ship" || saved.ProjectCount != 1 {
		t.Fatalf("save result = %#v, want a new 1-project context named ship", saved)
	}
	if hub.editor != nil {
		t.Fatal("a committed save must close the editor")
	}
	if got := app.runtime.Machine().GetCustomContext("ship"); !slices.Equal(got, []string{"beta"}) {
		t.Fatalf("committed membership = %v, want [beta]", got)
	}
	if scope, ok := currentContextHubScope(hub); !ok || scope != CustomContextScope("ship") {
		t.Fatalf("cursor parked on %#v/%v, want the context just saved", scope, ok)
	}
}

// TestContextHubCustomContextSaveSurfacesValidationErrors asserts the hub
// delegates validation to ContextMutator and keeps a rejected save inside the
// editor with nothing committed.
func TestContextHubCustomContextSaveSurfacesValidationErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		typed   string
		member  string
		wantErr string
	}{
		{name: "empty name", typed: "", member: "alpha", wantErr: "cannot be empty"},
		{name: "duplicate name", typed: "focus", member: "alpha", wantErr: "already exists"},
		{name: "duplicate name ignoring case", typed: "FOCUS", member: "alpha", wantErr: "already exists"},
		{name: "no members", typed: "ship", member: "", wantErr: "select at least one project"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := contextHubApp(t)
			hub := openContextHub(t, app)
			before := len(ContextScopeSummaries(app.runtime))

			hub.Update(syntheticKey('n'))
			typeIntoContextHub(hub, tc.typed)
			if tc.member != "" {
				checkEditorProject(t, hub, tc.member)
			}

			if _, cmd := hub.Update(contextHubSaveKey); cmd != nil {
				t.Fatalf("a rejected save emitted %T", cmd())
			}
			if hub.editor == nil {
				t.Fatal("a rejected save must keep the editor open for correction")
			}
			if !strings.Contains(hub.editor.err, tc.wantErr) {
				t.Fatalf("editor error = %q, want it to mention %q", hub.editor.err, tc.wantErr)
			}
			if got := len(ContextScopeSummaries(app.runtime)); got != before {
				t.Fatalf("a rejected save changed the scope count to %d, want %d", got, before)
			}
		})
	}
}

// TestContextHubEditPreservesHiddenCustomRefs locks the session's preservation
// rule: the editor lists active projects only, so an edit must not discard the
// archived and no-longer-cataloged members it never showed.
func TestContextHubEditPreservesHiddenCustomRefs(t *testing.T) {
	app := contextHubApp(t)
	archived := scopeTestProject("gamma", "Gamma", "1")
	archived.Slug = "gamma"
	archived.Context = "work"
	archived.Archived = true
	addTestAppProject(app, archived)
	app.runtime.Machine().SetCustomContext("focus", []string{"alpha", "gamma", "ghost"})

	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, CustomContextScope("focus"))
	hub.Update(syntheticKey('e'))
	if hub.editor == nil {
		t.Fatal("e did not open the editor on a custom context")
	}
	for _, project := range hub.editor.projects {
		if project.ID == "gamma" {
			t.Fatal("the editor must not list archived projects")
		}
	}

	checkEditorProject(t, hub, "beta")
	if _, cmd := hub.Update(contextHubSaveKey); cmd == nil {
		t.Fatal("ctrl+s emitted no save command")
	}
	want := []string{"alpha", "gamma", "ghost", "beta"}
	if got := app.runtime.Machine().GetCustomContext("focus"); !slices.Equal(got, want) {
		t.Fatalf("membership after edit = %v, want %v", got, want)
	}
}

// TestContextHubEditPreservesSandboxAndInternalMembers walks the whole surface
// the finding named: a custom context holding sandbox and internal projects must
// report them, must not preview or count them as active, and must survive a save
// that changes nothing — the case that used to delete them silently.
func TestContextHubEditPreservesSandboxAndInternalMembers(t *testing.T) {
	app := contextHubApp(t)
	sandbox := scopeTestProject("sandbox", "Scratch", "1")
	sandbox.Slug = "_sandbox"
	sandbox.Context = "work"
	sandbox.Sandbox = true
	system := scopeTestProject("system", "Inbox", "1")
	system.Slug = "inbox"
	system.Context = "work"
	system.Internal = true
	addTestAppProject(app, sandbox)
	addTestAppProject(app, system)
	stored := []string{"alpha", "sandbox", "system"}
	app.runtime.Machine().SetCustomContext("focus", stored)

	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, CustomContextScope("focus"))
	summary, ok := currentContextHubSummary(hub)
	if !ok {
		t.Fatal("List Surface cursor did not resolve a context summary")
	}
	if summary.ActiveProjectCount != 1 || summary.HiddenProjectCount != 2 {
		t.Fatalf("summary = %+v, want active=1 hidden=2", summary)
	}
	if rendered := contextHubRender(hub); !strings.Contains(rendered, "custom · 1 project(s) · 2 hidden") {
		t.Fatalf("hub render never reports the hidden members:\n%s", rendered)
	}

	hub.Update(syntheticKey('e'))
	if hub.editor == nil {
		t.Fatal("e did not open the editor on a custom context")
	}
	for _, project := range hub.editor.projects {
		if project.ID == "sandbox" || project.ID == "system" {
			t.Fatalf("the editor listed hidden project %q", project.ID)
		}
	}
	if !slices.Equal(hub.editor.hiddenMembers, []string{"sandbox", "system"}) {
		t.Fatalf("editor hidden members = %v, want the sandbox and internal members", hub.editor.hiddenMembers)
	}
	if len(hub.editor.selected) != 1 {
		t.Fatalf("editor reported %d selected, want only the one listed member", len(hub.editor.selected))
	}
	if rendered := contextHubRender(hub); !strings.Contains(rendered, "+ 2 hidden member(s) kept on save") {
		t.Fatalf("editor never says the hidden members are preserved:\n%s", rendered)
	}

	// Saving an otherwise untouched context is the exact case that dropped them.
	if _, cmd := hub.Update(contextHubSaveKey); cmd == nil {
		t.Fatal("ctrl+s emitted no save command")
	}
	if got := app.runtime.Machine().GetCustomContext("focus"); !slices.Equal(got, stored) {
		t.Fatalf("membership after an unchanged save = %v, want %v", got, stored)
	}
}

// TestContextHubEditorDiscardConfirmationGuardsUnsavedEdits covers the Esc
// ladder: Esc leaves the project list, a dirty editor then arms the shared
// discard confirmation, declining keeps the edit, and confirming drops it.
func TestContextHubEditorDiscardConfirmationGuardsUnsavedEdits(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)

	hub.Update(syntheticKey('n'))
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.editor != nil || hub.confirmDiscard {
		t.Fatal("Esc on a pristine editor must close it without a confirmation")
	}

	hub.Update(syntheticKey('n'))
	typeIntoContextHub(hub, "draft")
	hub.Update(syntheticSpecialKey(tea.KeyTab))
	if !hub.editor.listFocus {
		t.Fatal("tab did not move focus to the project list")
	}
	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.editor.listFocus || hub.confirmDiscard {
		t.Fatal("the first Esc must only leave the project list")
	}

	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if !hub.confirmDiscard {
		t.Fatal("Esc on a dirty editor must arm the discard confirmation")
	}
	dialog := hub.GetConfirmDialog()
	if dialog == nil || !dialog.IsActive() || dialog.Action() != "discard" {
		t.Fatalf("discard dialog = %#v, want an active discard confirmation", dialog)
	}

	hub.Update(syntheticKey('n'))
	if hub.confirmDiscard || hub.editor == nil {
		t.Fatal("declining the discard must keep the editor and its edits")
	}
	if got := hub.editor.nameInput.Value(); got != "draft" {
		t.Fatalf("declined discard lost the typed name: %q", got)
	}

	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	hub.Update(syntheticKey('y'))
	if hub.editor != nil || hub.confirmDiscard {
		t.Fatal("confirming the discard must close the editor")
	}
	if hub.GetConfirmDialog().IsActive() {
		t.Fatal("a resolved discard must leave no active dialog")
	}
}

// TestContextHubDeleteConfirmsAndFallsBackToAll covers `d`: the delete is
// confirmed first, discards the context's hidden refs, and resets an active
// scope to All through the shell.
func TestContextHubDeleteConfirmsAndFallsBackToAll(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(CustomContextScope("focus"))
	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, CustomContextScope("focus"))

	hub.Update(syntheticKey('d'))
	if !hub.confirm.IsActive() || hub.confirm.Action() != "delete" {
		t.Fatal("d did not arm the delete confirmation")
	}
	// The overlay reaches the screen only through the Manage hub's delegation.
	if got := ansi.Strip(app.renderConfirmDialogOverlay()); !strings.Contains(got, "Delete custom context") {
		t.Fatalf("the shell rendered no delete confirmation overlay: %q", got)
	}
	hub.Update(syntheticKey('n'))
	if hub.confirm.IsActive() || hub.pendingDelete.Name != "" {
		t.Fatal("declining the delete must disarm it")
	}
	if !app.runtime.Machine().CustomContextExists("focus") {
		t.Fatal("a declined delete must not commit")
	}

	loaded := app.crossOutliner.loadedProjects
	hub.Update(syntheticKey('d'))
	_, cmd := hub.Update(syntheticKey('y'))
	if cmd == nil {
		t.Fatal("confirming the delete emitted no command")
	}
	deleted, ok := cmd().(CustomContextDeletedMsg)
	if !ok || deleted.Name != "focus" || !deleted.WasActive {
		t.Fatalf("delete emitted %#v, want the active focus context", cmd())
	}
	if app.runtime.Machine().CustomContextExists("focus") {
		t.Fatal("confirming the delete must commit it")
	}
	if hub.active != AllContextScope() {
		t.Fatalf("hub active scope = %#v, want All after deleting the active context", hub.active)
	}

	app.handleContextMessages(deleted)
	app.crossOutliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   loaded,
		generation: app.crossOutliner.crossLoadGen,
	})
	if got := app.crossOutliner.activeContext; got != AllContextScope() {
		t.Fatalf("Outliner scope = %#v, want All after the active context was deleted", got)
	}
	if visible := scopeVisibleProjectIDs(app.crossOutliner); !visible["alpha"] || !visible["beta"] {
		t.Fatalf("Outliner rows span %v, want every project once the scope falls back to All", visible)
	}
}

// TestContextHubRenameFollowsTheActiveScope is the reconciliation half of the
// session: renaming the context the Outliner is scoped to must move the scope to
// the new name rather than stranding it on a name that no longer exists.
func TestContextHubRenameFollowsTheActiveScope(t *testing.T) {
	app := contextHubApp(t)
	app.crossOutliner.applyContextScope(CustomContextScope("focus"))
	hub := openContextHub(t, app)
	focusContextHubScope(t, hub, CustomContextScope("focus"))

	hub.Update(syntheticKey('r'))
	if hub.editor == nil || hub.editor.originalName != "focus" {
		t.Fatalf("r did not open the editor on focus: %#v", hub.editor)
	}
	for range "focus" {
		hub.Update(syntheticSpecialKey(tea.KeyBackspace))
	}
	typeIntoContextHub(hub, "deep")

	loaded := app.crossOutliner.loadedProjects
	_, cmd := hub.Update(contextHubSaveKey)
	if cmd == nil {
		t.Fatal("ctrl+s emitted no save command")
	}
	saved, ok := cmd().(CustomContextSavedMsg)
	if !ok || !saved.Renamed || saved.Name != "deep" || saved.PreviousName != "focus" || !saved.WasActive {
		t.Fatalf("save emitted %#v, want an active rename focus -> deep", cmd())
	}
	if app.runtime.Machine().CustomContextExists("focus") {
		t.Fatal("a rename must drop the old key")
	}
	if hub.active != CustomContextScope("deep") {
		t.Fatalf("hub active scope = %#v, want the renamed context", hub.active)
	}

	app.handleContextMessages(saved)
	app.crossOutliner.handleCrossProjectsLoaded(crossProjectsLoadedMsg{
		projects:   loaded,
		generation: app.crossOutliner.crossLoadGen,
	})
	if got := app.crossOutliner.activeContext; got != CustomContextScope("deep") {
		t.Fatalf("Outliner scope = %#v, want the renamed context", got)
	}
	visible := scopeVisibleProjectIDs(app.crossOutliner)
	if !visible["alpha"] || visible["beta"] {
		t.Fatalf("Outliner rows span %v, want only alpha under the renamed context", visible)
	}
}

// TestContextHubEditorSearchAndBulkSelection covers the remaining editor
// affordances the session names: filtering the project list and select
// all/none over the filtered subset.
func TestContextHubEditorSearchAndBulkSelection(t *testing.T) {
	app := contextHubApp(t)
	hub := openContextHub(t, app)
	hub.Update(syntheticKey('n'))
	typeIntoContextHub(hub, "wide")
	hub.Update(syntheticSpecialKey(tea.KeyTab))

	hub.Update(syntheticKey('a'))
	if len(hub.editor.selected) != 2 {
		t.Fatalf("a selected %d projects, want both", len(hub.editor.selected))
	}
	hub.Update(syntheticKey('A'))
	if len(hub.editor.selected) != 0 {
		t.Fatalf("A left %d projects selected, want none", len(hub.editor.selected))
	}

	hub.Update(syntheticKey('/'))
	if !hub.editor.searching {
		t.Fatal("/ did not open the editor's project filter")
	}
	typeIntoContextHub(hub, "bet")
	if len(hub.editor.filtered) != 1 || hub.editor.filtered[0].ID != "beta" {
		t.Fatalf("filtered projects = %#v, want only beta", hub.editor.filtered)
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if hub.editor.searching {
		t.Fatal("Enter must keep the filter and leave the input")
	}

	// a/A act on the filtered subset only, so the hidden project stays untouched.
	hub.Update(syntheticKey('a'))
	if len(hub.editor.selected) != 1 || !hub.editor.selected["beta"] {
		t.Fatalf("filtered select-all chose %#v, want only beta", hub.editor.selected)
	}

	hub.Update(syntheticSpecialKey(tea.KeyEscape))
	if hub.editor.query != "" || len(hub.editor.filtered) != 2 {
		t.Fatalf("Esc left query %q with %d projects", hub.editor.query, len(hub.editor.filtered))
	}
	if _, cmd := hub.Update(contextHubSaveKey); cmd == nil {
		t.Fatal("ctrl+s emitted no save command")
	}
	if got := app.runtime.Machine().GetCustomContext("wide"); !slices.Equal(got, []string{"beta"}) {
		t.Fatalf("committed membership = %v, want [beta]", got)
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

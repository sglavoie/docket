package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/sglavoie/tt/internal/tui/operation"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/route"
	"github.com/sglavoie/tt/internal/tui/status"
)

func TestProjectHubEscKeepsFilterThenCloses(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	if hub.HasActiveSearch() || hub.HasTextInput() || hub.ShouldCaptureEsc() {
		t.Fatalf(
			"the Hub opened searching: active=%v input=%v capture=%v",
			hub.HasActiveSearch(),
			hub.HasTextInput(),
			hub.ShouldCaptureEsc(),
		)
	}
	hub.Update(syntheticKey('/'))
	if !hub.HasActiveSearch() || !hub.HasTextInput() || !hub.ShouldCaptureEsc() {
		t.Fatalf(
			"'/' did not open search: active=%v input=%v capture=%v",
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
	if hub.HasActiveSearch() || hub.SearchQuery() != "bet" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	if app.CurrentRoute().ID() != route.SecondaryTool || app.currentView() != presentation.ViewProjectHub {
		t.Fatalf("first Esc closed Project Hub: %s/%s", app.CurrentRoute().ID(), app.currentView())
	}
	if len(hub.surface.Matches()) != 1 {
		t.Fatalf("Esc left %d matches, want the filter intact", len(hub.surface.Matches()))
	}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatalf("filtered browse Esc handled=%v route=%s, want Outliner", handled, app.CurrentRoute().ID())
	}
}

func TestProjectHubSearchEnterSelectsImmediately(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
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
	if app.crossOutliner.scopedProjectID != "beta" || app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatalf(
			"selection landed project=%q route=%s",
			app.crossOutliner.scopedProjectID,
			app.CurrentRoute().ID(),
		)
	}
}

func TestProjectHubSearchNavigationSelectsHighlightedProject(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
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
	spec, ok := hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "↑/↓ or Ctrl+J/K navigate") {
		t.Fatalf("search guidance does not advertise navigation: %q", help)
	}
}

func TestProjectHubSearchPlainJAndKRemainQueryInput(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('j'))
	hub.Update(syntheticKey('k'))

	if query := hub.SearchQuery(); query != "jk" {
		t.Fatalf("plain j/k query = %q, want %q", query, "jk")
	}
	if !hub.HasActiveSearch() {
		t.Fatal("plain j/k deactivated search")
	}
}

// TestProjectHubUndoKeysAskTheShellForTheWorkspaceLedger pins both halves of the
// Hub's undo binding. Archiving, parking, renaming, pinning and tagging are
// ledger transactions, so the browse press has to reach the one workspace
// history; the core undo descriptors answer only to the Outliner, so it asks the
// shell rather than dispatching through the executor.
func TestProjectHubUndoKeysAskTheShellForTheWorkspaceLedger(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))

	_, cmd := hub.Update(syntheticKey('u'))
	if cmd == nil {
		t.Fatal("browse u emitted no workspace undo request")
	}
	if msg, ok := cmd().(status.WorkspaceUndoMsg); !ok || msg.Redo {
		t.Fatalf("browse u emitted %#v, want an undo request", cmd())
	}

	_, cmd = hub.Update(syntheticCtrlKey('r'))
	if cmd == nil {
		t.Fatal("browse Ctrl+R emitted no workspace redo request")
	}
	if msg, ok := cmd().(status.WorkspaceUndoMsg); !ok || !msg.Redo {
		t.Fatalf("browse Ctrl+R emitted %#v, want a redo request", cmd())
	}
}

// TestProjectHubSearchPlainURemainsQueryInput holds u below the search branch
// where `?` already sits. u is a literal character in a project name, so a Hub
// that answered it above the query would reverse the last project mutation while
// the user was still typing.
func TestProjectHubSearchPlainURemainsQueryInput(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	hub.Update(syntheticKey('/'))
	if _, cmd := hub.Update(syntheticKey('u')); cmd != nil {
		if _, ok := cmd().(status.WorkspaceUndoMsg); ok {
			t.Fatal("u undid a transaction while the query was being typed")
		}
	}
	if query := hub.SearchQuery(); query != "u" {
		t.Fatalf("typed u query = %q, want %q", query, "u")
	}
}

func TestProjectHubArchivedEnterKeepsHubOpenWithNotice(t *testing.T) {
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:archived:archived")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("archived Enter emitted %#v", cmd())
	}
	if !strings.Contains(hub.notice, "unarchived") || app.currentView() != presentation.ViewProjectHub {
		t.Fatalf("archived Enter notice=%q view=%s", hub.notice, app.currentView())
	}
}

func TestProjectHubBrowseACreatesProject(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))

	_, cmd := hub.Update(syntheticKey('a'))
	if cmd == nil {
		t.Fatal("browse a emitted no project creation request")
	}
	if _, ok := cmd().(CreateProjectMsg); !ok {
		t.Fatalf("browse a emitted %T, want CreateProjectMsg", cmd())
	}
}

// TestProjectHubRetiredKeysAreSilent pins the keys the browse grammar dropped.
// n created a project, A cleared the selection, and a bare g jumped to the top;
// each now belongs to another surface or to nothing, so a press must leave the
// Hub untouched rather than act or complain.
func TestProjectHubRetiredKeysAreSilent(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	for _, retired := range []rune{'n', 'A'} {
		_, cmd := hub.Update(syntheticKey(retired))
		if cmd != nil {
			t.Fatalf("retired key %q emitted %#v", string(retired), cmd())
		}
		if hub.notice != "" {
			t.Fatalf("retired key %q raised notice %q", string(retired), hub.notice)
		}
		if hub.selectionCount() != 0 {
			t.Fatalf("retired key %q changed the selection", string(retired))
		}
	}

	hub.Update(syntheticKey('g'))
	current, ok := hub.surface.CurrentItem()
	if !ok || current.ID != "project-hub:project:beta" {
		t.Fatalf("a lone g moved the cursor to %q", current.ID)
	}
	hub.Update(syntheticKey('g'))
	if current, _ = hub.surface.CurrentItem(); current.ID != "project-hub:project:alpha" {
		t.Fatalf("gg landed on %q, want the first project", current.ID)
	}
}

func TestProjectHubReopensInBrowseWithoutTheOldQuery(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('a'))
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.SearchQuery() != "a" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled {
		t.Fatal("browse Esc did not close Project Hub")
	}

	reopened := openProjectHub(t, app)
	if reopened.HasActiveSearch() || reopened.SearchQuery() != "" || reopened == hub {
		t.Fatalf(
			"reopened hub active=%v query=%q reused=%v",
			reopened.HasActiveSearch(),
			reopened.SearchQuery(),
			reopened == hub,
		)
	}
}

func TestProjectHubStaleProjectActionUsesProductionExecutorError(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	if hub.registry != app.operationRegistry || hub.executor != app.operationExecutor {
		t.Fatal("Project Hub did not receive the production operation system")
	}

	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}
	if !hub.actions.surface.SetCursorToID(ListItemID("project-hub-action:" + string(operation.OperationScopeArchiveProject))) {
		t.Fatal("archive action was not selectable")
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

// TestProjectHubStaleDirectKeyUsesProductionExecutorError proves the direct
// grammar reaches the same executor as the menu: a key pressed on a row whose
// project has since left the workspace refuses with the executor's own sentence.
func TestProjectHubStaleDirectKeyUsesProductionExecutorError(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubItem(t, hub, "project-hub:project:beta")

	setTestAppProjects(app, app.projectByID("alpha"))
	_, cmd := hub.Update(syntheticKey('e'))
	if cmd == nil {
		t.Fatal("a stale direct key reported nothing")
	}
	failure, ok := cmd().(status.ErrorMsg)
	if !ok || failure.Err == nil || failure.Err.Error() != "project beta is no longer available" {
		t.Fatalf("stale direct key emitted %#v, want the production executor error", cmd())
	}
}

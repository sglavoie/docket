package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/projecthub"
	"github.com/sglavoie/docket/internal/tui/route"
	"github.com/sglavoie/docket/internal/tui/status"
)

// The browse grammar itself moved to internal/tui/projecthub with the surface.
// What stays here is every assertion that names the shell: the Esc rung that
// closes the Hub, the scope a selection applies, and the production operation
// system a mounted Hub dispatches through.

// TestProjectHubEscKeepsFilterThenCloses holds the three Esc rungs apart. The
// Hub answers the first one itself by leaving search and the second by clearing
// the query it kept; only the third closes the surface.
func TestProjectHubEscKeepsFilterThenCloses(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	hub.Update(syntheticKey('/'))
	for _, r := range "bet" {
		hub.Update(syntheticKey(r))
	}

	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.HasActiveSearch() || hub.SearchQuery() != "bet" {
		t.Fatalf("Esc left search active=%v query=%q", hub.HasActiveSearch(), hub.SearchQuery())
	}
	if app.CurrentRoute().ID() != route.SecondaryTool || app.currentView() != presentation.ViewProjectHub {
		t.Fatalf("first Esc closed Project Hub: %s/%s", app.CurrentRoute().ID(), app.currentView())
	}

	// The Hub still claims the second Esc, so the shell leaves it to the view.
	if handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape}); handled {
		t.Fatal("the shell took the Esc that clears the Hub's query")
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.SearchQuery() != "" || app.currentView() != presentation.ViewProjectHub {
		t.Fatalf("filtered browse Esc left query=%q view=%s, want the query cleared in the Hub", hub.SearchQuery(), app.currentView())
	}

	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatalf("unfiltered browse Esc handled=%v route=%s, want Outliner", handled, app.CurrentRoute().ID())
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
	selected, ok := cmd().(outliner.ApplyProjectScopeMsg)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("search Enter emitted %#v, want beta", cmd())
	}
	app.handleViewActionMessages(selected)
	if app.crossOutliner.Scope.ProjectID != "beta" || app.CurrentRoute().ID() != route.TaskWorkspace {
		t.Fatalf(
			"selection landed project=%q route=%s",
			app.crossOutliner.Scope.ProjectID,
			app.CurrentRoute().ID(),
		)
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

// TestProjectHubTaskFilterReachesTheHubFromTheShell holds Ctrl+T to the surface
// it was pressed on. The key belongs to the Hub, so no shell handler may claim it
// while the Hub is mounted: the press has to arrive there, and the subtitle then
// names the narrowing the list is under. Which projects that narrowing keeps is
// asserted inside internal/tui/projecthub.
func TestProjectHubTaskFilterReachesTheHubFromTheShell(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	if got := projectHubSubtitleLine(hub); strings.Contains(got, "with open tasks") {
		t.Fatalf("the Project Hub opened already under a task filter: %q", got)
	}

	// No shell handler may claim the key, or the Hub would never see it.
	if handled, cmd := app.handleGlobalKeyPress(syntheticCtrlKey('t')); handled || cmd != nil {
		t.Fatalf("a shell handler claimed Ctrl+T over the mounted Hub: handled=%v cmd=%v", handled, cmd)
	}
	app.delegateToCurrentView(syntheticCtrlKey('t'))
	if app.CurrentRoute().ID() != route.SecondaryTool || app.currentView() != presentation.ViewProjectHub {
		t.Fatalf("Ctrl+T left the Project Hub: %s/%s", app.CurrentRoute().ID(), app.currentView())
	}
	if got := projectHubSubtitleLine(hub); !strings.Contains(got, "with open tasks") {
		t.Fatalf("Ctrl+T did not reach the Hub's task filter: %q", got)
	}
}

func TestProjectHubBrowseACreatesProject(t *testing.T) {
	hub := openProjectHub(t, projectHubApp(t))

	_, cmd := hub.Update(syntheticKey('a'))
	if cmd == nil {
		t.Fatal("browse a emitted no project creation request")
	}
	if _, ok := cmd().(projecthub.CreateProjectMsg); !ok {
		t.Fatalf("browse a emitted %T, want CreateProjectMsg", cmd())
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
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	handled, _ := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !handled || app.currentView() == presentation.ViewProjectHub {
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

// TestProjectHubStaleDirectKeyUsesProductionExecutorError is what proves a
// mounted Hub dispatches through the shell's own registry and executor rather
// than one of its own: a key pressed on a row whose project has since left the
// workspace refuses with the production executor's sentence, which no Hub-local
// wiring would produce.
func TestProjectHubStaleDirectKeyUsesProductionExecutorError(t *testing.T) {
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
	focusProjectHubRow(t, hub, "Beta")

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

// TestEmptyWorkspaceHistoryAnswersQuietly holds an exhausted ledger to a quiet
// info notice on the shared route every surface's u and Ctrl+R reach.
func TestEmptyWorkspaceHistoryAnswersQuietly(t *testing.T) {
	app := projectHubApp(t)
	for _, tc := range []struct {
		redo bool
		want status.InfoStatusMsg
	}{
		{false, "Nothing to undo"},
		{true, "Nothing to redo"},
	} {
		cmd := app.performGlobalTaskUndoRedoForProject(nil, tc.redo)
		if cmd == nil {
			t.Fatalf("redo=%v: an empty history said nothing", tc.redo)
		}
		if got, ok := cmd().(status.InfoStatusMsg); !ok || got != tc.want {
			t.Fatalf("redo=%v emitted %#v, want %q", tc.redo, cmd(), tc.want)
		}
	}
}

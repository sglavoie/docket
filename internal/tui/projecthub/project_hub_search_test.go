package projecthub

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// The browse and search grammar the Hub answers from its own Update. What the
// shell does with a selection, and the second Esc that closes the surface, are
// asserted beside the shell.

func newProjectHubSearchFixture(t *testing.T) *projectHubFixture {
	t.Helper()
	return newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
}

func TestProjectHubEscLeavesSearchAndKeepsTheFilter(t *testing.T) {
	hub := newProjectHubSearchFixture(t).open()
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
	if len(hub.surface.Matches()) != 1 {
		t.Fatalf("Esc left %d matches, want the filter intact", len(hub.surface.Matches()))
	}
	// A filtered browse claims no further Esc, which is what hands the next one
	// to the shell to close the Hub with.
	if hub.ShouldCaptureEsc() {
		t.Fatal("a filtered browse still claimed Esc")
	}
}

func TestProjectHubSearchNavigationSelectsHighlightedProject(t *testing.T) {
	hub := newProjectHubSearchFixture(t).open()
	hub.Update(syntheticKey('/'))
	hub.Update(syntheticKey('a'))

	assertCurrent := func(want listsurface.ListItemID) {
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
	selected, ok := cmd().(openProjectRequest)
	if !ok || selected.ProjectID != "beta" {
		t.Fatalf("Enter after search navigation emitted %#v, want beta", cmd())
	}
	spec, ok := hub.ContextualGuidance()
	if help := guidanceEntryText(spec); !ok || !strings.Contains(help, "↑/↓ or Ctrl+J/K navigate") {
		t.Fatalf("search guidance does not advertise navigation: %q", help)
	}
}

func TestProjectHubArchivedEnterKeepsHubOpenWithNotice(t *testing.T) {
	fixture := newProjectHubSearchFixture(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	fixture.add(archived)
	hub := fixture.open()
	focusProjectHubItem(t, hub, "project-hub:archived:archived")

	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("archived Enter emitted %#v", cmd())
	}
	if !strings.Contains(hub.notice, "unarchived") {
		t.Fatalf("archived Enter notice = %q, want the unarchive route", hub.notice)
	}
}

// TestProjectHubRetiredKeysAreSilent pins the keys the browse grammar dropped.
// n created a project, A cleared the selection, and a bare g jumped to the top;
// each now belongs to another surface or to nothing, so a press must leave the
// Hub untouched rather than act or complain.
func TestProjectHubRetiredKeysAreSilent(t *testing.T) {
	hub := newProjectHubSearchFixture(t).open()
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

// A row's project can leave the catalog between the Space that froze the target
// and the Enter that runs it, and the refusal has to be the executor's own
// sentence rather than a second one the Hub keeps for itself.
func TestProjectHubStaleProjectActionReportsTheExecutorError(t *testing.T) {
	fixture := newProjectHubSearchFixture(t)
	hub := fixture.open()
	focusProjectHubItem(t, hub, "project-hub:project:beta")
	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatal("Space did not freeze the project action target")
	}
	archiveID := listsurface.ListItemID("project-hub-action:" + string(operation.OperationScopeArchiveProject))
	if !hub.actions.Surface.SetCursorToID(archiveID) {
		t.Fatal("archive action was not selectable")
	}

	fixture.projects = fixture.projects[:1]
	_, cmd := hub.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Fatalf("stale project action still emitted %#v", cmd())
	}
	if hub.notice != "project beta is no longer available" {
		t.Fatalf("stale project action notice = %q, want the executor error", hub.notice)
	}
}

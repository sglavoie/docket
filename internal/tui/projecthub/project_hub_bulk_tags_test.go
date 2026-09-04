package projecthub

import (
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/listsurface"
)

// selectProjectHubRows selects each named row and reports the resulting count,
// driving the public v key rather than the surface API so the test fails if the
// browse grammar stops reaching selection.
func selectProjectHubRows(t *testing.T, hub *ProjectHubModel, ids ...listsurface.ListItemID) {
	t.Helper()
	for _, id := range ids {
		focusProjectHubItem(t, hub, id)
		hub.Update(syntheticKey('v'))
	}
	if hub.selectionCount() != len(ids) {
		t.Fatalf("selected %d rows, want %d (notice %q)", hub.selectionCount(), len(ids), hub.notice)
	}
}

func TestProjectHubSelectionOpensExactlyTheBulkTagActions(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()
	selectProjectHubRows(t, hub, "project-hub:project:alpha", "project-hub:project:beta")

	hub.Update(syntheticKey(' '))
	if hub.actions == nil {
		t.Fatalf("Space over a selection opened no bulk sheet (notice %q)", hub.notice)
	}
	if got := projectHubActionIDs(hub); !reflect.DeepEqual(got, projectBulkOperations) {
		t.Fatalf("bulk sheet offers %v, want %v", got, projectBulkOperations)
	}
}

// Retiring A left Esc as the only way to drop every checkbox at once, so the
// selection is an unwind rung ahead of closing the Hub.
func TestProjectHubSelectEveryMatchThenClear(t *testing.T) {
	hub := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	).open()

	hub.Update(syntheticCtrlKey('a'))
	if hub.selectionCount() != len(hub.surface.Matches()) {
		t.Fatalf("Ctrl+A selected %d of %d matches", hub.selectionCount(), len(hub.surface.Matches()))
	}
	if !hub.ShouldCaptureEsc() {
		t.Fatal("a selection did not claim Esc")
	}
	hub.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if hub.selectionCount() != 0 {
		t.Fatalf("Esc left %d rows selected", hub.selectionCount())
	}
}

// An archived project takes no tag edit, so its row carries none of the bulk
// operations and the sheet's intersection empties rather than offering an action
// half the selection would refuse.
func TestProjectHubBulkSheetRefusesAnArchivedSelection(t *testing.T) {
	fixture := newProjectHubFixture(t,
		scopeTestProject("alpha", "Alpha", "1", "2"),
		scopeTestProject("beta", "Beta", "1"),
	)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	fixture.add(archived)
	hub := fixture.open()
	selectProjectHubRows(t, hub, "project-hub:project:alpha", "project-hub:archived:archived")

	hub.Update(syntheticKey(' '))
	if hub.actions != nil {
		t.Fatalf("mixed selection opened %v", projectHubActionIDs(hub))
	}
	if !strings.Contains(hub.notice, "active projects") {
		t.Fatalf("mixed selection notice = %q", hub.notice)
	}
}

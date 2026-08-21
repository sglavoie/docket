package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// selectProjectHubRows selects each named row and reports the resulting count,
// driving the public v key rather than the surface API so the test fails if the
// browse grammar stops reaching selection.
func selectProjectHubRows(t *testing.T, hub *ProjectHubModel, ids ...ListItemID) {
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
	app := projectHubApp(t)
	hub := openProjectHub(t, app)
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
	app := projectHubApp(t)
	hub := openProjectHub(t, app)

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
	app := projectHubApp(t)
	archived := scopeTestProject("archived", "Archived", "1")
	archived.Archived = true
	addTestAppProject(app, archived)
	hub := openProjectHub(t, app)
	selectProjectHubRows(t, hub, "project-hub:project:alpha", "project-hub:archived:archived")

	hub.Update(syntheticKey(' '))
	if hub.actions != nil {
		t.Fatalf("mixed selection opened %v", projectHubActionIDs(hub))
	}
	if !strings.Contains(hub.notice, "active projects") {
		t.Fatalf("mixed selection notice = %q", hub.notice)
	}
}

func TestProjectBulkAddTagCommitsTheWholeSelectionAtOnce(t *testing.T) {
	app := projectHubApp(t)
	if cmd := app.requestProjectBulkTags(ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     ProjectBulkTagAdd,
		Tag:        "Focus",
	}); cmd == nil {
		t.Fatal("bulk add emitted no follow-up command")
	}
	for _, projectID := range []string{"alpha", "beta"} {
		project := app.projectByID(projectID)
		if project == nil || !slices.Contains(project.Tags, "focus") {
			t.Fatalf("project %s tags = %v, want the normalized tag", projectID, project.Tags)
		}
	}
	if !strings.Contains(app.statusMessage, "Tagged 2 projects #focus") {
		t.Fatalf("bulk add status = %q", app.statusMessage)
	}
}

// The prompt follows the resolved effect, not the chosen action, so typing the
// reserved tag into Add tag confirms exactly like Park does.
func TestProjectBulkAddingTheReservedTagConfirmsBeforeCommitting(t *testing.T) {
	app := projectHubApp(t)
	if cmd := app.requestProjectBulkTags(ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     ProjectBulkTagAdd,
		Tag:        models.ParkedProjectTag,
	}); cmd != nil {
		t.Fatalf("parking bulk edit committed without confirmation: %#v", cmd())
	}
	if !app.projectParkConfirm.IsActive() {
		t.Fatal("parking bulk edit raised no confirmation")
	}
	if slices.Contains(app.projectByID("alpha").Tags, models.ParkedProjectTag) {
		t.Fatal("parking bulk edit wrote before the answer")
	}

	handled, _ := app.handleProjectMessages(projectParkConfirmedMsg{})
	if !handled {
		t.Fatal("park confirmation was not handled")
	}
	for _, projectID := range []string{"alpha", "beta"} {
		if !slices.Contains(app.projectByID(projectID).Tags, models.ParkedProjectTag) {
			t.Fatalf("project %s was not parked after confirmation", projectID)
		}
	}
	if app.pendingProjectBulkTags != nil {
		t.Fatal("confirmed bulk edit stayed pending")
	}
	if !strings.Contains(app.statusMessage, "Parked 2 projects") {
		t.Fatalf("bulk park status = %q", app.statusMessage)
	}
}

func TestProjectBulkTagNoChangeReportsInsteadOfWriting(t *testing.T) {
	app := projectHubApp(t)
	cmd := app.requestProjectBulkTags(ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     ProjectBulkTagUnpark,
	})
	if cmd == nil {
		t.Fatal("a no-op bulk edit reported nothing")
	}
	msg, ok := cmd().(ErrorMsg)
	if !ok || !strings.Contains(msg.Err.Error(), "no selected project is parked") {
		t.Fatalf("no-op bulk edit emitted %#v", cmd())
	}
	if app.statusMessage != "" {
		t.Fatalf("no-op bulk edit reported success %q", app.statusMessage)
	}
}

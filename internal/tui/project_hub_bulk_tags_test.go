package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/projecthub"
	"github.com/sglavoie/tt/internal/tui/status"
)

// The Hub's own selection and sheet coverage moved with the surface into
// internal/tui/projecthub. What stays here is the shell half: the bulk tag
// request the sheet raises, and the catalog transaction it commits.

func TestProjectBulkAddTagCommitsTheWholeSelectionAtOnce(t *testing.T) {
	app := projectHubApp(t)
	if cmd := app.requestProjectBulkTags(projecthub.ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     projecthub.ProjectBulkTagAdd,
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
	if cmd := app.requestProjectBulkTags(projecthub.ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     projecthub.ProjectBulkTagAdd,
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

	handled, _ := app.handleProjectMessages(projecthub.ProjectParkConfirmedMsg{})
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
	cmd := app.requestProjectBulkTags(projecthub.ProjectBulkTagsMsg{
		ProjectIDs: []string{"alpha", "beta"},
		Action:     projecthub.ProjectBulkTagUnpark,
	})
	if cmd == nil {
		t.Fatal("a no-op bulk edit reported nothing")
	}
	msg, ok := cmd().(status.ErrorMsg)
	if !ok || !strings.Contains(msg.Err.Error(), "no selected project is parked") {
		t.Fatalf("no-op bulk edit emitted %#v", cmd())
	}
	if app.statusMessage != "" {
		t.Fatalf("no-op bulk edit reported success %q", app.statusMessage)
	}
}

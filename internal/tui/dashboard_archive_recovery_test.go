package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestDashboardUnarchiveBindingOnlyWorksInArchivedMode(t *testing.T) {
	cfg := testutil.NewTestConfig()
	active := testutil.NewTestProject("Active Project")
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	cfg.Projects = []*models.Project{active, archived}

	dashboard := NewDashboardModel(cfg, nil)

	if _, _, consumed := dashboard.Dispatcher.Dispatch(syntheticKey('u')); consumed {
		t.Fatal("dashboard u binding should not be active outside archived mode")
	}

	_, _, consumed := dashboard.Dispatcher.Dispatch(syntheticKey('a'))
	if !consumed {
		t.Fatal("dashboard a binding should switch to archived mode")
	}
	if !dashboard.showArchived {
		t.Fatal("dashboard should be in archived mode")
	}

	_, cmd, consumed := dashboard.Dispatcher.Dispatch(syntheticKey('u'))
	if !consumed {
		t.Fatal("dashboard u binding should be active in archived mode")
	}
	if cmd == nil {
		t.Fatal("dashboard u binding should return a project archive request command")
	}

	result := cmd()
	msg, ok := result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("dashboard u returned %T, want ProjectArchiveRequestMsg", result)
	}
	if msg.Project != archived {
		t.Fatal("dashboard u should request unarchive for the selected archived project")
	}
	if msg.Archive {
		t.Fatal("dashboard u should request unarchive, not archive")
	}
	if !archived.Archived {
		t.Fatal("dashboard u should not mutate the project before confirmation")
	}
}

func TestDashboardHelpShowsUnarchiveOnlyInArchivedMode(t *testing.T) {
	cfg := testutil.NewTestConfig()
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	cfg.Projects = []*models.Project{archived}

	dashboard := NewDashboardModel(cfg, nil)

	if desc := dashboardHelpDescription(dashboard.GetHelpItems(), "a"); desc != "Show archived" {
		t.Fatalf("active dashboard a help = %q, want Show archived", desc)
	}
	if desc := dashboardHelpDescription(dashboard.GetHelpItems(), "u"); desc != "" {
		t.Fatalf("active dashboard should not show u help, got %q", desc)
	}

	dashboard.showArchived = true
	dashboard.loadProjects()

	if desc := dashboardHelpDescription(dashboard.GetHelpItems(), "a"); desc != "Show active" {
		t.Fatalf("archived dashboard a help = %q, want Show active", desc)
	}
	if desc := dashboardHelpDescription(dashboard.GetHelpItems(), "u"); desc != "Unarchive project" {
		t.Fatalf("archived dashboard u help = %q, want Unarchive project", desc)
	}
}

func TestDashboardUnarchiveConfirmRestoresAndRefreshesArchivedList(t *testing.T) {
	app := newTestAppModel()
	archived := testutil.NewTestProject("SIS Shield")
	archived.Archive()
	app.config.Projects = []*models.Project{archived}
	app.dashboard = NewDashboardModel(app.config, app.configManager)
	app.dashboard.showArchived = true
	app.dashboard.loadProjects()
	app.views.Register(ViewDashboard, app.dashboard)

	_, requestCmd, consumed := app.dashboard.Dispatcher.Dispatch(syntheticKey('u'))
	if !consumed {
		t.Fatal("dashboard u binding should be consumed in archived mode")
	}
	if requestCmd == nil {
		t.Fatal("dashboard u binding should return a request command")
	}
	requestResult := requestCmd()
	request, ok := requestResult.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("dashboard u command returned %T, want ProjectArchiveRequestMsg", requestResult)
	}

	handled, cmd := app.handleProjectMessages(request)
	if !handled {
		t.Fatal("ProjectArchiveRequestMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("ProjectArchiveRequestMsg should only open confirmation, got cmd %T", cmd)
	}
	if !archived.Archived {
		t.Fatal("project should still be archived before confirmation")
	}
	if !app.projectArchiveConfirm.IsActive() {
		t.Fatal("app project archive confirmation should be active")
	}

	_, confirmCmd := app.Update(syntheticKey('y'))
	if confirmCmd == nil {
		t.Fatal("confirming dashboard unarchive through app should return a confirmation command")
	}
	confirmResult := confirmCmd()
	confirmed, ok := confirmResult.(projectArchiveConfirmedMsg)
	if !ok {
		t.Fatalf("confirm command returned %T, want projectArchiveConfirmedMsg", confirmResult)
	}

	handled, restoreCmd := app.handleProjectMessages(confirmed)
	if !handled {
		t.Fatal("projectArchiveConfirmedMsg should be handled")
	}
	if restoreCmd == nil {
		t.Fatal("projectArchiveConfirmedMsg should return a restore command")
	}
	restoreResult := restoreCmd()
	restored, ok := restoreResult.(ProjectRestoredMsg)
	if !ok {
		t.Fatalf("restore command returned %T, want ProjectRestoredMsg", restoreResult)
	}

	handled, _ = app.handleProjectMessages(restored)
	if !handled {
		t.Fatal("ProjectRestoredMsg should be handled")
	}
	if archived.Archived {
		t.Fatal("project should be unarchived after confirmation")
	}
	if len(app.dashboard.projects) != 0 {
		t.Fatalf("archived dashboard list length = %d, want 0", len(app.dashboard.projects))
	}
	if app.dashboard.cursor != 0 {
		t.Fatalf("dashboard cursor = %d, want 0 after archived list shrinks", app.dashboard.cursor)
	}
	if app.statusMessage != "Unarchived SIS Shield" {
		t.Fatalf("statusMessage = %q, want Unarchived SIS Shield", app.statusMessage)
	}
}

func dashboardHelpDescription(items []HelpItem, key string) string {
	for _, item := range items {
		if item.Key == key {
			return item.Description
		}
	}
	return ""
}

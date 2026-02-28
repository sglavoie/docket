package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
)

func TestHandleProjectMessagesSelectProject(t *testing.T) {
	app := newTestAppModel()
	project := testutil.NewTestProjectWithTasks("New Project")

	msg := SelectProjectMsg{Project: project}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("SelectProjectMsg should be handled")
	}
	if app.currentProject != project {
		t.Error("currentProject should be set to selected project")
	}
	if len(app.viewStack) != 2 {
		t.Errorf("viewStack should have 2 views after select, got %d", len(app.viewStack))
	}
	if app.currentView() != ViewOutliner {
		t.Errorf("currentView should be ViewOutliner, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesCreateProject(t *testing.T) {
	app := newTestAppModel()

	msg := CreateProjectMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("CreateProjectMsg should be handled")
	}
	if app.currentView() != ViewProjectCreator {
		t.Errorf("currentView should be ProjectCreator, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesProjectCreated(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectCreator)

	msg := ProjectCreatedMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectCreatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after project created, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesProjectArchived(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectTree)

	msg := ProjectArchivedMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectArchivedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after project archived, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesProjectPinnedToggled(t *testing.T) {
	app, project := newTestAppModelWithProject()

	tests := []struct {
		name       string
		pinned     bool
		wantStatus string
	}{
		{"pin project", true, "Pinned"},
		{"unpin project", false, "Unpinned"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.statusMessage = ""
			project.Pinned = tt.pinned

			msg := ProjectPinnedToggledMsg{Project: project}
			handled, _ := app.handleProjectMessages(msg)

			if !handled {
				t.Error("ProjectPinnedToggledMsg should be handled")
			}
			if app.statusMessage != tt.wantStatus {
				t.Errorf("statusMessage = %q, want %q", app.statusMessage, tt.wantStatus)
			}
		})
	}
}

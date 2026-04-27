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

func TestHandleProjectMessagesProjectDeleted(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.pushView(ViewProjectTree)

	msg := ProjectDeletedMsg{Project: project}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectDeletedMsg should be handled")
	}
	if app.currentProject != nil {
		t.Error("currentProject should be nil after project deleted")
	}
	if len(app.viewStack) != 1 {
		t.Errorf("viewStack should have 1 view (Dashboard) after project deleted, got %d", len(app.viewStack))
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after project deleted, got %v", app.currentView())
	}
	// Verify project was removed from config
	for _, p := range app.config.Projects {
		if p.ID == project.ID {
			t.Error("project should be removed from config.Projects")
		}
	}
}

func TestHandleProjectMessagesEditProject(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := EditProjectMsg{Project: project}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("EditProjectMsg should be handled")
	}
	if app.currentView() != ViewProjectEditor {
		t.Errorf("currentView should be ProjectEditor, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesEditProjectNil(t *testing.T) {
	app := newTestAppModel()

	msg := EditProjectMsg{Project: nil}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("EditProjectMsg with nil project should still be handled")
	}
	if app.currentView() == ViewProjectEditor {
		t.Error("currentView should not be ProjectEditor when project is nil")
	}
}

func TestHandleProjectMessagesProjectUpdated(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pushView(ViewProjectEditor)

	msg := ProjectUpdatedMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectUpdatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after project updated, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesToggleInfoPanel(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := ToggleInfoPanelMsg{}
	handled, cmd := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ToggleInfoPanelMsg should be handled")
	}
	if app.statusMessage == "" {
		t.Error("statusMessage should be set after toggling info panel")
	}
	if cmd == nil {
		t.Error("ToggleInfoPanelMsg should return a status timeout command")
	}
}

func TestHandleProjectMessagesTutorDismissed(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := TutorDismissedMsg{}
	handled, cmd := app.handleProjectMessages(msg)

	if !handled {
		t.Error("TutorDismissedMsg should be handled")
	}
	if app.statusMessage != "Tutor prompt dismissed" {
		t.Errorf("statusMessage should be 'Tutor prompt dismissed', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("TutorDismissedMsg should return a status timeout command")
	}
}

func TestHandleProjectMessagesShowTrashedProjects(t *testing.T) {
	app := newTestAppModel()

	msg := ShowTrashedProjectsMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ShowTrashedProjectsMsg should be handled")
	}
	if app.currentView() != ViewTrashedProjects {
		t.Errorf("currentView should be TrashedProjects, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesCloseTrashedProjects(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTrashedProjects)

	app.Update(CloseTrashedProjectsMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing trashed projects, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesProjectRestored(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := ProjectRestoredMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectRestoredMsg should be handled")
	}
	if app.err != nil {
		t.Errorf("unexpected error after ProjectRestoredMsg: %v", app.err)
	}
}

func TestHandleProjectMessagesShowProjectPicker(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := ShowProjectPickerMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ShowProjectPickerMsg should be handled")
	}
	if app.currentView() != ViewProjectPicker {
		t.Errorf("currentView should be ProjectPicker, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesSandboxResetDone(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pendingSandboxReset = true
	app.pendingSandboxTaskCount = 5
	app.dashboard = NewDashboardModel(app.config, nil)

	msg := SandboxResetDoneMsg{TaskCount: 3}
	handled, cmd := app.handleProjectMessages(msg)

	if !handled {
		t.Error("SandboxResetDoneMsg should be handled")
	}
	if app.pendingSandboxReset {
		t.Error("pendingSandboxReset should be false after SandboxResetDoneMsg")
	}
	if app.pendingSandboxTaskCount != 0 {
		t.Errorf("pendingSandboxTaskCount should be 0, got %d", app.pendingSandboxTaskCount)
	}
	if app.statusMessage != "Cleared 3 item(s) from sandbox" {
		t.Errorf("statusMessage should contain clear count, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("SandboxResetDoneMsg should return a status timeout command")
	}
}

package tui

import (
	"path/filepath"
	"testing"

	appdomain "github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
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
	app.pushView(ViewOutliner)

	msg := ProjectArchivedMsg{}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("ProjectArchivedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after project archived, got %v", app.currentView())
	}
}

func TestHandleProjectMessagesProjectArchiveRequestShowsConfirmWithoutMutation(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.dashboard = NewDashboardModel(app.runtime)
	app.views.Register(ViewDashboard, app.dashboard)

	handled, cmd := app.handleProjectMessages(ProjectArchiveRequestMsg{Project: project, Archive: true})

	if !handled {
		t.Fatal("ProjectArchiveRequestMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("ProjectArchiveRequestMsg should only open confirmation, got cmd %T", cmd)
	}
	if project.Archived {
		t.Fatal("project should not be archived before confirmation")
	}
	if app.pendingProjectArchive != project {
		t.Fatal("pendingProjectArchive should store the requested project")
	}
	if !app.pendingProjectArchiveArchive {
		t.Fatal("pendingProjectArchiveArchive should store requested archive state")
	}
	confirm := &app.projectArchiveConfirm
	if !confirm.IsActive() {
		t.Fatal("app project archive confirmation should be active")
	}
	if confirm.Action() != projectArchiveConfirmAction {
		t.Fatalf("confirm action = %q, want %q", confirm.Action(), projectArchiveConfirmAction)
	}
	if app.dashboard.GetConfirmDialog().IsActive() {
		t.Fatal("dashboard confirmation should not host project archive confirmation")
	}
}

func TestHandleProjectMessagesProjectArchiveRequestShowsConfirmFromNonProviderView(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.views.Register(ViewHelp, NewHelpViewModel(nil, ViewDashboard, "Help"))
	app.viewStack = []PresentationID{ViewDashboard, ViewHelp}

	handled, cmd := app.handleProjectMessages(ProjectArchiveRequestMsg{Project: project, Archive: true})

	if !handled {
		t.Fatal("ProjectArchiveRequestMsg should be handled")
	}
	if cmd != nil {
		t.Fatalf("ProjectArchiveRequestMsg should only open confirmation, got cmd %T", cmd)
	}
	if project.Archived {
		t.Fatal("project should not be archived before confirmation")
	}
	if app.pendingProjectArchive != project {
		t.Fatal("pendingProjectArchive should store the requested project")
	}
	if !app.projectArchiveConfirm.IsActive() {
		t.Fatal("app project archive confirmation should be active from non-provider views")
	}
}

func TestHandleProjectMessagesProjectArchiveConfirmedArchivesAndSetsStatus(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.pendingProjectArchive = project
	app.pendingProjectArchiveArchive = true

	handled, cmd := app.handleProjectMessages(projectArchiveConfirmedMsg{})

	if !handled {
		t.Fatal("projectArchiveConfirmedMsg should be handled")
	}
	if !project.Archived {
		t.Fatal("project should be archived after confirmation")
	}
	if cmd == nil {
		t.Fatal("projectArchiveConfirmedMsg should return a result command")
	}
	result := cmd()
	msg, ok := result.(ProjectArchivedMsg)
	if !ok {
		t.Fatalf("confirmation cmd returned %T, want ProjectArchivedMsg", result)
	}
	handled, _ = app.handleProjectMessages(msg)
	if !handled {
		t.Fatal("ProjectArchivedMsg should be handled")
	}
	if app.runtime.Machine().CurrentProject != "" {
		t.Fatalf("CurrentProject = %q, want cleared after archiving current project", app.runtime.Machine().CurrentProject)
	}
	if app.currentProject != nil {
		t.Fatal("currentProject should be nil after archiving current project")
	}
	if app.currentView() != ViewDashboard {
		t.Fatalf("currentView = %v, want Dashboard after archiving current project", app.currentView())
	}
	if app.statusMessage != "Archived Test Project" {
		t.Fatalf("statusMessage = %q, want %q", app.statusMessage, "Archived Test Project")
	}
}

func TestConfirmProjectArchiveLoadsQualifiedCleanupTargets(t *testing.T) {
	m := newTestAppModel()
	project, err := models.NewProject("archive-project", "Archive Project", "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	project.Path = filepath.Join(t.TempDir(), "projects", project.Slug)
	task, err := models.NewTask("shared-id", 1, models.TaskTypeTask, "Shared")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}
	project.NextShortID = 2
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	project.Tasks = nil // Catalog/dashboard snapshots are intentionally unloaded.
	setTestAppProjects(m, project)
	m.pendingProjectArchive = project
	m.pendingProjectArchiveArchive = true

	cmd := m.confirmProjectArchive()
	if cmd == nil {
		t.Fatal("confirmProjectArchive returned nil")
	}
	raw := cmd()
	msg, ok := raw.(ProjectArchivedMsg)
	if !ok {
		t.Fatalf("archive command returned %T", raw)
	}
	if msg.Project != project || !project.Archived {
		t.Fatalf("archive result = %#v, want archived project", msg)
	}
	if len(project.Tasks) != 1 || project.Tasks[0].ID != task.ID {
		t.Fatalf("archive cleanup targets were not loaded: %#v", project.Tasks)
	}
}

func TestCommandPaletteProjectArchiveClosesPaletteAndShowsAppConfirm(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.views.Register(ViewHelp, NewHelpViewModel(nil, ViewDashboard, "Help"))
	app.viewStack = []PresentationID{ViewDashboard, ViewHelp, ViewCommandPalette}

	_, cmd := app.Update(CommandPaletteExecuteMsg{
		ResultMsg: ProjectArchiveRequestMsg{Project: project, Archive: true},
	})

	if cmd != nil {
		t.Fatalf("palette archive request should only open confirmation, got cmd %T", cmd)
	}
	if app.currentView() != ViewHelp {
		t.Fatalf("currentView = %v, want underlying Help view after palette closes", app.currentView())
	}
	if project.Archived {
		t.Fatal("project should not be archived before confirmation")
	}
	if app.pendingProjectArchive != project {
		t.Fatal("pendingProjectArchive should store the palette-selected project")
	}
	if !app.pendingProjectArchiveArchive {
		t.Fatal("pendingProjectArchiveArchive should request archive")
	}
	if !app.projectArchiveConfirm.IsActive() {
		t.Fatal("app project archive confirmation should be active after palette archive")
	}
}

func TestCommandPaletteProjectRestoreFromHelpConfirmsAtAppLevel(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.Archive()
	app.views.Register(ViewHelp, NewHelpViewModel(nil, ViewDashboard, "Help"))
	app.viewStack = []PresentationID{ViewDashboard, ViewHelp, ViewCommandPalette}

	_, cmd := app.Update(CommandPaletteExecuteMsg{
		ResultMsg: ProjectArchiveRequestMsg{Project: project, Archive: false},
	})

	if cmd != nil {
		t.Fatalf("palette restore request should only open confirmation, got cmd %T", cmd)
	}
	if app.currentView() != ViewHelp {
		t.Fatalf("currentView = %v, want underlying Help view after palette closes", app.currentView())
	}
	if !project.Archived {
		t.Fatal("project should remain archived before confirmation")
	}
	if app.pendingProjectArchive != project {
		t.Fatal("pendingProjectArchive should store the palette-selected project")
	}
	if app.pendingProjectArchiveArchive {
		t.Fatal("pendingProjectArchiveArchive should request restore")
	}
	if !app.projectArchiveConfirm.IsActive() {
		t.Fatal("app project archive confirmation should be active after palette restore")
	}

	_, confirmCmd := app.Update(syntheticKey('y'))
	if confirmCmd == nil {
		t.Fatal("confirming palette restore through app should return a confirmation command")
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
	if project.Archived {
		t.Fatal("project should be restored after confirmation")
	}
	if app.currentView() != ViewHelp {
		t.Fatalf("currentView = %v, want Help after restoring from palette", app.currentView())
	}
	if app.statusMessage != "Unarchived Test Project" {
		t.Fatalf("statusMessage = %q, want Unarchived Test Project", app.statusMessage)
	}
}

func TestHandleProjectMessagesProjectArchiveConfirmedUnarchivesAndSetsStatus(t *testing.T) {
	app, project := newTestAppModelWithProject()
	project.Archive()
	app.pendingProjectArchive = project
	app.pendingProjectArchiveArchive = false

	handled, cmd := app.handleProjectMessages(projectArchiveConfirmedMsg{})

	if !handled {
		t.Fatal("projectArchiveConfirmedMsg should be handled")
	}
	if project.Archived {
		t.Fatal("project should be unarchived after confirmation")
	}
	if cmd == nil {
		t.Fatal("projectArchiveConfirmedMsg should return a result command")
	}
	result := cmd()
	msg, ok := result.(ProjectRestoredMsg)
	if !ok {
		t.Fatalf("confirmation cmd returned %T, want ProjectRestoredMsg", result)
	}
	handled, _ = app.handleProjectMessages(msg)
	if !handled {
		t.Fatal("ProjectRestoredMsg should be handled")
	}
	if app.statusMessage != "Unarchived Test Project" {
		t.Fatalf("statusMessage = %q, want %q", app.statusMessage, "Unarchived Test Project")
	}
}

func TestProjectArchiveCommandsReturnRequestsWithoutMutation(t *testing.T) {
	app, project := newTestAppModelWithProject()

	outliner := NewOutlinerModel(project, app.configManager, app.runtime)
	result := outliner.archiveProject()()
	msg, ok := result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("outliner archiveProject returned %T, want ProjectArchiveRequestMsg", result)
	}
	if !msg.Archive {
		t.Fatal("outliner archiveProject should request archive for active project")
	}
	if project.Archived {
		t.Fatal("outliner archiveProject should not mutate before confirmation")
	}

	project.Archive()
	result = outliner.archiveProject()()
	msg, ok = result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("outliner archiveProject returned %T for archived project, want ProjectArchiveRequestMsg", result)
	}
	if msg.Archive {
		t.Fatal("outliner archiveProject should request unarchive for archived project")
	}
	if !project.Archived {
		t.Fatal("outliner archiveProject should not mutate before confirmation")
	}
}

func TestOutlinerCtrlARequestsProjectArchiveConfirmation(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)

	_, cmd, handled := outliner.Dispatcher.Dispatch(syntheticCtrlKey('a'))
	if !handled {
		t.Fatal("outliner ctrl+a should be handled")
	}
	if cmd == nil {
		t.Fatal("outliner ctrl+a should return a project archive request command")
	}
	result := cmd()
	msg, ok := result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("outliner ctrl+a returned %T, want ProjectArchiveRequestMsg", result)
	}
	if msg.Project != project {
		t.Fatal("outliner ctrl+a should target the current project")
	}
	if !msg.Archive {
		t.Fatal("outliner ctrl+a should request archive for an active project")
	}
	if project.Archived {
		t.Fatal("outliner ctrl+a should not archive before confirmation")
	}

	project.Archive()
	_, cmd, handled = outliner.Dispatcher.Dispatch(syntheticCtrlKey('a'))
	if !handled {
		t.Fatal("outliner ctrl+a should be handled for archived projects")
	}
	if cmd == nil {
		t.Fatal("outliner ctrl+a should return a project unarchive request command")
	}
	result = cmd()
	msg, ok = result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("outliner ctrl+a returned %T for archived project, want ProjectArchiveRequestMsg", result)
	}
	if msg.Archive {
		t.Fatal("outliner ctrl+a should request unarchive for an archived project")
	}
	if !project.Archived {
		t.Fatal("outliner ctrl+a should not unarchive before confirmation")
	}
}

func TestPaletteProjectArchiveActionsRequestConfirmWithoutMutation(t *testing.T) {
	project := testutil.NewTestProjectWithTasks("Palette Project")
	palette := &CommandPaletteModel{}

	archiveAction := findPaletteAction(t, palette.getProjectActions(PaletteItem{Project: project}), "Archive")
	result := archiveAction.Handler()
	msg, ok := result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("Archive action returned %T, want ProjectArchiveRequestMsg", result)
	}
	if !msg.Archive {
		t.Fatal("Archive action should request archive")
	}
	if project.Archived {
		t.Fatal("Archive action should not mutate before confirmation")
	}

	project.Archive()
	restoreAction := findPaletteAction(t, palette.getProjectActions(PaletteItem{Project: project}), "Restore")
	result = restoreAction.Handler()
	msg, ok = result.(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("Restore action returned %T, want ProjectArchiveRequestMsg", result)
	}
	if msg.Archive {
		t.Fatal("Restore action should request unarchive")
	}
	if !project.Archived {
		t.Fatal("Restore action should not mutate before confirmation")
	}
}

func findPaletteAction(t *testing.T, actions []PaletteAction, name string) PaletteAction {
	t.Helper()
	for _, action := range actions {
		if action.Name == name {
			return action
		}
	}
	t.Fatalf("action %q not found", name)
	return PaletteAction{}
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
	app.pushView(ViewOutliner)

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
	for _, p := range app.allProjects() {
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

func TestHandleProjectMessagesTutorDismissed(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := TutorDismissedMsg{}
	handled, cmd := app.handleProjectMessages(msg)

	if !handled {
		t.Error("TutorDismissedMsg should be handled")
	}
	if app.statusMessage != "Reference prompt dismissed" {
		t.Errorf("statusMessage should be 'Reference prompt dismissed', got %q", app.statusMessage)
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
	app.pendingSandboxProjectID = "sandbox"
	app.pendingSandboxTaskCount = 5
	app.dashboard = NewDashboardModel(app.runtime)

	msg := SandboxResetDoneMsg{Result: appdomain.SandboxResetResult{
		ProjectID: "sandbox",
		Cleared:   appdomain.SandboxResetCounts{Tasks: 3},
	}}
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
	if app.pendingSandboxProjectID != "" {
		t.Errorf("pendingSandboxProjectID should be empty, got %q", app.pendingSandboxProjectID)
	}
	if app.statusMessage != "Cleared 3 item(s) from sandbox" {
		t.Errorf("statusMessage should contain clear count, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("SandboxResetDoneMsg should return a status timeout command")
	}
}

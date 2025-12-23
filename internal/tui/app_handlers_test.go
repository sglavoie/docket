package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newTestAppModel creates an AppModel for testing with a temp config file.
func newTestAppModel() *AppModel {
	cfg := testutil.NewTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	return &AppModel{
		config:        cfg,
		configManager: manager,
		viewStack:     []ViewType{ViewDashboard},
		width:         80,
		height:        24,
		views:         NewViewRegistry(),
	}
}

// newTestAppModelWithProject creates an AppModel with a test project and temp config.
func newTestAppModelWithProject() (*AppModel, *models.Project) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	app := &AppModel{
		config:         cfg,
		configManager:  manager,
		viewStack:      []ViewType{ViewDashboard},
		width:          80,
		height:         24,
		views:          NewViewRegistry(),
		currentProject: project,
	}

	return app, project
}

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
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should be ProjectTree, got %v", app.currentView())
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

func TestHandleTaskMessagesCreateTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := CreateTaskMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskCreator {
		t.Errorf("currentView should be TaskCreator, got %v", app.currentView())
	}

	// Verify the project is still accessible
	if app.currentProject != project {
		t.Error("currentProject should still be set")
	}
}

func TestHandleTaskMessagesCreateTaskWithParent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	parentTask := project.Tasks[0]

	msg := CreateTaskMsg{ParentTask: parentTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg with parent should be handled")
	}
}

func TestHandleTaskMessagesEditTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := EditTaskMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("EditTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskEditor {
		t.Errorf("currentView should be TaskEditor, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesTaskCreated(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pushView(ViewTaskCreator)

	msg := TaskCreatedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskCreatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after task created, got %v", app.currentView())
	}
}

func TestHandleViewMessagesBack(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectTree)
	app.pushView(ViewTaskEditor)

	msg := BackMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BackMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should be ProjectTree after back, got %v", app.currentView())
	}
}

func TestHandleViewMessagesError(t *testing.T) {
	app := newTestAppModel()
	testErr := errors.New("task not found")

	msg := ErrorMsg{Err: testErr}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ErrorMsg should be handled")
	}
	if app.err != testErr {
		t.Error("app.err should be set to the error from message")
	}
}

func TestHandleViewMessagesSettings(t *testing.T) {
	app := newTestAppModel()

	msg := SettingsMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("SettingsMsg should be handled")
	}
	if app.currentView() != ViewSettings {
		t.Errorf("currentView should be Settings, got %v", app.currentView())
	}
}

func TestHandleViewMessagesSettingsSaved(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewSettings)

	msg := SettingsSavedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("SettingsSavedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after settings saved, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowHelp(t *testing.T) {
	app := newTestAppModel()
	items := []HelpItem{{Key: "q", Description: "quit"}}

	msg := ShowHelpMsg{Items: items, SourceView: ViewDashboard, ViewName: "Dashboard"}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowHelpMsg should be handled")
	}
	if app.currentView() != ViewHelp {
		t.Errorf("currentView should be Help, got %v", app.currentView())
	}
	if app.helpView == nil {
		t.Error("helpView should be initialized")
	}
}

func TestHandleViewMessagesCloseHelp(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewHelp)

	msg := CloseHelpMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseHelpMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing help, got %v", app.currentView())
	}
}

func TestHandleViewMessagesStatusClear(t *testing.T) {
	app := newTestAppModel()
	app.statusMessage = "Some status"

	msg := StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.statusMessage)
	}
}

func TestHandleViewMessagesMoveBlocked(t *testing.T) {
	app := newTestAppModel()

	msg := MoveBlockedMsg{Reason: "Cannot move completed task"}
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("MoveBlockedMsg should be handled")
	}
	if app.statusMessage != "Cannot move completed task" {
		t.Errorf("statusMessage should be set to reason, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("MoveBlockedMsg should return a status timeout command")
	}
}

func TestHandleTagMessagesShowTagManager(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := ShowTagManagerMsg{ProjectTree: app.projectTree}
	handled, _ := app.handleTagMessages(msg)

	if !handled {
		t.Error("ShowTagManagerMsg should be handled")
	}
	if app.currentView() != ViewTagManager {
		t.Errorf("currentView should be TagManager, got %v", app.currentView())
	}
}

func TestHandleTagMessagesCloseTagManager(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTagManager)

	msg := CloseTagManagerMsg{}
	handled, _ := app.handleTagMessages(msg)

	if !handled {
		t.Error("CloseTagManagerMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing tag manager, got %v", app.currentView())
	}
}

func TestHandleSectionMessagesShowSectionPicker(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowSectionPickerMsg{Task: task, Project: project}
	handled, _ := app.handleSectionMessages(msg)

	if !handled {
		t.Error("ShowSectionPickerMsg should be handled")
	}
	if app.currentView() != ViewSectionPicker {
		t.Errorf("currentView should be SectionPicker, got %v", app.currentView())
	}
}

func TestHandleClipboardMessagesTaskCopied(t *testing.T) {
	app := newTestAppModel()

	tests := []struct {
		name       string
		count      int
		fullDetail bool
		wantStatus string
	}{
		{
			name:       "single title",
			count:      1,
			fullDetail: false,
			wantStatus: "Copied title to clipboard",
		},
		{
			name:       "single description",
			count:      1,
			fullDetail: true,
			wantStatus: "Copied description to clipboard",
		},
		{
			name:       "multiple titles",
			count:      3,
			fullDetail: false,
			wantStatus: "Copied title for 3 tasks to clipboard",
		},
		{
			name:       "multiple descriptions",
			count:      5,
			fullDetail: true,
			wantStatus: "Copied description for 5 tasks to clipboard",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app.statusMessage = ""
			msg := TaskCopiedMsg{Count: tt.count, FullDetail: tt.fullDetail}
			handled, _ := app.handleClipboardMessages(msg)

			if !handled {
				t.Error("TaskCopiedMsg should be handled")
			}
			if app.statusMessage != tt.wantStatus {
				t.Errorf("statusMessage = %q, want %q", app.statusMessage, tt.wantStatus)
			}
		})
	}
}

func TestViewStackNavigation(t *testing.T) {
	app := newTestAppModel()

	// Initially at dashboard
	if app.currentView() != ViewDashboard {
		t.Error("should start at Dashboard")
	}
	if len(app.viewStack) != 1 {
		t.Errorf("viewStack should have 1 item initially, got %d", len(app.viewStack))
	}

	// Push some views
	app.pushView(ViewProjectTree)
	if app.currentView() != ViewProjectTree {
		t.Error("currentView should be ProjectTree after push")
	}

	app.pushView(ViewTaskEditor)
	if app.currentView() != ViewTaskEditor {
		t.Error("currentView should be TaskEditor after second push")
	}
	if len(app.viewStack) != 3 {
		t.Errorf("viewStack should have 3 items, got %d", len(app.viewStack))
	}

	// Pop back
	app.popView()
	if app.currentView() != ViewProjectTree {
		t.Error("currentView should be ProjectTree after pop")
	}

	app.popView()
	if app.currentView() != ViewDashboard {
		t.Error("currentView should be Dashboard after second pop")
	}

	// Pop at dashboard should not crash or go below 1
	app.popView()
	if app.currentView() != ViewDashboard {
		t.Error("currentView should still be Dashboard after pop at bottom")
	}
	if len(app.viewStack) != 1 {
		t.Errorf("viewStack should not go below 1, got %d", len(app.viewStack))
	}
}

func TestStatusTimeout(t *testing.T) {
	cmd := statusTimeout(100 * time.Millisecond)
	if cmd == nil {
		t.Error("statusTimeout should return a command")
	}

	// Execute the command and verify it returns StatusClearMsg
	msg := cmd()
	if _, ok := msg.(StatusClearMsg); !ok {
		t.Errorf("statusTimeout should return StatusClearMsg, got %T", msg)
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

func TestHandleProjectMessagesSetCurrentProject(t *testing.T) {
	app := newTestAppModel()
	project := testutil.NewTestProject("New Current Project")

	msg := SetCurrentProjectMsg{Project: project}
	handled, _ := app.handleProjectMessages(msg)

	if !handled {
		t.Error("SetCurrentProjectMsg should be handled")
	}
	if app.config.CurrentProject != project.ID {
		t.Error("config.CurrentProject should be set to project ID")
	}
	if app.statusMessage == "" {
		t.Error("statusMessage should be set for project change")
	}
}

func TestErrorCmd(t *testing.T) {
	cmd := errorCmd("test error: %s", "details")
	if cmd == nil {
		t.Fatal("errorCmd should return a command")
	}

	msg := cmd()
	errMsg, ok := msg.(ErrorMsg)
	if !ok {
		t.Fatalf("errorCmd should return ErrorMsg, got %T", msg)
	}
	if errMsg.Err == nil {
		t.Error("ErrorMsg.Err should not be nil")
	}
	if errMsg.Err.Error() != "test error: details" {
		t.Errorf("error message = %q, want %q", errMsg.Err.Error(), "test error: details")
	}
}

func TestUnhandledMessages(t *testing.T) {
	app := newTestAppModel()

	// Create a custom message type that shouldn't be handled
	type unknownMsg struct{}

	handled, _ := app.handleProjectMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleProjectMessages")
	}

	handled, _ = app.handleTaskMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleTaskMessages")
	}

	handled, _ = app.handleViewMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleViewMessages")
	}

	handled, _ = app.handleTagMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleTagMessages")
	}

	handled, _ = app.handleSectionMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleSectionMessages")
	}

	handled, _ = app.handleClipboardMessages(unknownMsg{})
	if handled {
		t.Error("unknown message should not be handled by handleClipboardMessages")
	}
}

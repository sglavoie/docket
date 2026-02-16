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
		persister:     NewProjectService(),
		viewStack:     []ViewType{ViewDashboard},
		width:         80,
		height:        24,
		views:         NewViewRegistry(),
		navHistory:    NewNavHistory(),
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
		persister:      NewProjectService(),
		viewStack:      []ViewType{ViewDashboard},
		width:          80,
		height:         24,
		views:          NewViewRegistry(),
		currentProject: project,
		navHistory:     NewNavHistory(),
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
	if !errors.Is(app.err, testErr) {
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

// =============================================================================
// Additional handler_task.go Tests
// =============================================================================

func TestHandleTaskMessagesTaskDeleted(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := TaskDeletedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskDeletedMsg should be handled")
	}
}

func TestHandleTaskMessagesTasksArchived(t *testing.T) {
	app := newTestAppModel()

	msg := TasksArchivedMsg{Count: 5}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TasksArchivedMsg should be handled")
	}
	if app.statusMessage != "Archived 5 task(s) to completed.md" {
		t.Errorf("statusMessage should contain archive count, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("TasksArchivedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesTaskStatusChanged(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := TaskStatusChangedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskStatusChangedMsg should be handled")
	}
}

func TestHandleTaskMessagesSetDueDate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := SetDueDateMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetDueDateMsg should be handled")
	}
	if app.currentView() != ViewDateInput {
		t.Errorf("currentView should be DateInput, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesSetDueDateNilTask(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := SetDueDateMsg{Task: nil, Project: nil}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("SetDueDateMsg with nil task should still be handled")
	}
	if app.currentView() == ViewDateInput {
		t.Error("currentView should not be DateInput when task is nil")
	}
}

func TestHandleTaskMessagesManageDependencies(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ManageDependenciesMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ManageDependenciesMsg should be handled")
	}
	if app.currentView() != ViewDependencySelector {
		t.Errorf("currentView should be DependencySelector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesDependencySelectorDone(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	app.pushView(ViewDependencySelector)

	msg := DependencySelectorDoneMsg{HasChanges: false, Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DependencySelectorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowDependencyViewer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowDependencyViewerMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDependencyViewer {
		t.Errorf("currentView should be DependencyViewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseDependencyViewer(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDependencyViewer)

	msg := CloseDependencyViewerMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing viewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowUndoHistory(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowUndoHistoryMsg{Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewUndoHistory {
		t.Errorf("currentView should be UndoHistory, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseUndoHistory(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewUndoHistory)

	msg := CloseUndoHistoryMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing undo history, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesUndoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewUndoHistory)

	msg := UndoPerformedMsg{Action: "Task deleted"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("UndoPerformedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after undo, got %v", app.currentView())
	}
	if app.statusMessage != "Undid: Task deleted" {
		t.Errorf("statusMessage should contain undo action, got %q", app.statusMessage)
	}
}

func TestHandleTaskMessagesCloseParentSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewParentSelector)

	msg := CloseParentSelectorMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseParentSelectorMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowTaskComments(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskCommentsMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskCommentsMsg should be handled")
	}
	if app.currentView() != ViewTaskComments {
		t.Errorf("currentView should be TaskComments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskComments(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskComments)

	msg := CloseTaskCommentsMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskCommentsMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing comments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommentAdded(t *testing.T) {
	app := newTestAppModel()

	msg := CommentAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.statusMessage != "Comment added" {
		t.Errorf("statusMessage should be 'Comment added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := CommentDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentDeletedMsg should be handled")
	}
	if app.statusMessage != "Comment deleted" {
		t.Errorf("statusMessage should be 'Comment deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentEdited(t *testing.T) {
	app := newTestAppModel()

	msg := CommentEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentEditedMsg should be handled")
	}
	if app.statusMessage != "Comment updated" {
		t.Errorf("statusMessage should be 'Comment updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCloseProjectSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectSelector)

	msg := CloseProjectSelectorMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseProjectSelectorMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommandPaletteTaskActionNilTask(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteTaskActionMsg{Task: nil, Project: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteTaskActionMsg with nil task should be handled")
	}
}

func TestHandleTaskMessagesCommandPaletteBulkActionEmpty(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteBulkActionMsg{Tasks: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteBulkActionMsg with empty tasks should be handled")
	}
}

// =============================================================================
// Additional handler_view.go Tests
// =============================================================================

func TestHandleViewMessagesRefresh(t *testing.T) {
	app := newTestAppModel()
	app.dashboard = NewDashboardModel(app.config)

	msg := RefreshMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RefreshMsg should be handled")
	}
}

func TestHandleViewMessagesShowWorkflowRules(t *testing.T) {
	app := newTestAppModel()

	msg := ShowWorkflowRulesMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowWorkflowRulesMsg should be handled")
	}
	if app.currentView() != ViewWorkflowRules {
		t.Errorf("currentView should be WorkflowRules, got %v", app.currentView())
	}
}

func TestHandleViewMessagesWorkflowRulesUpdated(t *testing.T) {
	app := newTestAppModel()

	msg := WorkflowRulesUpdatedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRulesUpdatedMsg should be handled")
	}
}

func TestHandleViewMessagesWorkflowRuleCreated(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewWorkflowRuleCreator)

	msg := WorkflowRuleCreatedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRuleCreatedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after rule created, got %v", app.currentView())
	}
}

func TestHandleViewMessagesWorkflowRuleEdited(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewWorkflowRuleEditor)

	msg := WorkflowRuleEditedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("WorkflowRuleEditedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after rule edited, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFilter(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	msg := ShowFilterMsg{ProjectTree: app.projectTree}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowFilterMsg should be handled")
	}
	if app.currentView() != ViewFilter {
		t.Errorf("currentView should be Filter, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseFilter(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewFilter)

	msg := CloseFilterMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseFilterMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing filter, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseTaskJumper(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskJumper)

	msg := CloseTaskJumperMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseTaskJumperMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing jumper, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowProjectPreview(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowProjectPreviewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowProjectPreviewMsg should be handled")
	}
	if app.currentView() != ViewProjectPreview {
		t.Errorf("currentView should be ProjectPreview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseProjectPreview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectPreview)

	msg := CloseProjectPreviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseProjectPreviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing preview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowDueDateOverview(t *testing.T) {
	app := newTestAppModel()

	msg := ShowDueDateOverviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowDueDateOverviewMsg should be handled")
	}
	if app.currentView() != ViewDueDateOverview {
		t.Errorf("currentView should be DueDateOverview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseDueDateOverview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDueDateOverview)

	msg := CloseDueDateOverviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseDueDateOverviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing overview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowDailyReview(t *testing.T) {
	app := newTestAppModel()

	msg := ShowDailyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowDailyReviewMsg should be handled")
	}
	if app.currentView() != ViewDailyReview {
		t.Errorf("currentView should be DailyReview, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseDailyReview(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDailyReview)

	msg := CloseDailyReviewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseDailyReviewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing review, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowFocusView(t *testing.T) {
	app := newTestAppModel()

	msg := ShowFocusViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowFocusViewMsg should be handled")
	}
	if app.currentView() != ViewFocusView {
		t.Errorf("currentView should be FocusView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseFocusView(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewFocusView)

	msg := CloseFocusViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseFocusViewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing focus view, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowProjectFocusView(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowProjectFocusViewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowProjectFocusViewMsg should be handled")
	}
	if app.currentView() != ViewProjectFocusView {
		t.Errorf("currentView should be ProjectFocusView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseProjectFocusView(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectFocusView)

	msg := CloseProjectFocusViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseProjectFocusViewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing project focus, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowGlobalSearch(t *testing.T) {
	app := newTestAppModel()

	msg := ShowGlobalSearchMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowGlobalSearchMsg should be handled")
	}
	if app.currentView() != ViewGlobalSearch {
		t.Errorf("currentView should be GlobalSearch, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseGlobalSearch(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewGlobalSearch)

	msg := CloseGlobalSearchMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseGlobalSearchMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing search, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseCommandPalette(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCommandPalette)

	msg := CloseCommandPaletteMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseCommandPaletteMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing palette, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowPriorityView(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowPriorityViewMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowPriorityViewMsg should be handled")
	}
	if app.currentView() != ViewPriorityView {
		t.Errorf("currentView should be PriorityView, got %v", app.currentView())
	}
}

func TestHandleViewMessagesClosePriorityView(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewPriorityView)

	msg := ClosePriorityViewMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ClosePriorityViewMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing priority view, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowQuickCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowQuickCaptureMsg should be handled")
	}
	if app.currentView() != ViewQuickCapture {
		t.Errorf("currentView should be QuickCapture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseQuickCapture(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewQuickCapture)

	msg := CloseQuickCaptureMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseQuickCaptureMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing quick capture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowBulkCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowBulkCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowBulkCaptureMsg should be handled")
	}
	if app.currentView() != ViewBulkCapture {
		t.Errorf("currentView should be BulkCapture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseBulkCapture(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkCapture)

	msg := CloseBulkCaptureMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("CloseBulkCaptureMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing bulk capture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesBulkEditDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkEdit)

	msg := BulkEditDoneMsg{Applied: false}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BulkEditDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after bulk edit done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesRecurrenceEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewRecurrenceEditor)

	msg := RecurrenceEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RecurrenceEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after recurrence editor done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesTextEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTextEditor)

	msg := TextEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TextEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after text editor done, got %v", app.currentView())
	}
}

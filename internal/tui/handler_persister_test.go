package tui

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newTestAppModelWithMockPersisterAndProject creates an AppModel with mock persister and test project.
func newTestAppModelWithMockPersisterAndProject() (*AppModel, *MockPersister, *models.Project) {
	cfg := testutil.NewTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)
	mockPersister := NewMockPersister()

	// Store the project in mock for Load operations
	mockPersister.Projects[project.ID] = project

	app := NewAppModel(cfg, manager, WithPersister(mockPersister))
	app.currentProject = project

	return app, mockPersister, project
}

func TestWithPersisterOption(t *testing.T) {
	cfg := testutil.NewTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	mockPersister := NewMockPersister()
	app := NewAppModel(cfg, manager, WithPersister(mockPersister))

	if app.persister != mockPersister {
		t.Error("WithPersister option should set the persister")
	}
}

func TestDefaultPersisterIsProjectService(t *testing.T) {
	cfg := testutil.NewTestConfig()
	tmpDir, _ := os.MkdirTemp("", "tt-test-*")
	configPath := filepath.Join(tmpDir, "tt.json")
	manager := config.NewManager(configPath)

	app := NewAppModel(cfg, manager)

	if app.persister == nil {
		t.Error("Default persister should not be nil")
	}
	if _, ok := app.persister.(*ProjectService); !ok {
		t.Error("Default persister should be ProjectService")
	}
}

func TestHandleTaskCreatedMsgUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()
	app.pushView(ViewTaskCreator)

	task := &models.Task{ID: "test-task", Title: "Test Task"}
	msg := TaskCreatedMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskCreatedMsg should be handled")
	}

	// Verify persister.Load was called
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
}

func TestHandleTaskUpdatedMsgUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()
	app.pushView(ViewTaskEditor)

	msg := TaskUpdatedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskUpdatedMsg should be handled")
	}

	// Verify persister.Load was called
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
}

func TestHandleTaskPropertyChangedMsg(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()
	// Push project tree so we're not on dashboard (simulates in-place operation)
	app.pushView(ViewProjectTree)
	initialStackLen := len(app.viewStack)

	task := &models.Task{ID: "test-task", Title: "Test Task"}
	msg := TaskPropertyChangedMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskPropertyChangedMsg should be handled")
	}

	// Verify the view was NOT popped (key difference from TaskUpdatedMsg)
	if len(app.viewStack) != initialStackLen {
		t.Errorf("View stack should not change: expected %d, got %d", initialStackLen, len(app.viewStack))
	}

	// Verify persister.Load was NOT called (handleWithRefresh with reload=false)
	if len(mockPersister.LoadCalls) != 0 {
		t.Errorf("Expected 0 Load calls (no reload), got %d", len(mockPersister.LoadCalls))
	}
}

func TestHandleInlineTaskCreateUsesPersister(t *testing.T) {
	app, mockPersister, project := newTestAppModelWithMockPersisterAndProject()

	msg := InlineTaskCreateMsg{
		Project: project,
		Title:   "New Inline Task",
	}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("InlineTaskCreateMsg should be handled")
	}

	// Verify persister.Load was called to ensure project is loaded
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
}

func TestHandleInlineTaskCreateWithLoadError(t *testing.T) {
	app, mockPersister, project := newTestAppModelWithMockPersisterAndProject()
	mockPersister.LoadError = errors.New("load failed")

	msg := InlineTaskCreateMsg{
		Project: project,
		Title:   "New Inline Task",
	}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("InlineTaskCreateMsg should be handled")
	}

	// Should return an error command
	if cmd == nil {
		t.Error("Expected error command when load fails")
	}

	// Execute the command and check for error message
	result := cmd()
	if errMsg, ok := result.(ErrorMsg); !ok {
		t.Error("Expected ErrorMsg when load fails")
	} else if errMsg.Err == nil {
		t.Error("ErrorMsg should contain the error")
	}
}

func TestHandleInlineTaskCreateRejectsInvalidTitle(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{"html comment open", "Fix <!-- bug"},
		{"html comment close", "Fix --> issue"},
		{"heading prefix", "## Not a heading"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _, project := newTestAppModelWithMockPersisterAndProject()
			initialTaskCount := len(project.Tasks)

			msg := InlineTaskCreateMsg{
				Project: project,
				Title:   tt.title,
			}
			handled, cmd := app.handleTaskMessages(msg)

			if !handled {
				t.Error("InlineTaskCreateMsg should be handled")
			}
			if cmd == nil {
				t.Fatal("Expected error command for invalid title")
			}
			result := cmd()
			if _, ok := result.(ErrorMsg); !ok {
				t.Error("Expected ErrorMsg for invalid title")
			}
			// Task should not have been added
			if len(project.Tasks) != initialTaskCount {
				t.Errorf("Expected %d tasks (unchanged), got %d", initialTaskCount, len(project.Tasks))
			}
		})
	}
}

func TestSwitchToProjectUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()
	newProject := testutil.NewTestProjectWithTasks("New Project")
	mockPersister.Projects[newProject.ID] = newProject

	app.switchToProject(newProject, false)

	// Verify persister.Load was called
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
	if mockPersister.LoadCalls[0] != newProject {
		t.Error("Load should be called with the new project")
	}
}

func TestHandleMoveTaskToProjectUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()

	// Create source project with a task manually
	sourceProject := &models.Project{
		ID:   "source-project-id",
		Name: "Source Project",
	}
	task := &models.Task{
		ID:    "task-to-move",
		Title: "Task to Move",
		Type:  models.TaskTypeTask,
	}
	sourceProject.Tasks = []*models.Task{task}
	mockPersister.Projects[sourceProject.ID] = sourceProject
	app.config.Projects = append(app.config.Projects, sourceProject)
	app.currentProject = sourceProject

	// Create destination project
	destProject := &models.Project{
		ID:   "dest-project-id",
		Name: "Dest Project",
	}
	mockPersister.Projects[destProject.ID] = destProject
	app.config.Projects = append(app.config.Projects, destProject)

	msg := MoveTaskToProjectMsg{
		Task:          task,
		SourceProject: sourceProject,
		DestProjectID: destProject.ID,
	}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("MoveTaskToProjectMsg should be handled")
	}

	// Verify persister.Load was called for destination project
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}

	// Execute the returned batch command to trigger the async save
	_ = cmd // The batch contains startLoading and saveBothProjectsAsync
	// We can't easily test the async part without running the program
}

func TestCloseInboxProcessorMsgUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject()
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 5, SkippedCount: 2}
	handled, _ := app.handleInboxMessages(msg)

	if !handled {
		t.Error("CloseInboxProcessorMsg should be handled")
	}

	// Verify persister.Load was called
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
}

func TestCycleProjectUsesPersister(t *testing.T) {
	app, mockPersister, project := newTestAppModelWithMockPersisterAndProject()

	// Add project to cycle
	project2 := testutil.NewTestProjectWithTasks("Project 2")
	mockPersister.Projects[project2.ID] = project2
	app.config.Projects = append(app.config.Projects, project2)
	app.config.CycleProjects = []string{project.ID, project2.ID}

	// Cycle forward
	handled, _ := app.handleCycleProject(true)

	if !handled {
		t.Error("CycleProjectMsg should be handled")
	}

	// Verify persister.Load was called for the next project
	if len(mockPersister.LoadCalls) != 1 {
		t.Errorf("Expected 1 Load call, got %d", len(mockPersister.LoadCalls))
	}
}

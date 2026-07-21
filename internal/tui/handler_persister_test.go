package tui

import (
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// newTestAppModelWithMockPersisterAndProject creates an AppModel with mock persister and test project.
func newTestAppModelWithMockPersisterAndProject(t *testing.T) (*AppModel, *MockPersister, *models.Project) {
	t.Helper()
	cfg := newTestConfig()
	project := testutil.NewTestProjectWithTasks("Test Project")
	setTestProjects(cfg, []*models.Project{project})
	cfg.Machine().CurrentProject = project.ID

	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)
	mockPersister := NewMockPersister()

	// Store the project in mock for Load operations
	mockPersister.Projects[project.ID] = project

	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager, WithPersister(mockPersister))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}
	app.currentProject = project
	// NewAppModel hydrates the selected project because RuntimeState catalogs
	// contain metadata only. These tests measure the subsequent handler call.
	mockPersister.LoadCalls = nil

	return app, mockPersister, project
}

func TestWithPersisterOption(t *testing.T) {
	cfg := newTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)

	mockPersister := NewMockPersister()
	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager, WithPersister(mockPersister))
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	if app.persister != mockPersister {
		t.Error("WithPersister option should set the persister")
	}
}

func TestDefaultPersisterIsProjectService(t *testing.T) {
	cfg := newTestConfig()
	configPath := filepath.Join(t.TempDir(), "tt.json")
	manager := config.NewManager(configPath)

	app, err := NewAppModel(testRuntimeAt(cfg.DataDir(), cfg), manager)
	if err != nil {
		t.Fatalf("NewAppModel: %v", err)
	}

	if app.persister == nil {
		t.Error("Default persister should not be nil")
	}
	if _, ok := app.persister.(*ProjectService); !ok {
		t.Error("Default persister should be ProjectService")
	}
}

func TestHandleTaskCreatedMsgUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewTaskForm)

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
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewTaskForm)

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
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject(t)
	// Push project tree so we're not on dashboard (simulates in-place operation)
	app.pushView(ViewOutliner)
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

// The inline-create handler now routes through the application-layer capture use
// case (migrated in session 33.7.3.5.3) rather than the persister, so these tests
// drive the real transactional storage stack and assert committed on-disk state.
// The former persister.Load and manual-field-init assertions no longer apply:
// capture builds the task via models.NewTask and commits one ledger ChangeSet.

func TestHandleInlineTaskCreateCommitsToInbox(t *testing.T) {
	env := newAppBulkPickerEnv(t, []*models.Task{})
	app := env.appModel()
	project := env.projects[0]

	handled, cmd := app.handleTaskMessages(InlineTaskCreateMsg{
		Project: project,
		Title:   "New Inline Task",
	})
	if !handled {
		t.Error("InlineTaskCreateMsg should be handled")
	}
	if app.err != nil {
		t.Fatalf("unexpected error: %v", app.err)
	}
	_ = cmd

	reloaded := env.reload(t, 0)
	if len(reloaded.Tasks) != 1 {
		t.Fatalf("committed task count = %d, want 1", len(reloaded.Tasks))
	}
	got := reloaded.Tasks[0]
	if got.Title != "New Inline Task" {
		t.Errorf("committed title = %q, want %q", got.Title, "New Inline Task")
	}
	if models.SectionForState(got.State()) != models.SectionInbox {
		t.Errorf("committed section = %q, want %q", models.SectionForState(got.State()), models.SectionInbox)
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
			env := newAppBulkPickerEnv(t, []*models.Task{})
			app := env.appModel()
			project := env.projects[0]

			handled, cmd := app.handleTaskMessages(InlineTaskCreateMsg{
				Project: project,
				Title:   tt.title,
			})
			if !handled {
				t.Error("InlineTaskCreateMsg should be handled")
			}
			if cmd == nil {
				t.Fatal("Expected error command for invalid title")
			}
			if _, ok := cmd().(ErrorMsg); !ok {
				t.Error("Expected ErrorMsg for invalid title")
			}
			// No task should have been committed.
			if reloaded := env.reload(t, 0); len(reloaded.Tasks) != 0 {
				t.Errorf("committed task count = %d, want 0", len(reloaded.Tasks))
			}
		})
	}
}

func TestHandleInlineTaskCreateDefaultsPriority(t *testing.T) {
	tests := []struct {
		name         string
		priority     int
		wantPriority int
	}{
		{"zero defaults to PriorityDefault", 0, models.PriorityDefault},
		{"negative defaults to PriorityDefault", -1, models.PriorityDefault},
		{"above range defaults to PriorityDefault", 6, models.PriorityDefault},
		{"valid priority 1 preserved", 1, 1},
		{"valid priority 5 preserved", 5, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := newAppBulkPickerEnv(t, []*models.Task{})
			app := env.appModel()
			project := env.projects[0]

			app.handleTaskMessages(InlineTaskCreateMsg{
				Project:  project,
				Title:    "Priority test task",
				Priority: tt.priority,
			})

			reloaded := env.reload(t, 0)
			if len(reloaded.Tasks) == 0 {
				t.Fatal("Expected at least one committed task")
			}
			if got := reloaded.Tasks[0].Priority; got != tt.wantPriority {
				t.Errorf("Priority = %d, want %d", got, tt.wantPriority)
			}
		})
	}
}

func TestSwitchToProjectUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject(t)
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

// The cross-project move no longer routes through persister.Load /
// saveBothProjectsAsync: it commits one atomic app-layer transaction (session
// 33.7.3.5.4). Its behavior is covered by the disk-backed transactional tests in
// grouped_undo_transactions_test.go (TestTUITaskMove*).

func TestCloseInboxProcessorMsgUsesPersister(t *testing.T) {
	app, mockPersister, _ := newTestAppModelWithMockPersisterAndProject(t)
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

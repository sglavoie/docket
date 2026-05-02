package tui

import (
	"testing"
)

func TestTaskCommands_NewTask(t *testing.T) {
	app := newTestAppModel()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "new task")
	if cmd == nil {
		t.Fatal("expected to find 'new task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CreateTaskMsg); !ok {
		t.Errorf("expected CreateTaskMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Errorf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Errorf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture_DashboardNoProject(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.currentProject = nil // No active project
	app.viewStack = []ViewType{ViewDashboard}

	dash := NewDashboardModel(app.config, app.configManager)
	app.views.Register(ViewDashboard, dash)

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Fatalf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_BulkCapture(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "bulk capture")
	if cmd == nil {
		t.Fatal("expected to find 'bulk capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowBulkCaptureMsg); !ok {
		t.Errorf("expected ShowBulkCaptureMsg, got %T", result)
	}
}

func TestTaskCommands_BulkCapture_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "bulk capture")
	if cmd == nil {
		t.Fatal("expected to find 'bulk capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(StatusMsg); !ok {
		t.Errorf("expected StatusMsg, got %T", result)
	}
}

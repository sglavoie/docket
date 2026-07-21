package tui

import (
	"testing"
)

func TestTaskCommands_QuickCapture(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowQuickCaptureMsg); !ok {
		t.Errorf("expected ShowQuickCaptureMsg, got %T", result)
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
	if _, ok := result.(ShowQuickCaptureMsg); !ok {
		t.Errorf("expected ShowQuickCaptureMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture_DashboardNoProject(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.currentProject = nil // No active project
	app.viewStack = []PresentationID{ViewDashboard}

	dash := NewDashboardModel(app.runtime)
	app.views.Register(ViewDashboard, dash)

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowQuickCaptureMsg); !ok {
		t.Fatalf("expected ShowQuickCaptureMsg, got %T", result)
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

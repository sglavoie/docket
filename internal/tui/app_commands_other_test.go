package tui

import (
	"testing"
)

func TestOtherCommands_ProjectFocus(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus")
	if cmd == nil {
		t.Fatal("expected to find 'project focus' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowProjectFocusViewMsg); !ok {
		t.Errorf("expected ShowProjectFocusViewMsg, got %T", result)
	}
}

func TestOtherCommands_ProjectFocus_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "project focus")
	if cmd == nil {
		t.Fatal("expected to find 'project focus' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestOtherCommands_BlockersView(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", result)
	}
	if msg.Project == nil {
		t.Error("expected Project to be set")
	}
}

func TestOtherCommands_AllBlockers(t *testing.T) {
	app := newTestAppModel()

	cmds := app.otherCommands()
	cmd := findCommand(cmds, "all blockers")
	if cmd == nil {
		t.Fatal("expected to find 'all blockers' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", result)
	}
	if msg.Project != nil {
		t.Error("expected Project to be nil for global blockers")
	}
}

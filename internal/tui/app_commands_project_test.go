package tui

import (
	"testing"
)

func TestProjectCommands_NewProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "new project")
	if cmd == nil {
		t.Fatal("expected to find 'new project' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CreateProjectMsg); !ok {
		t.Errorf("expected CreateProjectMsg, got %T", result)
	}
}

func TestProjectCommands_RestoreFromTrash(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "restore from trash")
	if cmd == nil {
		t.Fatal("expected to find 'restore from trash' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowTrashedProjectsMsg); !ok {
		t.Errorf("expected ShowTrashedProjectsMsg, got %T", result)
	}
}

func TestProjectCommands_ResetSandbox(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "reset sandbox")
	if cmd == nil {
		t.Fatal("expected to find 'reset sandbox' command")
	}

	result := cmd.Handler()
	if _, ok := result.(SandboxResetRequestMsg); !ok {
		t.Errorf("expected SandboxResetRequestMsg, got %T", result)
	}
}

func TestProjectCommands_OpenSandbox_NoSandbox(t *testing.T) {
	app := newTestAppModel()
	// No sandbox project configured

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "open sandbox")
	if cmd == nil {
		t.Fatal("expected to find 'open sandbox' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil when no sandbox project, got %T", result)
	}
}

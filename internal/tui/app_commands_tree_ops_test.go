package tui

import (
	"testing"
)

func TestTaskCommands_ArchiveCompletedTasks(t *testing.T) {
	app := newTestAppModel()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "archive completed tasks")
	if cmd == nil {
		t.Fatal("expected to find 'archive completed tasks' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ArchiveCompletedTasksMsg); !ok {
		t.Errorf("expected ArchiveCompletedTasksMsg, got %T", result)
	}
}

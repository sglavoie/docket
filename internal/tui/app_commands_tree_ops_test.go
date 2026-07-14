package tui

import (
	"testing"
)

func TestTaskCommands_ExpandAll(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "expand all")
	if cmd == nil {
		t.Fatal("expected to find 'expand all' command")
	}

	// Should not panic
	cmd.Handler()
}

func TestTaskCommands_CollapseAll(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "collapse all")
	if cmd == nil {
		t.Fatal("expected to find 'collapse all' command")
	}

	// Should not panic
	cmd.Handler()
}

func TestTaskCommands_ExpandAll_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "expand all")
	if cmd == nil {
		t.Fatal("expected to find 'expand all' command")
	}

	// Should not panic
	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

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

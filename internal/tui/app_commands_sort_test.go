package tui

import (
	"testing"
)

func TestSortCommands_SortByPriority(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)

	cmds := app.sortCommands()
	cmd := findCommand(cmds, "sort by priority")
	if cmd == nil {
		t.Fatal("expected to find 'sort by priority' command")
	}

	// Should not panic
	cmd.Handler()
}

func TestSortCommands_ResetSort(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)

	cmds := app.sortCommands()
	cmd := findCommand(cmds, "reset sort")
	if cmd == nil {
		t.Fatal("expected to find 'reset sort' command")
	}

	// Should not panic
	cmd.Handler()
}

func TestSortCommands_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.sortCommands()
	cmd := findCommand(cmds, "sort by priority")
	if cmd == nil {
		t.Fatal("expected to find 'sort by priority' command")
	}

	// Should not panic
	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

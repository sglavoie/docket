package tui

import (
	"testing"
)

// advanceCursorToTask moves the outliner cursor to the first row that has a task (skipping section headers).
func advanceCursorToTask(o *OutlinerModel) {
	for i, row := range o.rows {
		if row.task != nil {
			o.cursor = i
			return
		}
	}
}

// findCommand searches a command slice by name and returns the matching command.
func findCommand(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}

func TestViewModeCommands_SwitchToActive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)
	app.projectTree.viewMode = ViewModeNext // start at something else

	cmds := app.viewModeCommands()
	cmd := findCommand(cmds, "active view")
	if cmd == nil {
		t.Fatal("expected to find 'active view' command")
	}

	cmd.Handler()

	if app.projectTree.viewMode != ViewModeActive {
		t.Errorf("expected ViewModeActive, got %d", app.projectTree.viewMode)
	}
}

func TestViewModeCommands_CycleViewMode(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, nil, app.runtime)
	app.projectTree.viewMode = ViewModeActive

	cmds := app.viewModeCommands()
	cmd := findCommand(cmds, "cycle view mode")
	if cmd == nil {
		t.Fatal("expected to find 'cycle view mode' command")
	}

	// Cycle and verify it advances
	cmd.Handler()
	first := app.projectTree.viewMode
	if first == ViewModeActive {
		t.Error("expected view mode to change after cycling")
	}

	// Cycle again
	cmd.Handler()
	second := app.projectTree.viewMode
	if second == first {
		t.Error("expected view mode to change after second cycle")
	}
}

func TestViewModeCommands_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.viewModeCommands()
	cmd := findCommand(cmds, "active view")
	if cmd == nil {
		t.Fatal("expected to find 'active view' command")
	}

	// Should not panic and return nil
	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

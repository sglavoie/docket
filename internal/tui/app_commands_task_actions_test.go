package tui

import (
	"slices"
	"testing"
)

func TestTaskActionCommands_ParkTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	expectedTask := app.outliner.getCursorTask()
	if expectedTask == nil {
		t.Fatal("expected outliner to have a task under the cursor")
	}
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner, ViewCommandPalette}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "park task")
	if cmd == nil {
		t.Fatal("expected to find 'park task' command")
	}
	if cmd.Shortcut != "tP" {
		t.Errorf("park task shortcut = %q, want tP", cmd.Shortcut)
	}
	for _, view := range []ViewType{ViewOutliner, ViewCrossProjectOutliner} {
		if !slices.Contains(cmd.Views, view) {
			t.Errorf("park task missing view %v in %v", view, cmd.Views)
		}
	}

	result := cmd.Handler()
	msg, ok := result.(ShowParkInputMsg)
	if !ok {
		t.Fatalf("expected ShowParkInputMsg, got %T", result)
	}
	if msg.Task != expectedTask || msg.Project != project {
		t.Fatalf("ShowParkInputMsg refs = (%p, %p), want (%p, %p)", msg.Task, msg.Project, expectedTask, project)
	}
}

func TestTaskActionCommands_ParkTask_NoTaskSelected(t *testing.T) {
	app := newTestAppModel()
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner, ViewCommandPalette}

	cmd := findCommand(app.taskActionCommands(), "park task")
	if cmd == nil {
		t.Fatal("expected to find 'park task' command")
	}

	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoTaskSelected {
		t.Errorf("status text: want %q, got %q", statusNoTaskSelected, string(status))
	}
}

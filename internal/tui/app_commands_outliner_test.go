package tui

import (
	"testing"
)

func TestOutlinerCommands_ViewTaskDetails(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows from test project")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "view task details")
	if cmd == nil {
		t.Fatal("expected to find 'view task details' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowTaskDetailMsg); !ok {
		t.Errorf("expected ShowTaskDetailMsg, got %T", result)
	}
}

func TestOutlinerCommands_NilCursorTask(t *testing.T) {
	app := newTestAppModel()
	app.outliner = NewOutlinerModel(nil, app.config, app.configManager)

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "view task details")
	if cmd == nil {
		t.Fatal("expected to find 'view task details' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestOutlinerCommands_EditTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "edit task")
	if cmd == nil {
		t.Fatal("expected to find 'edit task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(EditTaskMsg); !ok {
		t.Errorf("expected EditTaskMsg, got %T", result)
	}
}

func TestOutlinerCommands_CopyTitle(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "copy title")
	if cmd == nil {
		t.Fatal("expected to find 'copy title' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CopyTaskTitleMsg); !ok {
		t.Errorf("expected CopyTaskTitleMsg, got %T", result)
	}
}

func TestOutlinerCommands_CopyDetails(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "copy details")
	if cmd == nil {
		t.Fatal("expected to find 'copy details' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CopyTaskDetailsMsg); !ok {
		t.Errorf("expected CopyTaskDetailsMsg, got %T", result)
	}
}

func TestOutlinerCommands_CreateSubtask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "create subtask")
	if cmd == nil {
		t.Fatal("expected to find 'create subtask' command")
	}

	result := cmd.Handler()
	msg, ok := result.(CreateLinkedTaskMsg)
	if !ok {
		t.Fatalf("expected CreateLinkedTaskMsg, got %T", result)
	}
	if msg.LinkType != LinkTypeSubtask {
		t.Errorf("expected LinkTypeSubtask, got %d", msg.LinkType)
	}
}

func TestOutlinerCommands_CreateSibling(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "create sibling task")
	if cmd == nil {
		t.Fatal("expected to find 'create sibling task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CreateTaskMsg); !ok {
		t.Errorf("expected CreateTaskMsg, got %T", result)
	}
}

func TestOutlinerCommands_DeleteTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}

	// Should show confirmation dialog, not panic
	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil (confirmation shown on outliner), got %T", result)
	}
}

func TestOutlinerCommands_ScoreEditor(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "score editor")
	if cmd == nil {
		t.Fatal("expected to find 'score editor' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowScoreEditorMsg); !ok {
		t.Errorf("expected ShowScoreEditorMsg, got %T", result)
	}
}

func TestOutlinerCommands_ProjectTree(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "project tree")
	if cmd == nil {
		t.Fatal("expected to find 'project tree' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowProjectTreeMsg); !ok {
		t.Errorf("expected ShowProjectTreeMsg, got %T", result)
	}
}

func TestOutlinerCommands_FocusBoard(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "today")
	if cmd == nil {
		t.Fatal("expected to find 'today' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowFocusBoardMsg); !ok {
		t.Errorf("expected ShowFocusBoardMsg, got %T", result)
	}
}

func TestOutlinerCommands_MultiSelectMode(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "multi-select mode")
	if cmd == nil {
		t.Fatal("expected to find 'multi-select mode' command")
	}

	// Should toggle multi-select mode without panicking
	_ = cmd.Handler()

	// Verify multi-select was toggled
	if !app.outliner.multiSelectMode {
		t.Error("expected multiSelectMode to be true after toggle")
	}
}

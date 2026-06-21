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

func TestOutlinerCommands_NewTaskNoTemplate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "new task (no template)")
	if cmd == nil {
		t.Fatal("expected to find 'new task (no template)' command")
	}
	if cmd.Shortcut != "tn" {
		t.Errorf("Shortcut = %q, want \"tn\"", cmd.Shortcut)
	}

	result := cmd.Handler()
	msg, ok := result.(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", result)
	}
	if msg.ParentTask == nil {
		t.Fatal("ParentTask should be set from cursor task")
	}
	if msg.Project != project {
		t.Errorf("Project = %v, want %v", msg.Project, project)
	}
	if !msg.SkipTemplatePicker {
		t.Error("SkipTemplatePicker should be true")
	}
}

func TestOutlinerCommands_NewTaskWithTemplate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "new task (with template picker)")
	if cmd == nil {
		t.Fatal("expected to find 'new task (with template picker)' command")
	}
	if cmd.Shortcut != "tN" {
		t.Errorf("Shortcut = %q, want \"tN\"", cmd.Shortcut)
	}

	result := cmd.Handler()
	msg, ok := result.(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", result)
	}
	if msg.ParentTask == nil {
		t.Fatal("ParentTask should be set from cursor task")
	}
	if msg.Project != project {
		t.Errorf("Project = %v, want %v", msg.Project, project)
	}
	if msg.SkipTemplatePicker {
		t.Error("SkipTemplatePicker should be false")
	}
}

func TestOutlinerCommands_CreateLinkedSubtask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}

	cmds := app.outlinerCommands()
	cmd := findCommand(cmds, "create linked subtask")
	if cmd == nil {
		t.Fatal("expected to find 'create linked subtask' command")
	}
	if cmd.Shortcut != "cs" {
		t.Errorf("Shortcut = %q, want \"cs\"", cmd.Shortcut)
	}

	result := cmd.Handler()
	msg, ok := result.(CreateLinkedTaskMsg)
	if !ok {
		t.Fatalf("expected CreateLinkedTaskMsg, got %T", result)
	}
	if msg.LinkType != LinkTypeSubtask {
		t.Errorf("expected LinkTypeSubtask, got %d", msg.LinkType)
	}
	if msg.Project != project {
		t.Errorf("Project = %v, want %v", msg.Project, project)
	}
	if msg.SourceTask == nil {
		t.Error("SourceTask should be set from cursor task")
	}
}

// TestOutlinerCommands_NewTaskUsesZoomedParent verifies that when the
// single-project outliner has a zoom stack, palette tn/tN use the zoomed
// (focused) task as the parent rather than the cursor row task.
func TestOutlinerCommands_NewTaskUsesZoomedParent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	if len(project.Tasks) < 2 {
		t.Fatal("expected test project with at least two tasks")
	}
	app.outliner = NewOutlinerModel(project, app.config, app.configManager)
	app.outliner.zoomStack = []ZoomEntry{{TaskID: project.Tasks[0].ID}}
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	cmd := findCommand(app.outlinerCommands(), "new task (no template)")
	if cmd == nil {
		t.Fatal("expected 'new task (no template)' command")
	}
	msg, ok := cmd.Handler().(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", cmd.Handler())
	}
	if msg.ParentTask != project.Tasks[0] {
		t.Errorf("ParentTask = %v, want zoomed task %v", msg.ParentTask, project.Tasks[0])
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

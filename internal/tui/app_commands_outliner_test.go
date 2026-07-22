package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestOutlinerCommands_ViewTaskDetails(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(nil, app.configManager, app.runtime)

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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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

	// Palette delete now emits the canonical, shell-owned TaskDeleteRequestedMsg
	// for the cursor task instead of showing an Outliner-local confirm dialog.
	result := cmd.Handler()
	msg, ok := result.(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("expected TaskDeleteRequestedMsg, got %T", result)
	}
	if msg.Origin != ViewOutliner {
		t.Errorf("Origin = %v, want ViewOutliner", msg.Origin)
	}
	if len(msg.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(msg.Refs))
	}
	if msg.Refs[0].Task != app.outliner.rows[app.outliner.cursor].task {
		t.Errorf("ref task = %v, want cursor task", msg.Refs[0].Task)
	}
	if msg.Refs[0].Project != project {
		t.Errorf("ref project = %v, want %v", msg.Refs[0].Project, project)
	}
}

// TestOutlinerUppercaseXEmitsDeleteRequest verifies the keyboard uppercase-X
// binding emits the canonical, shell-owned deletion request for the cursor task
// (single-project mode) without any pre-commit mutation.
func TestOutlinerUppercaseXEmitsDeleteRequest(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows")
	}
	cursorTask := app.outliner.rows[app.outliner.cursor].task

	_, cmd := app.outliner.handleKeyMsg(syntheticKey('X'))
	if cmd == nil {
		t.Fatal("expected command from uppercase-X keypress")
	}
	msg, ok := cmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("expected TaskDeleteRequestedMsg, got %T", cmd())
	}
	if msg.Origin != ViewOutliner {
		t.Errorf("Origin = %v, want ViewOutliner", msg.Origin)
	}
	if len(msg.Refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(msg.Refs))
	}
	if msg.Refs[0].Task != cursorTask {
		t.Errorf("ref task = %v, want cursor task", msg.Refs[0].Task)
	}
	if msg.Refs[0].Project != project {
		t.Errorf("ref project = %v, want %v", msg.Refs[0].Project, project)
	}
}

// TestOutlinerUppercaseXMultiSelectEmitsDeleteRequestForSelection verifies the
// keyboard uppercase-X binding deletes the whole multi-selection (not just the
// cursor) through the canonical request path.
func TestOutlinerUppercaseXMultiSelectEmitsDeleteRequestForSelection(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	app.outliner.toggleMultiSelectMode()
	var selected []*models.Task
	for _, row := range app.outliner.rows {
		if row.isTaskRow() && row.task != nil {
			app.outliner.ToggleSelection(row.task.ID)
			selected = append(selected, row.task)
		}
	}
	if len(selected) == 0 {
		t.Fatal("expected at least one selectable task")
	}

	_, cmd := app.outliner.handleKeyMsg(syntheticKey('X'))
	if cmd == nil {
		t.Fatal("expected command from uppercase-X keypress")
	}
	msg, ok := cmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("expected TaskDeleteRequestedMsg, got %T", cmd())
	}
	if msg.Origin != ViewOutliner {
		t.Errorf("Origin = %v, want ViewOutliner", msg.Origin)
	}
	if len(msg.Refs) != len(selected) {
		t.Fatalf("expected %d refs (whole selection), got %d", len(selected), len(msg.Refs))
	}
	// No pre-commit mutation: the live project still holds every task.
	for _, task := range selected {
		if project.GetTaskByID(task.ID) == nil {
			t.Errorf("task %s was removed before commit", task.ID)
		}
	}
}

// TestOutlinerPaletteDeleteMatchesKeyboardX verifies keyboard and palette
// deletion emit equivalent canonical requests for the same cursor target.
func TestOutlinerPaletteDeleteMatchesKeyboardX(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	_, keyCmd := app.outliner.handleKeyMsg(syntheticKey('X'))
	if keyCmd == nil {
		t.Fatal("expected command from uppercase-X keypress")
	}
	keyMsg, ok := keyCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("keyboard: expected TaskDeleteRequestedMsg, got %T", keyCmd())
	}

	cmd := findCommand(app.outlinerCommands(), "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}
	palMsg, ok := cmd.Handler().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("palette: expected TaskDeleteRequestedMsg, got %T", cmd.Handler())
	}

	if keyMsg.Origin != palMsg.Origin {
		t.Errorf("origin mismatch: keyboard %v, palette %v", keyMsg.Origin, palMsg.Origin)
	}
	if len(keyMsg.Refs) != 1 || len(palMsg.Refs) != 1 {
		t.Fatalf("expected 1 ref each, got keyboard %d palette %d", len(keyMsg.Refs), len(palMsg.Refs))
	}
	if keyMsg.Refs[0].Task != palMsg.Refs[0].Task || keyMsg.Refs[0].Project != palMsg.Refs[0].Project {
		t.Errorf("target mismatch: keyboard %v/%v, palette %v/%v",
			keyMsg.Refs[0].Task, keyMsg.Refs[0].Project, palMsg.Refs[0].Task, palMsg.Refs[0].Project)
	}
	_ = project
}

func TestOutlinerCommands_ScoreEditor(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
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

func TestOutlinerCommands_ProjectViewNavigationLivesInViewCommands(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	if cmd := findCommand(app.outlinerCommands(), "project tree"); cmd != nil {
		t.Fatalf("outlinerCommands should not contain project-view navigation: %+v", cmd)
	}
	if cmd := findCommand(app.outlinerCommands(), "toggle project tree/outliner"); cmd != nil {
		t.Fatalf("outlinerCommands should not contain project-view navigation: %+v", cmd)
	}
}

func TestOutlinerCommands_MultiSelectMode(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)

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

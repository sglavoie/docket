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
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
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
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
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

func TestNotesCommands_ShowNotes(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNotesMsg); !ok {
		t.Errorf("expected ShowNotesMsg, got %T", result)
	}
}

func TestNotesCommands_ShowNotes_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestNotesCommands_CreateNote(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "create note")
	if cmd == nil {
		t.Fatal("expected to find 'create note' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNoteCreatorMsg); !ok {
		t.Errorf("expected ShowNoteCreatorMsg, got %T", result)
	}
}

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

func TestSortCommands_SortByPriority(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

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
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

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

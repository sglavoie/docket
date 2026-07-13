package tui

import (
	"slices"
	"testing"
)

func TestTaskActionCommands_EditTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected project tree to have visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "edit task")
	if cmd == nil {
		t.Fatal("expected to find 'edit task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(EditTaskMsg); !ok {
		t.Errorf("expected EditTaskMsg, got %T", result)
	}
}

func TestTaskActionCommands_EditTask_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "edit task")
	if cmd == nil {
		t.Fatal("expected to find 'edit task' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTaskActionCommands_EditTask_MultiSelect(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	// Enable multi-select and select tasks
	app.projectTree.multiSelectMode = true
	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}
	task := visible[0].Task
	app.projectTree.selectedTasks[task.ID] = true

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "edit task")
	if cmd == nil {
		t.Fatal("expected to find 'edit task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowBulkEditMsg); !ok {
		t.Errorf("expected ShowBulkEditMsg, got %T", result)
	}
}

func TestTaskActionCommands_DeleteTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}

	// Should show delete confirmation, not panic
	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil (confirmation shown on projectTree), got %T", result)
	}
}

func TestTaskActionCommands_DeleteTask_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTaskActionCommands_ParkTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected project tree to have visible nodes")
	}
	app.projectTree.cursor = 0
	expectedTask := visible[0].Task
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "park task")
	if cmd == nil {
		t.Fatal("expected to find 'park task' command")
	}
	if cmd.Shortcut != "tP" {
		t.Errorf("park task shortcut = %q, want tP", cmd.Shortcut)
	}
	for _, view := range []ViewType{ViewProjectTree, ViewOutliner, ViewCrossProjectOutliner} {
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
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

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

func TestTaskActionCommands_CopyTitle(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "copy title")
	if cmd == nil {
		t.Fatal("expected to find 'copy title' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CopyTaskTitleMsg); !ok {
		t.Errorf("expected CopyTaskTitleMsg, got %T", result)
	}
}

func TestTaskActionCommands_CopyTitle_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "copy title")
	if cmd == nil {
		t.Fatal("expected to find 'copy title' command")
	}

	result := cmd.Handler()
	if _, ok := result.(StatusMsg); !ok {
		t.Errorf("expected StatusMsg, got %T", result)
	}
}

func TestTaskActionCommands_MoveToProject(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "move to project")
	if cmd == nil {
		t.Fatal("expected to find 'move to project' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowProjectSelectorForMoveMsg); !ok {
		t.Errorf("expected ShowProjectSelectorForMoveMsg, got %T", result)
	}
}

func TestTaskActionCommands_MoveToProject_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil
	app.currentProject = nil

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "move to project")
	if cmd == nil {
		t.Fatal("expected to find 'move to project' command")
	}

	result := cmd.Handler()
	if _, ok := result.(StatusMsg); !ok {
		t.Errorf("expected StatusMsg, got %T", result)
	}
}

func TestTaskActionCommands_TogglePin(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "toggle pin")
	if cmd == nil {
		t.Fatal("expected to find 'toggle pin' command")
	}

	result := cmd.Handler()
	msg, ok := result.(TaskPropertyChangedMsg)
	if !ok {
		t.Errorf("expected TaskPropertyChangedMsg, got %T", result)
	}
	if !msg.NeedsSave {
		t.Error("toggle pin command should set NeedsSave=true")
	}
	if msg.UndoAction != "Toggle Pin" {
		t.Errorf("UndoAction = %q, want Toggle Pin", msg.UndoAction)
	}
	if len(msg.UndoTaskIDs) != 1 || msg.UndoTaskIDs[0] != msg.Task.ID {
		t.Errorf("UndoTaskIDs = %v, want [%q]", msg.UndoTaskIDs, msg.Task.ID)
	}
}

func TestTaskActionCommands_SetDueDate(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "set due date")
	if cmd == nil {
		t.Fatal("expected to find 'set due date' command")
	}

	result := cmd.Handler()
	if _, ok := result.(SetDueDateMsg); !ok {
		t.Errorf("expected SetDueDateMsg, got %T", result)
	}
}

func TestTaskActionCommands_MoveToSectionShortcutAndHandler(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "move to section")
	if cmd == nil {
		t.Fatal("expected to find 'move to section' command")
	}
	if cmd.Shortcut != "ts" {
		t.Fatalf("move to section shortcut = %q, want ts", cmd.Shortcut)
	}

	result := cmd.Handler()
	msg, ok := result.(ShowSectionPickerMsg)
	if !ok {
		t.Fatalf("expected ShowSectionPickerMsg, got %T", result)
	}
	if msg.Project != project {
		t.Fatal("ShowSectionPickerMsg should target the current project")
	}
	if msg.Task != visible[0].Task {
		t.Fatal("ShowSectionPickerMsg should target the selected task")
	}
}

func TestTaskActionCommands_FilterOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "filter overlay")
	if cmd == nil {
		t.Fatal("expected to find 'filter overlay' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowFilterMsg)
	if !ok {
		t.Fatalf("expected ShowFilterMsg, got %T", result)
	}
	if msg.ProjectTree == nil {
		t.Error("expected ProjectTree to be set in ShowFilterMsg")
	}
}

func TestTaskActionCommands_Undo_NilProjectTree(t *testing.T) {
	app := newTestAppModel()
	app.projectTree = nil

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "undo")
	if cmd == nil {
		t.Fatal("expected to find 'undo' command")
	}

	result := cmd.Handler()
	if _, ok := result.(StatusMsg); !ok {
		t.Errorf("expected StatusMsg, got %T", result)
	}
}

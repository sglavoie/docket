package tui

import (
	"testing"
)

func TestTaskCommands_NewTask(t *testing.T) {
	app := newTestAppModel()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "new task")
	if cmd == nil {
		t.Fatal("expected to find 'new task' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CreateTaskMsg); !ok {
		t.Errorf("expected CreateTaskMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Errorf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Errorf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_QuickCapture_DashboardNoProject(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.currentProject = nil // No active project
	app.viewStack = []ViewType{ViewDashboard}

	dash := NewDashboardModel(app.config, app.configManager)
	app.views.Register(ViewDashboard, dash)

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Fatalf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestTaskCommands_BulkCapture(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "bulk capture")
	if cmd == nil {
		t.Fatal("expected to find 'bulk capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowBulkCaptureMsg); !ok {
		t.Errorf("expected ShowBulkCaptureMsg, got %T", result)
	}
}

func TestTaskCommands_BulkCapture_NilProject(t *testing.T) {
	app := newTestAppModel()
	app.currentProject = nil

	cmds := app.taskCommands()
	cmd := findCommand(cmds, "bulk capture")
	if cmd == nil {
		t.Fatal("expected to find 'bulk capture' command")
	}

	result := cmd.Handler()
	if _, ok := result.(StatusMsg); !ok {
		t.Errorf("expected StatusMsg, got %T", result)
	}
}

func TestTaskCommands_ExpandAll(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

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
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

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

func TestProjectCommands_NewProject(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "new project")
	if cmd == nil {
		t.Fatal("expected to find 'new project' command")
	}

	result := cmd.Handler()
	if _, ok := result.(CreateProjectMsg); !ok {
		t.Errorf("expected CreateProjectMsg, got %T", result)
	}
}

func TestProjectCommands_ToggleInfoPanel(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "toggle info panel")
	if cmd == nil {
		t.Fatal("expected to find 'toggle info panel' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ToggleInfoPanelMsg); !ok {
		t.Errorf("expected ToggleInfoPanelMsg, got %T", result)
	}
}

func TestProjectCommands_RestoreFromTrash(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "restore from trash")
	if cmd == nil {
		t.Fatal("expected to find 'restore from trash' command")
	}

	result := cmd.Handler()
	if _, ok := result.(ShowTrashedProjectsMsg); !ok {
		t.Errorf("expected ShowTrashedProjectsMsg, got %T", result)
	}
}

func TestProjectCommands_ResetSandbox(t *testing.T) {
	app := newTestAppModel()

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "reset sandbox")
	if cmd == nil {
		t.Fatal("expected to find 'reset sandbox' command")
	}

	result := cmd.Handler()
	if _, ok := result.(SandboxResetRequestMsg); !ok {
		t.Errorf("expected SandboxResetRequestMsg, got %T", result)
	}
}

func TestProjectCommands_OpenSandbox_NoSandbox(t *testing.T) {
	app := newTestAppModel()
	// No sandbox project configured

	cmds := app.projectCommands()
	cmd := findCommand(cmds, "open sandbox")
	if cmd == nil {
		t.Fatal("expected to find 'open sandbox' command")
	}

	result := cmd.Handler()
	if result != nil {
		t.Errorf("expected nil when no sandbox project, got %T", result)
	}
}

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
	if result != nil {
		t.Errorf("expected nil, got %v", result)
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
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestTaskActionCommands_ToggleActive(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)

	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected visible nodes")
	}

	cmds := app.taskActionCommands()
	cmd := findCommand(cmds, "toggle active")
	if cmd == nil {
		t.Fatal("expected to find 'toggle active' command")
	}

	result := cmd.Handler()
	msg, ok := result.(TaskPropertyChangedMsg)
	if !ok {
		t.Errorf("expected TaskPropertyChangedMsg, got %T", result)
	}
	if !msg.NeedsSave {
		t.Error("toggle active command should set NeedsSave=true")
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

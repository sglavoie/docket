package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// setupCrossPaletteApp wires both a single-project outliner and a cross-project
// outliner into the AppModel, then stacks the views so that the non-palette
// origin is ViewCrossProjectOutliner. Returns the cursor task and project for
// assertion convenience.
func setupCrossPaletteApp(t *testing.T) (app *AppModel, singleProj *models.Project, crossProj *models.Project, crossTask *models.Task) {
	t.Helper()
	app, singleProj = newTestAppModelWithProject()

	app.outliner = NewOutlinerModel(singleProj, app.config, app.configManager)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	crossProj = &models.Project{ID: "cross-proj", Name: "Cross", Path: t.TempDir()}
	crossTask, _ = models.NewTask("cross-task", 1, models.TaskTypeTask, "Cross Task")
	crossProj.Tasks = []*models.Task{crossTask}
	crossProj.BuildTaskTree()

	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.loadedProjects = []*models.Project{crossProj}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProj.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)
	app.viewStack = append(app.viewStack, ViewCommandPalette)
	return app, singleProj, crossProj, crossTask
}

func TestOutlinerCommands_Cross_ViewTaskDetails(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "view task details")
	if cmd == nil {
		t.Fatal("expected to find 'view task details' command")
	}
	msg, ok := cmd.Handler().(ShowTaskDetailMsg)
	if !ok {
		t.Fatalf("expected ShowTaskDetailMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestOutlinerCommands_Cross_EditTask(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "edit task")
	if cmd == nil {
		t.Fatal("expected to find 'edit task' command")
	}
	msg, ok := cmd.Handler().(EditTaskMsg)
	if !ok {
		t.Fatalf("expected EditTaskMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestOutlinerCommands_Cross_RenameSetsQuickEditProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "rename task")
	if cmd == nil {
		t.Fatal("expected to find 'rename task' command")
	}
	_ = cmd.Handler()
	if app.crossOutliner.crossQuickEditProject != crossProj {
		t.Errorf("crossQuickEditProject: want cross project, got %v", app.crossOutliner.crossQuickEditProject)
	}
}

func TestOutlinerCommands_Cross_CreateLinkedSubtask(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "create linked subtask")
	if cmd == nil {
		t.Fatal("expected to find 'create linked subtask' command")
	}
	msg, ok := cmd.Handler().(CreateLinkedTaskMsg)
	if !ok {
		t.Fatalf("expected CreateLinkedTaskMsg, got %T", cmd.Handler())
	}
	if msg.SourceTask != crossTask {
		t.Errorf("SourceTask: want cross task, got %v", msg.SourceTask)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
	if msg.LinkType != LinkTypeSubtask {
		t.Errorf("LinkType: want subtask, got %d", msg.LinkType)
	}
}

func TestOutlinerCommands_Cross_NewTaskNoTemplate(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "new task (no template)")
	if cmd == nil {
		t.Fatal("expected to find 'new task (no template)' command")
	}
	msg, ok := cmd.Handler().(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", cmd.Handler())
	}
	if msg.ParentTask != crossTask {
		t.Errorf("ParentTask: want cross task, got %v", msg.ParentTask)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
	if !msg.SkipTemplatePicker {
		t.Error("SkipTemplatePicker should be true for tn")
	}
}

func TestOutlinerCommands_Cross_NewTaskWithTemplate(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "new task (with template picker)")
	if cmd == nil {
		t.Fatal("expected to find 'new task (with template picker)' command")
	}
	msg, ok := cmd.Handler().(CreateTaskMsg)
	if !ok {
		t.Fatalf("expected CreateTaskMsg, got %T", cmd.Handler())
	}
	if msg.ParentTask != crossTask {
		t.Errorf("ParentTask: want cross task, got %v", msg.ParentTask)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
	if msg.SkipTemplatePicker {
		t.Error("SkipTemplatePicker should be false for tN")
	}
}

func TestOutlinerCommands_Cross_DeleteSetsTargetOnCrossOutliner(t *testing.T) {
	app, _, _, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}
	if result := cmd.Handler(); result != nil {
		t.Errorf("expected nil (confirmation shown), got %T", result)
	}
	if app.crossOutliner.deleteTarget != crossTask {
		t.Errorf("crossOutliner.deleteTarget: want cross task, got %v", app.crossOutliner.deleteTarget)
	}
	if app.outliner.deleteTarget != nil {
		t.Errorf("single outliner.deleteTarget should not have been touched, got %v", app.outliner.deleteTarget)
	}
}

func TestOutlinerCommands_Cross_ScoreEditor(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "score editor")
	if cmd == nil {
		t.Fatal("expected to find 'score editor' command")
	}
	msg, ok := cmd.Handler().(ShowScoreEditorMsg)
	if !ok {
		t.Fatalf("expected ShowScoreEditorMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestOutlinerCommands_Cross_ProjectTreeUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "project tree")
	if cmd == nil {
		t.Fatal("expected to find 'project tree' command")
	}
	msg, ok := cmd.Handler().(ShowProjectTreeMsg)
	if !ok {
		t.Fatalf("expected ShowProjectTreeMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestOutlinerCommands_Cross_MultiSelectTogglesCrossOutliner(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "multi-select mode")
	if cmd == nil {
		t.Fatal("expected to find 'multi-select mode' command")
	}
	_ = cmd.Handler()
	if !app.crossOutliner.multiSelectMode {
		t.Error("expected crossOutliner multiSelectMode to be true after toggle")
	}
	if app.outliner.multiSelectMode {
		t.Error("expected single outliner multiSelectMode to remain false")
	}
}

func TestOutlinerCommands_Cross_MoveToProjectSetsSource(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "move to project")
	if cmd == nil {
		t.Fatal("expected to find 'move to project' command")
	}
	msg, ok := cmd.Handler().(ShowProjectSelectorForMoveMsg)
	if !ok {
		t.Fatalf("expected ShowProjectSelectorForMoveMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.SourceProject != crossProj {
		t.Errorf("SourceProject: want cross project, got %v", msg.SourceProject)
	}
}

func TestOutlinerCommands_Cross_EditContent(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "edit content")
	if cmd == nil {
		t.Fatal("expected to find 'edit content' command")
	}
	msg, ok := cmd.Handler().(EditTaskContentMsg)
	if !ok {
		t.Fatalf("expected EditTaskContentMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestOutlinerCommands_Cross_SortByPrioritySortsCrossOutliner(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.outlinerCommands(), "outliner sort by priority")
	if cmd == nil {
		t.Fatal("expected to find 'outliner sort by priority' command")
	}
	if cmd.Shortcut != "1" {
		t.Errorf("Shortcut: want \"1\", got %q", cmd.Shortcut)
	}
	_ = cmd.Handler()
	if app.crossOutliner.sort.Field != SortByPriority {
		t.Errorf("crossOutliner sort field: want SortByPriority, got %v", app.crossOutliner.sort.Field)
	}
}

func TestOutlinerCommands_Cross_ResetSort(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)

	// Seed a non-default sort first
	app.crossOutliner.sort.Field = SortByScore
	cmd := findCommand(app.outlinerCommands(), "outliner reset sort")
	if cmd == nil {
		t.Fatal("expected to find 'outliner reset sort' command")
	}
	if cmd.Shortcut != "0" {
		t.Errorf("Shortcut: want \"0\", got %q", cmd.Shortcut)
	}
	_ = cmd.Handler()
	if app.crossOutliner.sort.Field == SortByScore {
		t.Errorf("crossOutliner sort should have been reset, still %v", app.crossOutliner.sort.Field)
	}
}

func TestOutlinerCommands_DeleteShortcutIsTD(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmd := findCommand(app.outlinerCommands(), "delete task")
	if cmd == nil {
		t.Fatal("expected to find 'delete task' command")
	}
	if cmd.Shortcut != "tD" {
		t.Errorf("Shortcut: want \"tD\", got %q", cmd.Shortcut)
	}
}

func TestOutlinerCommands_DimUndimShortcutIsTH(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	cmd := findCommand(app.outlinerCommands(), "dim/undim task")
	if cmd == nil {
		t.Fatal("expected to find 'dim/undim task' command")
	}
	if cmd.Shortcut != "th" {
		t.Errorf("Shortcut: want \"th\", got %q", cmd.Shortcut)
	}
}

func TestOutlinerCommands_AllStatusCommandsAdvertiseCrossProjectScope(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	statusCommands := []string{
		"start task", "complete task", "block task", "standby task",
		"cancel task", "reopen task", "review task", "maybe task",
	}
	cmds := app.outlinerCommands()
	for _, name := range statusCommands {
		cmd := findCommand(cmds, name)
		if cmd == nil {
			t.Errorf("missing command %q", name)
			continue
		}
		if !containsView(cmd.Views, ViewCrossProjectOutliner) {
			t.Errorf("command %q: missing ViewCrossProjectOutliner in Views=%v", name, cmd.Views)
		}
		if !containsView(cmd.Views, ViewOutliner) {
			t.Errorf("command %q: missing ViewOutliner in Views=%v", name, cmd.Views)
		}
	}
}

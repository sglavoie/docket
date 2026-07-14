package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestStructureCommands_Reparent_DispatchesToCrossOutliner verifies the
// palette "move/reparent task" handler uses m.crossOutliner (not m.outliner)
// when invoked from ViewCrossProjectOutliner, even if a single-project
// outliner also exists. Otherwise the OpenParentSelectorMsg would describe
// the wrong task and Source.
func TestStructureCommands_Reparent_DispatchesToCrossOutliner(t *testing.T) {
	app, singleProj := newTestAppModelWithProject()

	// Create the single-project outliner (would have been picked up by the
	// old "first non-nil" handler).
	app.outliner = NewOutlinerModel(singleProj, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	// Now open a cross-project outliner with a distinct project.
	crossProj := &models.Project{ID: "cross-proj", Name: "Cross", Path: t.TempDir()}
	crossTask, _ := models.NewTask("cross-task", 1, models.TaskTypeTask, "Cross Task")
	crossProj.Tasks = []*models.Task{crossTask}
	crossProj.BuildTaskTree()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{crossProj}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProj.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)
	// Simulate the palette overlay being on top.
	app.viewStack = append(app.viewStack, ViewCommandPalette)

	cmds := app.structureCommands()
	cmd := findCommand(cmds, "move/reparent task")
	if cmd == nil {
		t.Fatal("expected to find 'move/reparent task' command")
	}

	result := cmd.Handler()
	msg, ok := result.(OpenParentSelectorMsg)
	if !ok {
		t.Fatalf("expected OpenParentSelectorMsg, got %T", result)
	}
	if msg.Source != ViewCrossProjectOutliner {
		t.Errorf("Source: want ViewCrossProjectOutliner, got %v", msg.Source)
	}
	if msg.ReparentTaskID != crossTask.ID {
		t.Errorf("ReparentTaskID: want %q, got %q", crossTask.ID, msg.ReparentTaskID)
	}
}

// TestStructureCommands_QuickNote_DispatchesToCrossOutliner verifies the
// palette "quick note" handler resolves the project from the cross outliner's
// row (via getTaskRef) rather than calling getProject() — which returns nil
// in cross-project mode and would silently drop the action.
func TestStructureCommands_QuickNote_DispatchesToCrossOutliner(t *testing.T) {
	app, singleProj := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(singleProj, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	crossProj := &models.Project{ID: "cross-proj", Name: "Cross", Path: t.TempDir()}
	crossTask, _ := models.NewTask("cross-task", 1, models.TaskTypeTask, "Cross Task")
	crossProj.Tasks = []*models.Task{crossTask}
	crossProj.BuildTaskTree()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{crossProj}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProj.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)
	app.viewStack = append(app.viewStack, ViewCommandPalette)

	cmds := app.structureCommands()
	cmd := findCommand(cmds, "quick note")
	if cmd == nil {
		t.Fatal("expected to find 'quick note' command")
	}

	result := cmd.Handler()
	msg, ok := result.(OpenQuickNoteMsg)
	if !ok {
		t.Fatalf("expected OpenQuickNoteMsg, got %T (nil project would short-circuit)", result)
	}
	if msg.Task == nil || msg.Task.ID != crossTask.ID {
		t.Errorf("Task mismatch: want %q", crossTask.ID)
	}
	if msg.Project == nil || msg.Project.ID != crossProj.ID {
		t.Errorf("Project mismatch: want %q", crossProj.ID)
	}
}

// TestStructureCommands_Reparent_DispatchesToOutliner verifies the palette
// handler still uses the single-project outliner when invoked from
// ViewOutliner.
func TestStructureCommands_Reparent_DispatchesToOutliner(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(project, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)

	if len(app.outliner.rows) == 0 {
		t.Fatal("expected outliner to have rows from test project")
	}
	cursorTask := app.outliner.rows[app.outliner.cursor].task

	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)
	// Palette overlay on top.
	app.viewStack = append(app.viewStack, ViewCommandPalette)

	cmds := app.structureCommands()
	cmd := findCommand(cmds, "move/reparent task")
	if cmd == nil {
		t.Fatal("expected to find 'move/reparent task' command")
	}

	result := cmd.Handler()
	msg, ok := result.(OpenParentSelectorMsg)
	if !ok {
		t.Fatalf("expected OpenParentSelectorMsg, got %T", result)
	}
	if msg.Source != ViewOutliner {
		t.Errorf("Source: want ViewOutliner, got %v", msg.Source)
	}
	if msg.ReparentTaskID != cursorTask.ID {
		t.Errorf("ReparentTaskID: want %q, got %q", cursorTask.ID, msg.ReparentTaskID)
	}
}

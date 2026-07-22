package tui

import (
	"bytes"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Session 15.1 (FR-017) parity: the Task Workspace and Planner palette adapters
// expose Peek, Undo, and Redo, but AppModel intercepts the direct Shift+Enter /
// u / Ctrl+R keys before either view dispatcher runs. Before the repair the
// palette commands delegated to the view dispatcher, which opened full task
// detail for Peek and no-op'd Undo/Redo. These end-to-end tests execute all
// three registered palette commands on both surfaces and assert their effect
// matches the direct key: Peek activates the same in-app overlay and preserves
// the palette; Undo/Redo restore the same on-disk tasks.md the keyboard does.

// --- Task Workspace ------------------------------------------------------------

func TestTaskWorkspacePaletteShellActionsMatchDirectKeys(t *testing.T) {
	t.Run("peek", func(t *testing.T) {
		app, _, _ := newWorkspaceParityApp(t)
		assertPalettePeekParity(t, app, app.taskWorkspaceCommands(), "Task Workspace Peek at task")
	})

	t.Run("undo-redo", func(t *testing.T) {
		app, project, task := newWorkspaceParityApp(t)
		// The live controller refreshes its own snapshot after each restore, so
		// the workspace cursor survives without re-seeding.
		assertPaletteUndoRedoParity(t, app, project, task,
			"Task Workspace Undo task change", "Task Workspace Redo task change",
			func() []Command { return app.taskWorkspaceCommands() }, func() {})
	})
}

// --- Planner -------------------------------------------------------------------

func TestPlannerPaletteShellActionsMatchDirectKeys(t *testing.T) {
	t.Run("peek", func(t *testing.T) {
		app, _, _ := newPlannerParityApp(t)
		assertPalettePeekParity(t, app, app.focusBoardCommands(), "Planner Peek at task")
	})

	t.Run("undo-redo", func(t *testing.T) {
		app, project, task := newPlannerParityApp(t)
		planner := app.views.Get(ViewFocusBoard).(*FocusBoardModel)
		// The minimal test Planner reloads from an empty runtime, so each restore
		// wipes its hand-planted rows; re-plant the cursor before every step so the
		// adapter and direct keys keep resolving the same project.
		reseed := func() { plantPlannerCursor(planner, project, task) }
		assertPaletteUndoRedoParity(t, app, project, task,
			"Planner Undo task change", "Planner Redo task change",
			func() []Command { return app.focusBoardCommands() }, reseed)
	})
}

// --- Shared parity assertions --------------------------------------------------

// assertPalettePeekParity drives the direct Shift+Enter key and the registered
// palette Peek command over the same cursor task, then asserts they activate the
// same in-app Peek overlay. The palette path additionally proves the palette is
// preserved (not discarded into full task detail as the pre-repair dispatcher
// did): after routing the command through the execute flow the palette is still
// the current view and Peek is active.
func assertPalettePeekParity(t *testing.T, app *AppModel, cmds []Command, peekName string) {
	t.Helper()

	// Direct Shift+Enter activates the in-app Peek overlay for the cursor task.
	handled, cmd := app.handleGlobalKeyPress(tea.KeyPressMsg{Code: tea.KeyEnter, Mod: tea.ModShift})
	if !handled || cmd != nil {
		t.Fatalf("direct Shift+Enter handled=%v cmd!=nil=%v, want peek activation", handled, cmd != nil)
	}
	if !app.peekActive || app.peekTask == nil {
		t.Fatalf("direct Shift+Enter did not activate Peek: active=%v task=%v", app.peekActive, app.peekTask)
	}
	directTaskID := app.peekTask.ID
	app.dismissPeek()

	// The palette Peek command emits a PalettePeekRequestMsg for the same task —
	// the shell-owned overlay message, not the dispatcher's full-detail message.
	peek := findParityCommand(t, cmds, peekName)
	res := peek.Handler()
	req, ok := res.(PalettePeekRequestMsg)
	if !ok {
		t.Fatalf("palette Peek returned %T, want PalettePeekRequestMsg (in-app overlay, not full detail)", res)
	}
	if req.Task == nil || req.Task.ID != directTaskID {
		t.Fatalf("palette Peek task = %v, want direct-key target %q", req.Task, directTaskID)
	}

	// Route the command result through the palette execute flow: Peek must keep
	// the palette on the stack so Esc restores its exact search/cursor state.
	app.pushView(ViewCommandPalette)
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("failed to open palette for parity check: view=%v", app.currentView())
	}
	handled, _ = app.handleSearchMessages(CommandPaletteExecuteMsg{ResultMsg: res})
	if !handled {
		t.Fatal("CommandPaletteExecuteMsg carrying the palette Peek was not handled")
	}
	if app.currentView() != ViewCommandPalette {
		t.Fatalf("palette Peek discarded the palette (view=%v); it must preserve palette state", app.currentView())
	}
	if !app.peekActive || app.peekTask == nil || app.peekTask.ID != directTaskID {
		t.Fatalf("palette Peek did not activate the same overlay: active=%v task=%v want=%q", app.peekActive, app.peekTask, directTaskID)
	}
}

// assertPaletteUndoRedoParity seeds one ledger-backed mutation, then exercises
// all four (palette|direct) × (undo|redo) combinations against the same on-disk
// tasks.md. Palette undo (step 1) and direct undo (step 3) must both restore the
// pre-mutation bytes; direct redo (step 2) and palette redo (step 4) must both
// replay the post-mutation bytes. Each palette command must return a routed
// PaletteCommandResultMsg — proving it invokes the shell restore rather than the
// pre-repair no-op.
func assertPaletteUndoRedoParity(t *testing.T, app *AppModel, project *models.Project, task *models.Task, undoName, redoName string, commands func() []Command, reseed func()) {
	t.Helper()

	before := readTasksFile(t, project)
	if _, err := app.appService.SetPriority(task.ID, 1); err != nil {
		t.Fatalf("seed ledger mutation: %v", err)
	}
	after := readTasksFile(t, project)
	if bytes.Equal(before, after) {
		t.Fatal("seed mutation did not change tasks.md; cannot compare undo/redo")
	}
	// Keep the live workspace snapshot consistent with the committed mutation so
	// the palette adapter still resolves the cursor task after the change.
	if app.taskWorkspace != nil {
		if err := app.taskWorkspace.RefreshIfLoaded(); err != nil {
			t.Fatalf("refresh workspace after mutation: %v", err)
		}
	}

	cmds := commands()
	undo := findParityCommand(t, cmds, undoName)
	redo := findParityCommand(t, cmds, redoName)

	// Step 1 — palette undo restores the pre-mutation state.
	reseed()
	if res := undo.Handler(); !isRoutedPaletteResult(res) {
		t.Fatalf("palette undo returned %T, want a routed PaletteCommandResultMsg (not a no-op)", res)
	}
	if got := readTasksFile(t, project); !bytes.Equal(got, before) {
		t.Fatalf("palette undo did not restore pre-mutation tasks.md\n got: %q\nwant: %q", got, before)
	}

	// Step 2 — direct Ctrl+R replays the mutation.
	reseed()
	app = sendAppKey(app, syntheticCtrlKey('r'))
	if got := readTasksFile(t, project); !bytes.Equal(got, after) {
		t.Fatalf("direct Ctrl+R did not replay the mutation\n got: %q\nwant: %q", got, after)
	}

	// Step 3 — direct u restores the pre-mutation state.
	reseed()
	app = sendAppKey(app, syntheticKey('u'))
	if got := readTasksFile(t, project); !bytes.Equal(got, before) {
		t.Fatalf("direct u did not restore pre-mutation tasks.md\n got: %q\nwant: %q", got, before)
	}

	// Step 4 — palette redo replays the mutation, matching direct Ctrl+R.
	reseed()
	if res := redo.Handler(); !isRoutedPaletteResult(res) {
		t.Fatalf("palette redo returned %T, want a routed PaletteCommandResultMsg (not a no-op)", res)
	}
	if got := readTasksFile(t, project); !bytes.Equal(got, after) {
		t.Fatalf("palette redo did not replay the mutation\n got: %q\nwant: %q", got, after)
	}
}

// plantPlannerCursor re-installs a single-row Today cursor over the current
// on-disk task pointer. The minimal test Planner has no live data source, so
// each ledger restore clears its rows; re-planting keeps getRowAtCursor pointing
// at the parity project between steps.
func plantPlannerCursor(planner *FocusBoardModel, project *models.Project, task *models.Task) {
	target := project.GetTaskByID(task.ID)
	if target == nil {
		target = task
	}
	row := boardRow{Task: target, Project: project, Priority: target.Priority, Source: project.Name}
	planner.tabs.Active = boardTabToday
	planner.todayRows = []boardRow{row}
	planner.nextRows = []boardRow{row}
	planner.backlogRows = []boardRow{row}
	planner.allRows = []boardRow{row}
	planner.applyFilter()
	planner.cursor = 0
}

func isRoutedPaletteResult(msg tea.Msg) bool {
	result, ok := msg.(PaletteCommandResultMsg)
	return ok && result.Cmd != nil
}

func findParityCommand(t *testing.T, cmds []Command, name string) Command {
	t.Helper()
	for _, command := range cmds {
		if command.Name == name {
			return command
		}
	}
	t.Fatalf("palette catalog missing command %q", name)
	return Command{}
}

// --- Live-surface fixtures over the ledger-backed transaction stack ------------

// newWorkspaceParityApp builds a ledger-backed AppModel whose live, active Task
// Workspace is scoped to one project with the cursor on task. Direct u/Ctrl+R
// resolve the workspace via activeTaskWorkspace(); the palette adapter resolves
// it via taskWorkspacePrimary() — both must reach the same shell restore.
func newWorkspaceParityApp(t *testing.T) (*AppModel, *models.Project, *models.Task) {
	t.Helper()
	dataDir := t.TempDir()
	task := seedTask(t, 1, "Workspace parity target", models.StateReady)
	project := newLedgerUndoProject(t, dataDir, "workspace-parity", task)
	app := newLedgerUndoApp(t, dataDir, project, ViewOutliner, task.ID, true)

	controller, err := NewTaskWorkspaceController(
		TaskWorkspaceParams{Scope: ProjectEntityScope(project.ID), Layout: WorkspaceOutline, Focus: WorkspaceAll},
		NewTaskWorkspaceQuery(staticTaskWorkspaceSource{data: TaskWorkspaceData{Projects: []*models.Project{project}}}),
		NewFeatureTaskWorkspaceMutationAdapter(TaskItemCoordinator{}, BulkCoordinator{}),
	)
	if err != nil {
		t.Fatalf("build workspace controller: %v", err)
	}
	view, err := NewTaskWorkspaceView(controller)
	if err != nil {
		t.Fatalf("build workspace view: %v", err)
	}
	view.resolveProject = app.projectByID
	app.taskWorkspace = controller
	app.taskWorkspaceView = view
	app.enableTypedNavigation()
	if err := controller.Refresh(); err != nil {
		t.Fatalf("refresh workspace: %v", err)
	}
	if !controller.State.SetCursor(TaskIdentity{ProjectID: project.ID, TaskID: task.ID}) {
		t.Fatal("failed to place workspace cursor on the parity task")
	}
	if !app.activeTaskWorkspace() {
		t.Fatal("task workspace did not activate after wiring")
	}
	if _, _, ok := app.taskWorkspacePrimary(); !ok {
		t.Fatal("taskWorkspacePrimary did not resolve the live workspace")
	}
	return app, project, task
}

// newPlannerParityApp builds a ledger-backed AppModel whose current view is a
// live Planner (FocusBoardModel) with the cursor on task. Direct u/Ctrl+R and
// the palette adapter both resolve the Planner through ViewFocusBoard.
func newPlannerParityApp(t *testing.T) (*AppModel, *models.Project, *models.Task) {
	t.Helper()
	dataDir := t.TempDir()
	task := seedTask(t, 1, "Planner parity target", models.StateReady)
	project := newLedgerUndoProject(t, dataDir, "planner-parity", task)
	app := newLedgerUndoApp(t, dataDir, project, ViewFocusBoard, task.ID, true)

	planner := newTestFocusBoard()
	plantPlannerCursor(planner, project, task)
	planner.rebuildPlannerDispatcher()
	app.views.Register(ViewFocusBoard, planner)

	if app.currentView() != ViewFocusBoard {
		t.Fatalf("planner is not the current view: %v", app.currentView())
	}
	if resolved, ok := app.views.Get(ViewFocusBoard).(*FocusBoardModel); !ok || resolved == nil {
		t.Fatal("planner did not register at ViewFocusBoard")
	}
	if resolvedTask, _ := planner.getTaskRef(); resolvedTask == nil {
		t.Fatal("planner cursor did not resolve the parity task")
	}
	return app, project, task
}

package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// Cross-project palette parity for read/view/capture commands. Each test
// confirms that the named palette command, when invoked from the
// ViewCrossProjectOutliner origin, routes to the cursor row's project rather
// than m.currentProject. setupCrossPaletteApp parks the non-palette origin at
// ViewCrossProjectOutliner with a cursor on a known task/project, mirroring the
// existing Session 2 palette tests in app_commands_outliner_cross_project_test.go.

func TestCrossPalette_BlockersViewUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.otherCommands(), "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}
	msg, ok := cmd.Handler().(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_BlockersView_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.otherCommands(), "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_InboxProcessorUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.viewCommands(), "inbox processor")
	if cmd == nil {
		t.Fatal("expected to find 'inbox processor' command")
	}
	msg, ok := cmd.Handler().(ShowInboxProcessorMsg)
	if !ok {
		t.Fatalf("expected ShowInboxProcessorMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_CreateNoteUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.notesCommands(), "create note")
	if cmd == nil {
		t.Fatal("expected to find 'create note' command")
	}
	msg, ok := cmd.Handler().(ShowNoteCreatorMsg)
	if !ok {
		t.Fatalf("expected ShowNoteCreatorMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_CreateNote_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSectionHeader: true, SectionName: "Inbox"}}

	cmd := findCommand(app.notesCommands(), "create note")
	if cmd == nil {
		t.Fatal("expected to find 'create note' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_QuickCaptureUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.taskCommands(), "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}
	msg, ok := cmd.Handler().(ShowQuickCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowQuickCaptureMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_QuickCapture_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.taskCommands(), "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_QuickCaptureFromProjectTreeFallsBackToGlobalPicker(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmd := findCommand(app.taskCommands(), "quick capture")
	if cmd == nil {
		t.Fatal("expected to find 'quick capture' command")
	}
	result := cmd.Handler()
	if _, ok := result.(ShowGlobalProjectPickerMsg); !ok {
		t.Fatalf("expected ShowGlobalProjectPickerMsg, got %T", result)
	}
}

func TestCrossPalette_BulkCaptureUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.taskCommands(), "bulk capture")
	if cmd == nil {
		t.Fatal("expected to find 'bulk capture' command")
	}
	msg, ok := cmd.Handler().(ShowBulkCaptureMsg)
	if !ok {
		t.Fatalf("expected ShowBulkCaptureMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_EditLastTaskUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)
	// Seed a recent task on the cross project (existing crossTask becomes the
	// most recent task because it is the only task).
	cmd := findCommand(app.taskCommands(), "edit last task")
	if cmd == nil {
		t.Fatal("expected to find 'edit last task' command")
	}
	result := cmd.Handler()
	msg, ok := result.(EditTaskMsg)
	if !ok {
		t.Fatalf("expected EditTaskMsg, got %T", result)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
	if msg.Task == nil || msg.Task.Title != "Cross Task" {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
}

func TestCrossPalette_EditLastTask_NoTasks(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)
	crossProj.Tasks = nil

	cmd := findCommand(app.taskCommands(), "edit last task")
	if cmd == nil {
		t.Fatal("expected to find 'edit last task' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != "No recent tasks" {
		t.Errorf("status text: want \"No recent tasks\", got %q", string(status))
	}
}

func TestCrossPalette_TaskActivityUsesCursorTaskAndProject(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	cmd := findCommand(app.taskActionCommands(), "task activity")
	if cmd == nil {
		t.Fatal("expected to find 'task activity' command")
	}
	msg, ok := cmd.Handler().(OpenTaskActivityMsg)
	if !ok {
		t.Fatalf("expected OpenTaskActivityMsg, got %T", cmd.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_TaskActivity_NoTaskUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.taskActionCommands(), "task activity")
	if cmd == nil {
		t.Fatal("expected to find 'task activity' command")
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

func TestCrossPalette_ProjectRecentActivityUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.taskActionCommands(), "project recent activity")
	if cmd == nil {
		t.Fatal("expected to find 'project recent activity' command")
	}
	msg, ok := cmd.Handler().(ShowProjectRecentActivityMsg)
	if !ok {
		t.Fatalf("expected ShowProjectRecentActivityMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_ProjectRecentActivity_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.taskActionCommands(), "project recent activity")
	if cmd == nil {
		t.Fatal("expected to find 'project recent activity' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

// Registry-level availability: every read/view/capture palette command updated
// in Session 3 must be reachable through GetForView(ViewCrossProjectOutliner)
// so the command palette renders them when the user opens it from the
// cross-project outliner. Mirrors the existing TestViewCommands_PriorityView_CrossOutlinerScope
// pattern but covers the broader read/view/capture set.
func TestCrossPalette_RegistryExposesReadViewCaptureCommandsInCrossOutliner(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := registerCommandGroupsForTest(
		t,
		app.notesCommands(),
		app.otherCommands(),
		app.viewCommands(),
		app.taskCommands(),
		app.taskActionCommands(),
		app.outlinerCommands(),
	)

	cases := []struct {
		name     string
		shortcut string
	}{
		{"blockers view", "b"},
		{"inbox processor", "W"},
		{"create note", "}"},
		{"bulk capture", "I"},
		{"edit last task", "ctrl+l"},
		{"task activity", "gh"},
		{"project recent activity", "gl"},
		{"project tree", "ctrl+p"},
	}

	for _, tc := range cases {
		cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), tc.name)
		if cmd == nil {
			t.Errorf("%q should be reachable from ViewCrossProjectOutliner", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("%q shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
	}

	// `quick capture` is a global command (empty Views slice). GetForView falls
	// back to the global registration when no view-specific entry exists; the
	// command must still be reachable from the cross-project outliner so users
	// can press `i` from there.
	if cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), "quick capture"); cmd == nil {
		t.Error("'quick capture' should be reachable from ViewCrossProjectOutliner")
	}
}

// TestCrossPalette_BlockersView_FallsBackToCurrentProject confirms that the
// updated `blockers view` handler keeps its non-cross-outliner behavior intact:
// when invoked from the project tree or single outliner the palette continues
// to target m.currentProject.
func TestCrossPalette_BlockersView_FallsBackToCurrentProject(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmd := findCommand(app.otherCommands(), "blockers view")
	if cmd == nil {
		t.Fatal("expected to find 'blockers view' command")
	}
	msg, ok := cmd.Handler().(ShowBlockersMsg)
	if !ok {
		t.Fatalf("expected ShowBlockersMsg, got %T", cmd.Handler())
	}
	if msg.Project != project {
		t.Errorf("Project: want current project, got %v", msg.Project)
	}
}

// TestCrossPalette_TaskActivity_FromProjectTreeUsesSelectedTask exercises the
// non-cross branch of paletteTaskTarget so the project-tree origin keeps the
// `selectedTreeTask()` + currentProject behavior intact.
func TestCrossPalette_TaskActivity_FromProjectTreeUsesSelectedTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	visible := app.projectTree.visibleNodes()
	if len(visible) == 0 {
		t.Fatal("expected project tree to have visible nodes")
	}
	app.projectTree.cursor = 0
	expectedTask := visible[0].Task
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmd := findCommand(app.taskActionCommands(), "task activity")
	if cmd == nil {
		t.Fatal("expected to find 'task activity' command")
	}
	msg, ok := cmd.Handler().(OpenTaskActivityMsg)
	if !ok {
		t.Fatalf("expected OpenTaskActivityMsg, got %T", cmd.Handler())
	}
	if msg.Task != expectedTask {
		t.Errorf("Task: want %v, got %v", expectedTask, msg.Task)
	}
	if msg.Project != project {
		t.Errorf("Project: want %v, got %v", project, msg.Project)
	}
}

// Sanity check: paletteCursorProject returns the cursor project on cross
// outliner origin and falls back to currentProject otherwise. This catches
// regressions where a future refactor swaps the helper without noticing the
// origin branch.
func TestPaletteCursorProject_CrossOutlinerVsCurrent(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	if got := app.paletteCursorProject(); got != crossProj {
		t.Errorf("cross-outliner palette project: want %v, got %v", crossProj, got)
	}

	// Pop the palette and cross-outliner frames so the non-palette origin is
	// ViewOutliner (single-project). The helper must fall back to
	// m.currentProject in that case.
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner, ViewCommandPalette}
	if got := app.paletteCursorProject(); got != app.currentProject {
		t.Errorf("single-outliner palette project: want %v, got %v", app.currentProject, got)
	}
}

// Defensive: ensure setupCrossPaletteApp delivered a cursor task with a
// non-nil project so other assertions cannot silently pass on a nil-row
// scenario. Mirrors the assertion style used by the existing palette test
// helpers in this file.
func TestSetupCrossPaletteApp_PrimesCursorTask(t *testing.T) {
	_, _, crossProj, crossTask := setupCrossPaletteApp(t)
	if crossTask == nil {
		t.Fatal("cross task should be primed")
	}
	if crossProj == nil {
		t.Fatal("cross project should be primed")
	}
}

// Cross-project palette parity for mutation and project-operation commands.
// Direct-key parity for these handlers is covered by
// outliner_keys_mutation_project_op_parity_test.go; these tests assert the
// palette-routed counterparts behave the same.

func TestCrossPalette_ArchiveCompletedTasksUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.taskCommands(), "archive completed tasks")
	if cmd == nil {
		t.Fatal("expected to find 'archive completed tasks' command")
	}
	msg, ok := cmd.Handler().(ArchiveCompletedTasksMsg)
	if !ok {
		t.Fatalf("expected ArchiveCompletedTasksMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

func TestCrossPalette_ArchiveCompletedTasks_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.taskCommands(), "archive completed tasks")
	if cmd == nil {
		t.Fatal("expected to find 'archive completed tasks' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_ArchiveCompletedTasks_FallsBackToCurrentProject(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmd := findCommand(app.taskCommands(), "archive completed tasks")
	if cmd == nil {
		t.Fatal("expected to find 'archive completed tasks' command")
	}
	msg, ok := cmd.Handler().(ArchiveCompletedTasksMsg)
	if !ok {
		t.Fatalf("expected ArchiveCompletedTasksMsg, got %T", cmd.Handler())
	}
	if msg.Project != project {
		t.Errorf("Project: want current project, got %v", msg.Project)
	}
}

func TestCrossPalette_ArchiveProjectUsesCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)

	cmd := findCommand(app.otherCommands(), "archive project")
	if cmd == nil {
		t.Fatal("expected to find 'archive project' command")
	}
	msg, ok := cmd.Handler().(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("expected ProjectArchiveRequestMsg, got %T", cmd.Handler())
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
	if !msg.Archive {
		t.Errorf("Archive: want true (project starts unarchived), got false")
	}
}

func TestCrossPalette_ArchiveProject_UnarchivesArchivedCursorProject(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)
	crossProj.Archived = true

	cmd := findCommand(app.otherCommands(), "archive project")
	if cmd == nil {
		t.Fatal("expected to find 'archive project' command")
	}
	msg, ok := cmd.Handler().(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("expected ProjectArchiveRequestMsg, got %T", cmd.Handler())
	}
	if msg.Archive {
		t.Errorf("Archive: want false (project starts archived), got true")
	}
}

func TestCrossPalette_ArchiveProject_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSeparator: true}}

	cmd := findCommand(app.otherCommands(), "archive project")
	if cmd == nil {
		t.Fatal("expected to find 'archive project' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_ArchiveProject_SandboxEmitsError(t *testing.T) {
	app, _, crossProj, _ := setupCrossPaletteApp(t)
	crossProj.Sandbox = true

	cmd := findCommand(app.otherCommands(), "archive project")
	if cmd == nil {
		t.Fatal("expected to find 'archive project' command")
	}
	if _, ok := cmd.Handler().(ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for sandbox archive, got %T", cmd.Handler())
	}
}

func TestCrossPalette_PasteClipboard_NoProjectUnderCursor(t *testing.T) {
	app, _, _, _ := setupCrossPaletteApp(t)
	app.crossOutliner.rows = []outlinerRow{{IsSectionHeader: true, SectionName: "Inbox"}}

	cmd := findCommand(app.otherCommands(), "paste clipboard")
	if cmd == nil {
		t.Fatal("expected to find 'paste clipboard' command")
	}
	result := cmd.Handler()
	status, ok := result.(StatusMsg)
	if !ok {
		t.Fatalf("expected StatusMsg, got %T", result)
	}
	if string(status) != statusNoProjectUnderCursor {
		t.Errorf("status text: want %q, got %q", statusNoProjectUnderCursor, string(status))
	}
}

func TestCrossPalette_MoveToProjectUsesCursorTaskAndProject(t *testing.T) {
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

func TestCrossPalette_QuickNoteUsesCursorTaskAndProject(t *testing.T) {
	app, _, crossProj, crossTask := setupCrossPaletteApp(t)

	quickNote := app.quickNoteCommand()
	msg, ok := quickNote.Handler().(OpenQuickNoteMsg)
	if !ok {
		t.Fatalf("expected OpenQuickNoteMsg, got %T", quickNote.Handler())
	}
	if msg.Task != crossTask {
		t.Errorf("Task: want cross task, got %v", msg.Task)
	}
	if msg.Project != crossProj {
		t.Errorf("Project: want cross project, got %v", msg.Project)
	}
}

// Registry-level availability: every mutation / project-operation palette
// command updated in Session 4 must be reachable through
// GetForView(ViewCrossProjectOutliner).
func TestCrossPalette_RegistryExposesMutationProjectOpCommandsInCrossOutliner(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	registry := registerCommandGroupsForTest(
		t,
		app.taskCommands(),
		app.otherCommands(),
		app.outlinerCommands(),
		[]Command{app.quickNoteCommand()},
	)

	cases := []struct {
		name     string
		shortcut string
	}{
		{"archive completed tasks", ""},
		{"archive project", "ctrl+a"},
		{"paste clipboard", "ctrl+v"},
		{"move to project", "tm"},
		{"quick note", "ctrl+."},
	}

	for _, tc := range cases {
		cmd := findCommand(registry.GetForView(ViewCrossProjectOutliner), tc.name)
		if cmd == nil {
			t.Errorf("%q should be reachable from ViewCrossProjectOutliner", tc.name)
			continue
		}
		if cmd.Shortcut != tc.shortcut {
			t.Errorf("%q shortcut = %q, want %q", tc.name, cmd.Shortcut, tc.shortcut)
		}
	}
}

// TestCrossPalette_ArchiveProject_FallsBackToCurrentProject confirms that the
// updated `archive project` handler keeps its non-cross-outliner behavior
// intact: when invoked from the project tree or single outliner the palette
// continues to target m.currentProject.
func TestCrossPalette_ArchiveProject_FallsBackToCurrentProject(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

	cmd := findCommand(app.otherCommands(), "archive project")
	if cmd == nil {
		t.Fatal("expected to find 'archive project' command")
	}
	msg, ok := cmd.Handler().(ProjectArchiveRequestMsg)
	if !ok {
		t.Fatalf("expected ProjectArchiveRequestMsg, got %T", cmd.Handler())
	}
	if msg.Project != project {
		t.Errorf("Project: want current project, got %v", msg.Project)
	}
}

// Used to silence the linter that would flag the imported models package as
// unused if every test above happened to not reference it. Future tests
// instantiating ad-hoc tasks/projects rely on this package import.
var _ = models.TaskTypeTask

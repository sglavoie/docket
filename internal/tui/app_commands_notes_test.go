package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNotesCommands_ShowNotesIsPaletteOnly(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}
	if cmd.Shortcut != "" {
		t.Fatalf("show notes shortcut = %q, want palette-only", cmd.Shortcut)
	}
	expectedViews := []PresentationID{
		ViewOutliner,
		ViewCrossProjectOutliner,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewRecentActivity,
		ViewProjectRecentActivity,
		ViewInboxProcessor,
		ViewGlobalInboxProcessor,
		ViewTaskDetail,
	}
	if len(cmd.Views) != len(expectedViews) {
		t.Fatalf("show notes views = %d, want %d: %v", len(cmd.Views), len(expectedViews), cmd.Views)
	}
	for _, view := range expectedViews {
		if !slices.Contains(cmd.Views, view) {
			t.Fatalf("show notes command missing view %s", view)
		}
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNotesMsg); !ok {
		t.Errorf("expected ShowNotesMsg, got %T", result)
	}
}

func TestNotesCommands_ShowNotes_UsesSelectedTaskProjectFromCrossProjectOrigin(t *testing.T) {
	app, singleProject := newTestAppModelWithProject()
	app.currentProject = singleProject

	crossProject := &models.Project{ID: "cross-project", Name: "Cross Project", Path: t.TempDir()}
	crossTask, err := models.NewTask("cross-task", 1, models.TaskTypeTask, "Cross Task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	crossProject.Tasks = []*models.Task{crossTask}
	crossProject.BuildTaskTree()

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{crossProject}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProject.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCrossProjectOutliner, ViewCommandPalette}

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowNotesMsg)
	if !ok {
		t.Fatalf("expected ShowNotesMsg, got %T", result)
	}
	if msg.Project != crossProject {
		t.Fatalf("ShowNotesMsg should target selected task project %q, got %v", crossProject.ID, msg.Project)
	}
}

func TestNotesCommands_ShowNotes_FallsBackToCurrentProject(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCommandPalette}

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}

	result := cmd.Handler()
	msg, ok := result.(ShowNotesMsg)
	if !ok {
		t.Fatalf("expected ShowNotesMsg, got %T", result)
	}
	if msg.Project != project {
		t.Fatal("ShowNotesMsg should fall back to current project")
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

// TestCrossOutlinerNotesUseVisibleProjectContext locks in FR17-003: on the
// canonical cross-project Outliner, both the direct `{` binding and the "show
// notes" palette command must resolve the visible cursor project or active
// project scope — never the stale AppModel.currentProject — and a missing
// project must surface an explicit status instead of a silent no-op.
func TestCrossOutlinerNotesUseVisibleProjectContext(t *testing.T) {
	newCrossTask := func(t *testing.T, title string) *models.Task {
		t.Helper()
		task, err := models.NewTask(title, 1, models.TaskTypeTask, title)
		if err != nil {
			t.Fatalf("NewTask: %v", err)
		}
		return task
	}

	t.Run("cursor project differs from currentProject", func(t *testing.T) {
		app, stale := newTestAppModelWithProject()
		app.currentProject = stale // stale single-project selection

		cursorProject := &models.Project{ID: "cursor-project", Name: "Cursor Project", Path: t.TempDir()}
		cursorTask := newCrossTask(t, "Cursor Task")
		cursorProject.Tasks = []*models.Task{cursorTask}
		cursorProject.BuildTaskTree()

		app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
		app.crossOutliner.loadedProjects = []*models.Project{stale, cursorProject}
		app.crossOutliner.rows = []outlinerRow{{task: cursorTask, ProjectID: cursorProject.ID}}
		app.crossOutliner.cursor = 0
		app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
		app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCrossProjectOutliner, ViewCommandPalette}

		// Direct `{` binding and the palette share the canonical resolver.
		if got := app.crossOutliner.contextProject(); got != cursorProject {
			t.Fatalf("contextProject = %v, want cursor project %q", got, cursorProject.ID)
		}

		// Palette command resolves the same cursor project.
		cmd := findCommand(app.notesCommands(), "show notes")
		if cmd == nil {
			t.Fatal("expected to find 'show notes' command")
		}
		msg, ok := cmd.Handler().(ShowNotesMsg)
		if !ok {
			t.Fatalf("expected ShowNotesMsg, got %T", cmd.Handler())
		}
		if msg.Project != cursorProject {
			t.Fatalf("show notes targeted %v, want cursor project %q (not stale currentProject)", msg.Project, cursorProject.ID)
		}
	})

	t.Run("empty scoped project resolves to the active scope", func(t *testing.T) {
		app, stale := newTestAppModelWithProject()
		app.currentProject = stale // stale single-project selection

		scoped := &models.Project{ID: "scoped-project", Name: "Scoped Project", Path: t.TempDir()}

		app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
		app.crossOutliner.loadedProjects = []*models.Project{stale, scoped}
		app.crossOutliner.scopedProjectID = scoped.ID
		// Cursor rests on a non-task section header, so resolution must use the
		// active scope rather than currentProject.
		app.crossOutliner.rows = []outlinerRow{{IsSectionHeader: true, SectionName: "Inbox"}}
		app.crossOutliner.cursor = 0
		app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
		app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCrossProjectOutliner, ViewCommandPalette}

		if got := app.crossOutliner.contextProject(); got != scoped {
			t.Fatalf("contextProject = %v, want scoped project %q", got, scoped.ID)
		}

		cmd := findCommand(app.notesCommands(), "show notes")
		if cmd == nil {
			t.Fatal("expected to find 'show notes' command")
		}
		msg, ok := cmd.Handler().(ShowNotesMsg)
		if !ok {
			t.Fatalf("expected ShowNotesMsg, got %T", cmd.Handler())
		}
		if msg.Project != scoped {
			t.Fatalf("show notes targeted %v, want scoped project %q (not stale currentProject)", msg.Project, scoped.ID)
		}
	})

	t.Run("unscoped global root surfaces explicit status, not silent no-op", func(t *testing.T) {
		app, stale := newTestAppModelWithProject()
		app.currentProject = stale // stale single-project selection

		app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
		app.crossOutliner.loadedProjects = []*models.Project{stale}
		// Header row at the unscoped global root: no cursor project, no scope.
		app.crossOutliner.rows = []outlinerRow{{IsSectionHeader: true, SectionName: "Inbox"}}
		app.crossOutliner.cursor = 0
		app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
		app.viewStack = []PresentationID{ViewDashboard, ViewOutliner, ViewCrossProjectOutliner, ViewCommandPalette}

		// Direct binding must not silently no-op; it resolves nil so the caller
		// surfaces the no-project status.
		if got := app.crossOutliner.contextProject(); got != nil {
			t.Fatalf("contextProject = %v, want nil at unscoped global root", got)
		}

		cmd := findCommand(app.notesCommands(), "show notes")
		if cmd == nil {
			t.Fatal("expected to find 'show notes' command")
		}
		result := cmd.Handler()
		status, ok := result.(StatusMsg)
		if !ok {
			t.Fatalf("expected StatusMsg (explicit no-project status), got %T", result)
		}
		if string(status) != statusNoProjectUnderCursor {
			t.Fatalf("status = %q, want %q", string(status), statusNoProjectUnderCursor)
		}
	})
}

func TestNotesCommands_CreateNote(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "create note")
	if cmd == nil {
		t.Fatal("expected to find 'create note' command")
	}
	if cmd.Shortcut != "}" {
		t.Fatalf("create note shortcut = %q, want }", cmd.Shortcut)
	}

	result := cmd.Handler()
	if _, ok := result.(ShowNoteCreatorMsg); !ok {
		t.Errorf("expected ShowNoteCreatorMsg, got %T", result)
	}
}

func TestOutlinerCtrlNIsReservedForGlobalTaskComposer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.configManager, app.runtime)

	if _, _, handled := outliner.Dispatcher.Dispatch(syntheticCtrlKey('n')); handled {
		t.Fatal("outliner must not consume global Ctrl+N Task Composer")
	}

	if _, _, handled := outliner.Dispatcher.Dispatch(syntheticCtrlKey('o')); handled {
		t.Fatal("outliner should not bind ctrl+o locally")
	}
}

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
		ViewTemporaryOutliner,
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

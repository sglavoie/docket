package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestNotesCommands_CtrlNShowNotes(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	cmds := app.notesCommands()
	cmd := findCommand(cmds, "show notes")
	if cmd == nil {
		t.Fatal("expected to find 'show notes' command")
	}
	if cmd.Shortcut != "ctrl+n" {
		t.Fatalf("show notes shortcut = %q, want ctrl+n", cmd.Shortcut)
	}
	expectedViews := []ViewType{
		ViewProjectTree,
		ViewOutliner,
		ViewTemporaryOutliner,
		ViewCrossProjectOutliner,
		ViewFocusBoard,
		ViewAllTasks,
		ViewDailyReview,
		ViewWeeklyReview,
		ViewFocusView,
		ViewProjectFocusView,
		ViewPriorityView,
		ViewDueDateOverview,
		ViewKanban,
		ViewCrossProjectKanban,
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

	app.crossOutliner = NewOutlinerModel(nil, app.config, app.configManager)
	app.crossOutliner.loadedProjects = []*models.Project{crossProject}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProject.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = []ViewType{ViewDashboard, ViewOutliner, ViewCrossProjectOutliner, ViewCommandPalette}

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
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree, ViewCommandPalette}

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

func TestProjectTreeCtrlNDispatchesShowNotes(t *testing.T) {
	app, project := newTestAppModelWithProject()
	tree := NewProjectTreeModel(project, app.config, nil)

	_, cmd, handled := tree.Dispatcher.Dispatch(syntheticCtrlKey('n'))
	if !handled {
		t.Fatal("expected ctrl+n to be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+n to return a command")
	}
	msg := cmd()
	showNotes, ok := msg.(ShowNotesMsg)
	if !ok {
		t.Fatalf("expected ShowNotesMsg, got %T", msg)
	}
	if showNotes.Project != project {
		t.Fatal("ShowNotesMsg should target the current project")
	}

	if _, _, handled := tree.Dispatcher.Dispatch(syntheticCtrlKey('o')); handled {
		t.Fatal("project tree should not bind ctrl+o locally")
	}
}

func TestOutlinerCtrlNDispatchesShowNotes(t *testing.T) {
	app, project := newTestAppModelWithProject()
	outliner := NewOutlinerModel(project, app.config, app.configManager)

	_, cmd, handled := outliner.Dispatcher.Dispatch(syntheticCtrlKey('n'))
	if !handled {
		t.Fatal("expected ctrl+n to be handled")
	}
	if cmd == nil {
		t.Fatal("expected ctrl+n to return a command")
	}
	msg := cmd()
	showNotes, ok := msg.(ShowNotesMsg)
	if !ok {
		t.Fatalf("expected ShowNotesMsg, got %T", msg)
	}
	if showNotes.Project != project {
		t.Fatal("ShowNotesMsg should target the current project")
	}

	if _, _, handled := outliner.Dispatcher.Dispatch(syntheticCtrlKey('o')); handled {
		t.Fatal("outliner should not bind ctrl+o locally")
	}
}

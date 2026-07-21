package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newCrossScoreTestProject(t *testing.T) (*models.Project, *models.Task) {
	t.Helper()

	project := &models.Project{ID: "cross-score-proj", Name: "Cross Score", Path: t.TempDir()}
	task, err := models.NewTask("cross-score-task", 1, models.TaskTypeTask, "Cross Score Task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}
	project.BuildTaskTree()
	return project, task
}

func TestOutlinerScoreKey_DispatchesCrossProjectTaskAndProject(t *testing.T) {
	app := newTestAppModel()
	project, task := newCrossScoreTestProject(t)

	outliner := NewOutlinerModel(nil, app.configManager, app.runtime)
	outliner.loadedProjects = []*models.Project{project}
	outliner.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	outliner.cursor = 0

	model, cmd, consumed := outliner.Dispatcher.Dispatch(syntheticKey('S'))
	if !consumed {
		t.Fatal("S key was not consumed")
	}
	if model != outliner {
		t.Fatalf("model = %T, want original outliner", model)
	}
	if cmd == nil {
		t.Fatal("expected ShowScoreEditorMsg command")
	}
	got := cmd()
	msg, ok := got.(ShowScoreEditorMsg)
	if !ok {
		t.Fatalf("expected ShowScoreEditorMsg, got %T", got)
	}
	if msg.Task != task {
		t.Fatalf("Task = %v, want %v", msg.Task, task)
	}
	if msg.Project != project {
		t.Fatalf("Project = %v, want %v", msg.Project, project)
	}
}

func TestOutlinerCommands_ScoreEditor_DispatchesToCrossOutliner(t *testing.T) {
	app, singleProject := newTestAppModelWithProject()
	app.outliner = NewOutlinerModel(singleProject, app.configManager, app.runtime)
	app.outliner.rebuildRows()
	advanceCursorToTask(app.outliner)
	app.views.Register(ViewOutliner, app.outliner)
	app.viewStack = append(app.viewStack, ViewOutliner)

	crossProject, crossTask := newCrossScoreTestProject(t)
	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{crossProject}
	app.crossOutliner.rows = []outlinerRow{{task: crossTask, ProjectID: crossProject.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = append(app.viewStack, ViewCrossProjectOutliner)
	app.viewStack = append(app.viewStack, ViewCommandPalette)

	cmd := findCommand(app.outlinerCommands(), "score editor")
	if cmd == nil {
		t.Fatal("expected to find score editor command")
	}
	if !slices.Contains(cmd.Views, ViewCrossProjectOutliner) {
		t.Fatalf("score editor command views = %v, want ViewCrossProjectOutliner", cmd.Views)
	}

	result := cmd.Handler()
	msg, ok := result.(ShowScoreEditorMsg)
	if !ok {
		t.Fatalf("expected ShowScoreEditorMsg, got %T", result)
	}
	if msg.Task != crossTask {
		t.Fatalf("Task = %v, want %v", msg.Task, crossTask)
	}
	if msg.Project != crossProject {
		t.Fatalf("Project = %v, want %v", msg.Project, crossProject)
	}
}

func TestScoreEditorDone_ReturningToCrossOutlinerForwardsViewDidAppear(t *testing.T) {
	app := newTestAppModel()
	project, task := newCrossScoreTestProject(t)

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewScoreEditor}

	handled, cmd := app.handleScoringMessages(ScoreEditorDoneMsg{
		Task:    task,
		Project: project,
		Saved:   true,
	})
	if !handled {
		t.Fatal("ScoreEditorDoneMsg was not handled")
	}
	if !app.crossOutliner.crossForceRebuild && len(app.crossOutliner.rows) == 0 {
		t.Fatal("expected cross outliner to be refreshed or marked for rebuild")
	}
	assertViewDidAppearCmd(t, cmd)
}

func TestScoreEditorDone_CancelReturningToCrossOutlinerForwardsViewDidAppear(t *testing.T) {
	app := newTestAppModel()
	project, task := newCrossScoreTestProject(t)

	app.crossOutliner = NewOutlinerModel(nil, app.configManager, app.runtime)
	app.crossOutliner.loadedProjects = []*models.Project{project}
	app.crossOutliner.rows = []outlinerRow{{task: task, ProjectID: project.ID}}
	app.crossOutliner.cursor = 0
	app.views.Register(ViewCrossProjectOutliner, app.crossOutliner)
	app.viewStack = []PresentationID{ViewDashboard, ViewCrossProjectOutliner, ViewScoreEditor}

	handled, cmd := app.handleScoringMessages(ScoreEditorDoneMsg{
		Task:    task,
		Project: project,
		Saved:   false,
	})
	if !handled {
		t.Fatal("ScoreEditorDoneMsg was not handled")
	}
	assertViewDidAppearCmd(t, cmd)
}

package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// A parked project is a real destination: capture and task moves must commit to
// it exactly as they do for the base workspace. What changes is the reporting.
// The task lands somewhere no base projection will ever draw it, so a status
// that named only its section or its bare project name would leave the user
// hunting for work the workspace really did save.

// parkedDestinationApp builds a durable two-project workspace and parks the
// second one through the shell's own assignment path.
func parkedDestinationApp(t *testing.T) (*AppModel, *OutlinerModel, *models.Project, *models.Project) {
	t.Helper()
	dataDir := t.TempDir()
	active := newSession2Project(t, dataDir, "active work")
	parked := newSession2Project(t, dataDir, "parked work")
	app, outliner := newSession2OutlinerApp(t, dataDir, active, parked)
	app.appService.WithPlanningArtifacts(planningReconcileArtifactBuilder(app.configManager))
	app.features = newFeatureCoordinators(app.appService)

	boundary := createParkedBoundary(t, app)
	handled, cmd := app.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID:  parked.ID,
		BoundaryID: boundary.ID,
	})
	if !handled {
		t.Fatal("AssignProjectBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)
	return app, outliner, active, parked
}

func parkedDestinationTaskCount(t *testing.T, project *models.Project) int {
	t.Helper()
	fresh := &models.Project{ID: project.ID, Slug: project.Slug, Name: project.Name, Path: project.Path}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("load %q from disk: %v", project.Slug, err)
	}
	return len(fresh.Tasks)
}

// %project into a parked owner commits, and the success status names the owner
// and its boundary so the section label cannot be read as a place to look.
func TestCaptureIntoAParkedProjectCommitsAndNamesTheBoundary(t *testing.T) {
	app, outliner, active, parked := parkedDestinationApp(t)
	before := parkedDestinationTaskCount(t, parked)

	preset, ok := app.captureComposerPreset(app.projectByID(active.ID), nil, "")
	if !ok {
		t.Fatal("captureComposerPreset did not build a preset")
	}
	composer := NewTaskComposerModel(preset, app.runtime)
	composer.SetCaptureCoordinator(app.features.Capture)
	composer.input.SetValue("%parked-work parked capture")

	cmd := composer.createCompact(false)
	if cmd == nil {
		t.Fatalf("capture into a parked project returned no command: %+v", composer.errors)
	}
	created, isCreated := cmd().(TaskCreatedMsg)
	if !isCreated || created.Result == nil {
		t.Fatalf("capture result = %#v, want TaskCreatedMsg", cmd())
	}
	if created.Result.ProjectID != parked.ID {
		t.Fatalf("capture landed in %q, want the parked project", created.Result.ProjectID)
	}

	_, refreshCmd := app.handleTaskCreated(created)

	// Read the status before draining: the refresh batch carries the status
	// timeout that clears it.
	want := parked.Name + scopeBoundarySeparator + parkedBoundaryName
	if !strings.Contains(app.statusMessage, want) {
		t.Fatalf("capture status = %q, want it to name %q", app.statusMessage, want)
	}
	if app.statusIsInfo {
		t.Fatalf("capture into a parked project reported a non-success status: %q", app.statusMessage)
	}
	runSession2AppCommand(t, app, refreshCmd)

	if got := parkedDestinationTaskCount(t, parked); got != before+1 {
		t.Fatalf("parked project holds %d tasks, want %d", got, before+1)
	}
	if outlinerHasTask(outliner, parked.ID, created.Result.TaskID) {
		t.Fatal("a task captured into a parked project appeared in the base workspace")
	}
}

// Moving a task into a parked project commits the same transaction it would for
// any destination, and the status names the boundary the task just left the base
// workspace for.
func TestMovingATaskIntoAParkedProjectCommitsAndNamesTheBoundary(t *testing.T) {
	app, outliner, active, parked := parkedDestinationApp(t)
	before := parkedDestinationTaskCount(t, parked)
	source := app.projectByID(active.ID)
	moved := source.Tasks[0]

	handled, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          moved,
		SourceProject: source,
		DestProjectID: parked.ID,
		FromGlobal:    true,
	})
	if !handled || cmd == nil {
		t.Fatalf("move into a parked project handled=%v cmd=%v", handled, cmd)
	}
	movedMsg, isMoved := cmd().(TaskMovedToProjectMsg)
	if !isMoved {
		t.Fatalf("move result = %#v, want TaskMovedToProjectMsg", cmd())
	}
	_, followup := app.handleTaskMovedToProject(movedMsg)

	want := parked.Name + scopeBoundarySeparator + parkedBoundaryName
	if !strings.Contains(app.statusMessage, want) {
		t.Fatalf("move status = %q, want it to name %q", app.statusMessage, want)
	}
	runSession2AppCommand(t, app, followup)

	if got := parkedDestinationTaskCount(t, parked); got != before+1 {
		t.Fatalf("parked project holds %d tasks, want %d", got, before+1)
	}
	if got := parkedDestinationTaskCount(t, active); got != 0 {
		t.Fatalf("source project still holds %d tasks", got)
	}
	if outlinerHasTask(outliner, parked.ID, moved.ID) {
		t.Fatal("a task moved into a parked project stayed in the base workspace")
	}
}

// The same move to an Active destination keeps naming the bare project: the
// boundary clause exists to explain an absence, so it must not appear when the
// task stays visible.
func TestMovingATaskIntoAnActiveProjectNamesOnlyTheProject(t *testing.T) {
	app, _, active, parked := parkedDestinationApp(t)
	source := app.projectByID(parked.ID)
	moved := source.Tasks[0]

	_, cmd := app.handleMoveTaskToProject(MoveTaskToProjectMsg{
		Task:          moved,
		SourceProject: source,
		DestProjectID: active.ID,
		FromGlobal:    true,
	})
	movedMsg, isMoved := cmd().(TaskMovedToProjectMsg)
	if !isMoved {
		t.Fatalf("move result = %#v, want TaskMovedToProjectMsg", cmd())
	}
	app.handleTaskMovedToProject(movedMsg)

	if strings.Contains(app.statusMessage, parkedBoundaryName) {
		t.Fatalf("move status = %q, want no boundary clause for an Active destination", app.statusMessage)
	}
	if !strings.Contains(app.statusMessage, active.Name) {
		t.Fatalf("move status = %q, want it to name the destination project", app.statusMessage)
	}
}

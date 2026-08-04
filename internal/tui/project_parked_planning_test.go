package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/app"
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// Parking clears a project's planning memberships, but the parked project is
// still fully interactive inside its explicit focus. That combination is what
// these tests pin: work planned from a parked focus is durable and visible where
// it was planned, invisible everywhere the park excluded it, and never
// resurrected by a later readmission that was not asked to restore it.

func parkedPlanningDurable(t *testing.T, fixture *session4LifecycleFixture) *config.PlanningState {
	t.Helper()
	planning, err := config.NewDomainFileStore(fixture.dataDir).LoadPlanning()
	if err != nil {
		t.Fatalf("load planning from disk: %v", err)
	}
	return planning
}

// landCrossProjectLoad runs the Outliner's own load command and hands the
// message it produces to the handler, so the test observes the exact shape
// production emits instead of one assembled by hand.
func landCrossProjectLoad(t *testing.T, cross *OutlinerModel) crossProjectsLoadedMsg {
	t.Helper()
	raw := cross.loadCrossProjectsCmd()()
	loaded, ok := raw.(crossProjectsLoadedMsg)
	if !ok {
		t.Fatalf("cross-project load returned %T", raw)
	}
	cross.handleCrossProjectsLoaded(loaded)
	return loaded
}

func planningHasTask(locators []config.ProjectTaskLocator, projectID, taskID string) bool {
	for _, locator := range locators {
		if locator.ProjectID == projectID && locator.TaskID == taskID {
			return true
		}
	}
	return false
}

// A parked focus is a working surface, not a read-only preview. Planning a
// parked task must commit like any other planning move, show up in the focus
// that planned it, and stay out of every base projection — including the
// open-loop counter, which is the one place a hidden Today reference could
// still nag the user about work they deliberately parked.
func TestPlanningInsideAParkedFocusIsDurableAndStaysOutOfTheBaseWorkspace(t *testing.T) {
	fixture, parkedProject, _ := parkedLifecycleFixture(t)
	shell := fixture.app
	parkedTask := parkedProject.Tasks[0]

	adoptParkedFocus(t, fixture.cross, shell.projectByID("Active-project"), parkedProject)

	if _, _, err := shell.applyTaskPlanningAction(
		parkedTask.ID, parkedProject.ID, app.PlanningAddToday,
	); err != nil {
		t.Fatalf("planning a parked task from its own focus was rejected: %v", err)
	}

	planning := parkedPlanningDurable(t, fixture)
	if !planningHasTask(planning.GetToday(), parkedProject.ID, parkedTask.ID) {
		t.Fatalf("Today from a parked focus was not durable: %v", planning.GetToday())
	}

	fixture.cross.crossForceRebuild = true
	fixture.cross.selectOutlinerMode(ModeToday)
	if !outlinerHasTask(fixture.cross, parkedProject.ID, parkedTask.ID) {
		t.Fatal("the focus that planned the task does not show it in Today")
	}

	loops := ComputeOpenLoops(shell.runtime, shell.todayIDs)
	if loops.TodayTotal != 0 {
		t.Fatalf("open-loop Today total = %d, want a parked Today reference to count nothing", loops.TodayTotal)
	}

	if !fixture.cross.clearProjectScope() {
		t.Fatal("leaving the parked focus reported no change")
	}
	fixture.cross.crossForceRebuild = true
	fixture.cross.rebuildRows()
	if outlinerHasTask(fixture.cross, parkedProject.ID, parkedTask.ID) {
		t.Fatal("work planned inside a parked focus followed the user back to the base workspace")
	}
	for _, mode := range []OutlinerModeID{ModeAll, ModeToday, ModeNext} {
		fixture.cross.selectOutlinerMode(mode)
		fixture.cross.crossForceRebuild = true
		fixture.cross.rebuildRows()
		if containsProject(taskRowProjectIDs(fixture.cross), parkedProject.ID) {
			t.Fatalf("mode %q admitted the parked project after it was planned from its focus", mode)
		}
	}
}

// Deleting a boundary reassigns its members; it does not undo the park. The
// memberships the park cleared were a separate committed decision, so a project
// readmitted to Active comes back with the planning it actually has, not the
// planning it had before it left.
func TestDeletingAParkedBoundaryIntoActiveDoesNotResurrectClearedPlanning(t *testing.T) {
	env := newBoundaryEnv(t, nil)
	parked := env.createBoundary(t, "On Hold")

	_, cmd := env.shell.handleBoundaryMessages(AssignProjectBoundaryMsg{
		ProjectID: env.planned.ID, BoundaryID: parked.ID,
	})
	assertNoBoundaryError(t, cmd)
	if planning := env.diskPlanning(t); len(planning.Today) != 0 || len(planning.GetNext()) != 0 {
		t.Fatalf("the fixture park did not clear planning: today=%v next=%v", planning.Today, planning.GetNext())
	}

	_, cmd = env.shell.handleBoundaryMessages(DeleteBoundaryMsg{
		BoundaryID: parked.ID, DestinationID: models.ActiveBoundaryID,
	})
	assertNoBoundaryError(t, cmd)

	catalog := env.diskCatalog(t)
	if _, ok := catalog.Boundary(parked.ID); ok {
		t.Fatal("the deleted boundary survived the commit")
	}
	if got := catalog.ProjectBoundaryID(env.planned.ID); got != models.ActiveBoundaryID {
		t.Fatalf("readmitted project landed in %q, want %q", got, models.ActiveBoundaryID)
	}

	planning := env.diskPlanning(t)
	if len(planning.Today) != 0 || len(planning.GetNext()) != 0 {
		t.Fatalf(
			"readmission resurrected memberships the park deleted: today=%v next=%v",
			planning.Today, planning.GetNext(),
		)
	}
	if len(env.shell.today) != 0 || len(env.shell.nextOrder) != 0 {
		t.Fatalf(
			"shell planning caches gained the readmitted project: today=%+v next=%+v",
			env.shell.today, env.shell.nextOrder,
		)
	}
}

// Deleting a boundary into Active while one of its projects is focused readmits
// that project mid-focus. The focus is still where the user left it, but it is
// no longer an exception: the chrome must stop naming a boundary that no longer
// exists, and leaving the focus must leave the project behind in the base
// workspace instead of shedding it with the exception.
func TestDeletingTheFocusedBoundaryIntoActiveKeepsTheProjectInTheBaseWorkspace(t *testing.T) {
	fixture, parkedProject, boundary := parkedLifecycleFixture(t)
	fixture.cross.beginParkedProjectFocus(parkedProject.ID)
	adopted := landCrossProjectLoad(t, fixture.cross)
	if adopted.focusProjectID != parkedProject.ID {
		t.Fatalf("the focus load adopted %q, want the requested parked project", adopted.focusProjectID)
	}
	if fixture.cross.scopedProjectID != parkedProject.ID {
		t.Fatalf("scoped project = %q, want the focused parked project", fixture.cross.scopedProjectID)
	}
	parkedIdentity := taskIdentityFor(parkedProject.Tasks[0], parkedProject)
	fixture.cross.ReplaceSelection([]TaskIdentity{parkedIdentity})

	parkedLabel := parkedProject.Name + scopeBoundarySeparator + boundary.Name
	if got := fixture.cross.scopedProjectChromeLabel(parkedProject.ID); got != parkedLabel {
		t.Fatalf("focus chrome = %q, want %q before the boundary is deleted", got, parkedLabel)
	}

	handled, cmd := fixture.app.handleBoundaryMessages(DeleteBoundaryMsg{
		BoundaryID: boundary.ID, DestinationID: models.ActiveBoundaryID,
	})
	if !handled {
		t.Fatal("DeleteBoundaryMsg was not handled")
	}
	assertNoBoundaryError(t, cmd)

	// The deletion asks the Outliner to reload. That reload adopts nothing: it
	// retains the focus the user is already in, which is the shape production
	// emits and the one that has to re-decide the exception.
	reload := landCrossProjectLoad(t, fixture.cross)
	if reload.focusProjectID != "" {
		t.Fatalf("a steady-state reload reported adopting %q", reload.focusProjectID)
	}
	if reload.retainedFocusProjectID != parkedProject.ID {
		t.Fatalf("the reload carried focus %q, want the retained parked project", reload.retainedFocusProjectID)
	}

	if fixture.cross.scopedProjectID != parkedProject.ID {
		t.Fatalf("scoped project = %q, want the focus the user is still in", fixture.cross.scopedProjectID)
	}
	if got := fixture.cross.scopedProjectChromeLabel(parkedProject.ID); got != parkedProject.Name {
		t.Fatalf("focus chrome = %q, want the bare name once the boundary is gone", got)
	}
	if fixture.cross.parkedFocusProjectID != "" {
		t.Fatalf("a readmitted project is still held by exception: %q", fixture.cross.parkedFocusProjectID)
	}

	if !fixture.cross.clearProjectScope() {
		t.Fatal("leaving the focus reported no change")
	}
	fixture.cross.crossForceRebuild = true
	fixture.cross.rebuildRows()
	if !containsProject(taskRowProjectIDs(fixture.cross), parkedProject.ID) {
		t.Fatal("leaving the focus shed a project the base workspace had readmitted")
	}
	if !fixture.cross.IsTaskIdentitySelected(parkedIdentity) {
		t.Fatal("leaving the focus pruned a selection the readmitted project still owns")
	}
}

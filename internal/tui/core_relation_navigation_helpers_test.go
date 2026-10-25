package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/outliner"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// A relationship visit is a round trip through a live workspace. The tests here
// drive it with the real keys and mutate what they find at the far end, because
// the return frame's whole claim — that it restores a presentation and never a
// reading of the data — is only testable against a workspace that moved while
// the user was standing in it.

const relationLifecycleTimeout = 2 * time.Second

// relationLifecycleEnv is two projects and one cross-project reference: the
// source names the target, so `g~` has somewhere to go and Esc has somewhere to
// come back to. The spare task in the origin is the nearest survivor a deleted
// source falls back to.
type relationLifecycleEnv struct {
	driver          *UITestDriver
	originProjectID string
	targetProjectID string
	sourceID        string
	spareID         string
	targetID        string
}

func newRelationLifecycleEnv(t *testing.T) *relationLifecycleEnv {
	t.Helper()
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	var sourceID, spareID, targetID string
	origin, err := workspace.addProject("Origin", func(project *models.Project) {
		spare := newFixtureTask(project, "Agree the schema")
		spare.ApplyWorkflowState(models.StateBacklog)
		spareID = spare.ID

		// The source sits inside a subtree, so Focus can be armed over a view that
		// still holds the row the hop departs from.
		source := newFixtureTask(project, "Wire the importer")
		source.ApplyWorkflowState(models.StateBacklog)
		source.ParentID = &spare.ID
		sourceID = source.ID

		// A sibling under the same parent, so deleting the source leaves a survivor
		// for the return to seat.
		sibling := newFixtureTask(project, "Draft the columns")
		sibling.ApplyWorkflowState(models.StateBacklog)
		sibling.ParentID = &spare.ID
	})
	if err != nil {
		t.Fatalf("add the origin project: %v", err)
	}
	target, err := workspace.addProject("Target", func(project *models.Project) {
		remote := newFixtureTask(project, "Publish the contract")
		remote.ApplyWorkflowState(models.StateBacklog)
		targetID = remote.ID
	})
	if err != nil {
		t.Fatalf("add the target project: %v", err)
	}
	// The reference is written in memory and persisted, because these tests are
	// about the visit rather than about how a reference is captured.
	origin.GetTaskByID(sourceID).Dependencies = []models.TaskReference{
		{ProjectID: target.ID, TaskID: targetID},
	}
	if err = storage.SaveProject(origin); err != nil {
		t.Fatalf("persist the reference: %v", err)
	}

	driver, err := workspace.launch(100, 35)
	if err != nil {
		t.Fatalf("launch the driver: %v", err)
	}
	driver.SetDrainTimeout(750 * time.Millisecond)
	// Every task here sits in Backlog, which the launch folds.
	driver.App().crossOutliner.ClearAllFolds()
	return &relationLifecycleEnv{
		driver:          driver,
		originProjectID: origin.ID,
		targetProjectID: target.ID,
		sourceID:        sourceID,
		spareID:         spareID,
		targetID:        targetID,
	}
}

func (env *relationLifecycleEnv) press(t *testing.T, keys ...string) {
	t.Helper()
	for _, key := range keys {
		if err := env.driver.SendKey(key); err != nil {
			t.Fatalf("send key %q: %v", key, err)
		}
		env.driver.drain()
	}
}

// settle presses the keys and then waits for the transaction they started, so an
// assertion reads committed state rather than a command still in flight.
func (env *relationLifecycleEnv) settle(t *testing.T, keys ...string) {
	t.Helper()
	env.press(t, keys...)
	env.driver.Wait(relationLifecycleTimeout)
}

func (env *relationLifecycleEnv) outliner() *outliner.OutlinerModel {
	return env.driver.App().crossOutliner
}

func (env *relationLifecycleEnv) cursorTo(t *testing.T, taskID string) {
	t.Helper()
	outlinerView := env.outliner()
	for index, row := range outlinerView.Rows {
		if row.IsTaskRow() && row.Task != nil && row.Task.ID == taskID {
			outlinerView.Cursor = index
			return
		}
	}
	t.Fatalf("no row holds the task %q", taskID)
}

// hop is the whole gesture: the picker over the source, and Enter on its one
// listed target.
func (env *relationLifecycleEnv) hop(t *testing.T) {
	t.Helper()
	env.cursorTo(t, env.sourceID)
	env.press(t, "g", "~", "enter")
	if env.outliner().Scope.ProjectID != env.targetProjectID {
		t.Fatalf("the hop landed in scope %q, want the target project",
			env.outliner().Scope.ProjectID)
	}
	if got := env.outliner().CurrentTaskIdentity().TaskID; got != env.targetID {
		t.Fatalf("the hop parked on %q, want the target task", got)
	}
}

// task resolves one task out of the live population, which is the reading the
// view renders from.
func (env *relationLifecycleEnv) task(t *testing.T, projectID, taskID string) *models.Task {
	t.Helper()
	project := env.driver.App().projectByID(projectID)
	if project == nil {
		t.Fatalf("project %q is no longer loaded", projectID)
	}
	return project.GetTaskByID(taskID)
}

// selectPickerRow walks a value picker's list to the row carrying one label. The
// label is read off the frame rather than counted, so a preset added above it
// does not silently retarget the press.
func (env *relationLifecycleEnv) selectPickerRow(t *testing.T, label string) {
	t.Helper()
	for range 32 {
		for _, line := range strings.Split(env.driver.Observe(ObserveOptions{}).PlainScreen, "\n") {
			if strings.Contains(line, "›") && strings.Contains(line, label) {
				return
			}
		}
		env.press(t, "down")
	}
	t.Fatalf("no picker row named %q:\n%s", label, env.driver.Observe(ObserveOptions{}).PlainScreen)
}

// requireReturnedToOrigin holds the whole of what a return owes: the origin's
// view, and the cursor on the row the picker was opened over.
func (env *relationLifecycleEnv) requireReturnedToOrigin(t *testing.T) {
	t.Helper()
	outlinerView := env.outliner()
	if outlinerView.IsProjectScoped() {
		t.Fatalf("the return left the scope at %q", outlinerView.Scope.ProjectID)
	}
	if outlinerView.HasRelationTrail() {
		t.Fatal("the return left a trail standing")
	}
	want := taskref.TaskIdentity{ProjectID: env.originProjectID, TaskID: env.sourceID}
	if got := outlinerView.CurrentTaskIdentity(); got != want {
		t.Fatalf("the return parked on %+v, want the task the hop was taken from %+v", got, want)
	}
}

package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/outliner"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskref"
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

// TestMutationsWhileVisitingLeaveTheReturnResolvingAgainstCurrentData is the
// core of the visit contract. Each case mutates the task it traveled to and
// then comes back: the mutation is committed and stays committed, and the return
// lands on the origin as it stands now rather than as the frame recorded it.
func TestMutationsWhileVisitingLeaveTheReturnResolvingAgainstCurrentData(t *testing.T) {
	for _, testCase := range []struct {
		name   string
		mutate func(t *testing.T, env *relationLifecycleEnv)
		verify func(t *testing.T, env *relationLifecycleEnv)
	}{
		{
			name: "review toggle",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.settle(t, "=")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if !env.task(t, env.targetProjectID, env.targetID).Review {
					t.Fatal("= did not flag the visited task for review")
				}
			},
		},
		{
			name: "completion",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.settle(t, "$", "d")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if got := env.task(t, env.targetProjectID, env.targetID).State(); got != models.StateDone {
					t.Fatalf("the visited task is %s, want Done", got)
				}
			},
		},
		{
			name: "deferral",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.press(t, "@")
				env.selectPickerRow(t, "In 4 hours")
				env.settle(t, "enter")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if !env.task(t, env.targetProjectID, env.targetID).IsDeferred() {
					t.Fatal("the visited task was not deferred")
				}
			},
		},
		{
			name: "dependency edit",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.press(t, "~")
				env.selectPickerRow(t, "[ ] Wire the importer")
				env.press(t, "space")
				env.settle(t, "enter")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				references := env.task(t, env.targetProjectID, env.targetID).Dependencies
				if len(references) != 1 || references[0].TaskID != env.sourceID {
					t.Fatalf("the visited task holds %+v, want one reference back to the source", references)
				}
			},
		},
		{
			name: "move",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.press(t, "%")
				env.selectPickerRow(t, "Origin")
				env.settle(t, "enter")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if env.task(t, env.targetProjectID, env.targetID) != nil {
					t.Fatal("the moved task is still in the project it left")
				}
				if env.task(t, env.originProjectID, env.targetID) == nil {
					t.Fatal("the moved task never arrived in its destination")
				}
			},
		},
		{
			name: "deletion",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.settle(t, "space", "enter", "y")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if env.task(t, env.targetProjectID, env.targetID) != nil {
					t.Fatal("the visited task survived its own deletion")
				}
			},
		},
		{
			name: "undo and redo",
			mutate: func(t *testing.T, env *relationLifecycleEnv) {
				env.settle(t, "=")
				if !env.task(t, env.targetProjectID, env.targetID).Review {
					t.Fatal("= did not flag the visited task")
				}
				env.settle(t, "u")
				if env.task(t, env.targetProjectID, env.targetID).Review {
					t.Fatal("undo did not take the flag back")
				}
				env.settle(t, "ctrl+r")
			},
			verify: func(t *testing.T, env *relationLifecycleEnv) {
				if !env.task(t, env.targetProjectID, env.targetID).Review {
					t.Fatal("redo did not put the flag forward again")
				}
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			env := newRelationLifecycleEnv(t)
			env.hop(t)

			testCase.mutate(t, env)
			if err := env.driver.App().feedback.err; err != nil {
				t.Fatalf("the mutation failed: %v", err)
			}
			testCase.verify(t, env)

			env.settle(t, "esc")
			env.requireReturnedToOrigin(t)
			// The mutation was committed by the visit and the return is presentation
			// alone, so nothing about coming back may undo it.
			testCase.verify(t, env)
		})
	}
}

// TestAFailedMutationWhileVisitingLeavesTheVisitIntact is the other half. A
// refusal publishes nothing, and it is not a navigation event either: the trail,
// the scope, and the cursor are all exactly where the user left them, so the
// retry is one key away and the way back is still open.
func TestAFailedMutationWhileVisitingLeavesTheVisitIntact(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.hop(t)

	project := env.driver.App().projectByID(env.targetProjectID)
	if project == nil {
		t.Fatal("the target project is not loaded")
	}
	if err := os.Chmod(project.Path, 0o500); err != nil {
		t.Skipf("cannot make the project directory read-only: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(project.Path, 0o700) })

	env.settle(t, "=")
	if env.driver.App().feedback.err == nil {
		t.Skip("the project directory stayed writable; the refusal path is untested here")
	}
	if env.task(t, env.targetProjectID, env.targetID).Review {
		t.Error("a rejected toggle published a flag anyway")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("a refused mutation discarded the way back")
	}
	if env.outliner().Scope.ProjectID != env.targetProjectID {
		t.Fatalf("a refused mutation moved the view to %q", env.outliner().Scope.ProjectID)
	}
	if got := env.outliner().CurrentTaskIdentity().TaskID; got != env.targetID {
		t.Fatalf("a refused mutation moved the cursor to %q", got)
	}

	if err := os.Chmod(project.Path, 0o700); err != nil {
		t.Fatalf("restore the project directory: %v", err)
	}
	env.settle(t, "esc")
	env.requireReturnedToOrigin(t)
}

// TestDeletingTheSourceWhileVisitingStillReturns is the vanished-origin case
// driven end to end: the row the picker was opened over is gone by the time Esc
// is pressed, so the return seats the nearest survivor rather than nothing.
func TestDeletingTheSourceWhileVisitingStillReturns(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.hop(t)

	// The source is deleted from underneath the visit: the population is edited
	// and written the way another process would leave it, and the view reloads.
	origin := env.driver.App().projectByID(env.originProjectID)
	if origin == nil {
		t.Fatal("the origin project is not loaded")
	}
	surviving := make([]*models.Task, 0, len(origin.Tasks))
	for _, task := range origin.Tasks {
		if task != nil && task.ID != env.sourceID {
			surviving = append(surviving, task)
		}
	}
	origin.Tasks = surviving
	origin.BuildTaskTree()
	if err := storage.SaveProject(origin); err != nil {
		t.Fatalf("persist the deletion: %v", err)
	}

	env.settle(t, "esc")
	outlinerView := env.outliner()
	if outlinerView.HasRelationTrail() {
		t.Fatal("the return left a trail standing")
	}
	identity := outlinerView.CurrentTaskIdentity()
	if identity.TaskID == env.sourceID {
		t.Fatal("the return seated a task the workspace no longer holds")
	}
	if identity.TaskID == "" {
		t.Fatalf("the return seated no row at all (rows=%d)", len(outlinerView.Rows))
	}
}

// TestNavigationLeavesLensDefinitionsAndTheActivePointerAlone is the preference
// rule. Browsing the gallery, following a reference, coming back, resetting the
// view, and relaunching are all navigation: none of them is a lens edit, so the
// saved definitions and the machine-local pointer read the same afterwards.
func TestNavigationLeavesLensDefinitionsAndTheActivePointerAlone(t *testing.T) {
	env := newRelationLifecycleEnv(t)

	// An explicit lens edit does persist, which is what gives the rest of the test
	// a pointer to hold still.
	env.press(t, "\\")
	env.selectPickerRow(t, "Save current view…")
	env.settle(t, "enter")
	env.press(t, "T", "r", "a", "i", "l")
	env.settle(t, "enter")

	machine := env.driver.App().domain.runtime.Machine()
	definitions := len(machine.NamedLenses)
	pointer := machine.ActiveLensID
	if pointer == "" {
		t.Fatal("saving the current view did not remember the lens it created")
	}
	if !env.outliner().HasActiveLens() {
		t.Fatal("the saved lens did not become live")
	}

	requireUnchanged := func(t *testing.T, after string) {
		t.Helper()
		machine := env.driver.App().domain.runtime.Machine()
		if got := len(machine.NamedLenses); got != definitions {
			t.Fatalf("%s changed the saved definitions: %d, want %d", after, got, definitions)
		}
		if got := machine.ActiveLensID; got != pointer {
			t.Fatalf("%s wrote the active-lens pointer: %q, want %q", after, got, pointer)
		}
	}

	// Browsing the gallery and putting it down again.
	env.press(t, "\\", "esc")
	requireUnchanged(t, "browsing the lens gallery")

	// Following a reference suspends the lens rather than dismissing it, so the
	// pointer is untouched on the way out.
	env.hop(t)
	if env.outliner().HasActiveLens() {
		t.Fatal("the hop left the lens applied over the destination")
	}
	requireUnchanged(t, "following a reference")

	env.settle(t, "esc")
	if !env.outliner().HasActiveLens() {
		t.Fatal("the return did not put the suspended lens back")
	}
	requireUnchanged(t, "returning from a reference")

	// The reset puts the projection down. It is not a lens edit either: the lens
	// stops being applied now and still opens at the next launch, which is exactly
	// what the untouched pointer is for.
	env.settle(t, "0")
	if env.outliner().HasActiveLens() {
		t.Fatal("the reset left the lens applied")
	}
	requireUnchanged(t, "resetting the view")

	if err := env.driver.Restart(); err != nil {
		t.Fatalf("relaunch: %v", err)
	}
	requireUnchanged(t, "relaunching")
	if !env.outliner().HasActiveLens() {
		t.Fatal("the relaunch did not reapply the remembered lens")
	}
}

// armScope puts the view inside the origin project through the real picker.
func (env *relationLifecycleEnv) armScope(t *testing.T) {
	t.Helper()
	env.press(t, "P")
	env.selectPickerRow(t, "Origin")
	env.settle(t, "enter")
	if env.outliner().Scope.ProjectID != env.originProjectID {
		t.Fatalf("the picker scoped to %q, want the origin", env.outliner().Scope.ProjectID)
	}
}

// TestEscClaimedByModalInputAndTheMenuBeforeTheCascade holds the two exemptions.
// A composer is value entry rather than a grammar and an open menu is the
// innermost thing there is, so neither press reaches the axes behind them.
func TestEscClaimedByModalInputAndTheMenuBeforeTheCascade(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)

	env.press(t, "a")
	if got := env.driver.App().currentView(); got != presentation.ViewTaskComposer {
		t.Fatalf("a opened %v, want the composer", got)
	}
	env.press(t, "esc")
	if got := env.driver.App().currentView(); got == presentation.ViewTaskComposer {
		t.Fatal("Esc did not put the composer down")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("closing the composer also spent the scope")
	}

	env.press(t, "ctrl+m")
	if !env.outliner().MenuBar.Open {
		t.Fatal("Ctrl+M did not open the menu bar")
	}
	env.press(t, "esc")
	if env.outliner().MenuBar.Open {
		t.Fatal("Esc did not close the menu bar")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("closing the menu also spent the scope")
	}
}

// TestEscSpendsPeekAndTheSelectionPairBeforeTheTrail is the top of the cascade.
// Each of the three costs its own press and none of them reaches the way back.
func TestEscSpendsPeekAndTheSelectionPairBeforeTheTrail(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.hop(t)

	env.press(t, "b")
	if !env.outliner().PeekMode {
		t.Fatal("b did not open Peek")
	}
	env.press(t, "esc")
	if env.outliner().PeekMode {
		t.Fatal("Esc did not close Peek")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("closing Peek also spent the trail")
	}

	env.press(t, "V")
	if !env.outliner().IsRangeSelectionActive() {
		t.Fatal("V did not start a range")
	}
	env.press(t, "esc")
	if env.outliner().IsRangeSelectionActive() {
		t.Fatal("Esc did not put the live range down")
	}
	if !env.outliner().MultiSelectMode {
		t.Fatal("the range left no frozen selection behind")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("freezing the range also spent the trail")
	}

	env.press(t, "esc")
	if env.outliner().MultiSelectMode {
		t.Fatal("Esc did not clear the frozen selection")
	}
	if !env.outliner().HasRelationTrail() {
		t.Fatal("clearing the selection also spent the trail")
	}

	env.settle(t, "esc")
	env.requireReturnedToOrigin(t)
}

// TestEscUnwindsTheRestoredNarrowingsInOrder is the wide leg. The origin is a
// scope holding a zoom holding a focus; the hop replaced all three at once, so
// one press hands them all back and each further press spends exactly one,
// narrowest first.
func TestEscUnwindsTheRestoredNarrowingsInOrder(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)

	env.press(t, "home", "enter")
	if !env.outliner().Focus.Zoom.Active() {
		t.Fatal("Enter on a section header did not zoom it")
	}
	env.cursorTo(t, env.spareID)
	env.press(t, "enter")
	if len(env.outliner().Focus.CrossStack) != 1 {
		t.Fatalf("Enter on a parent pushed %d focus level(s), want one",
			len(env.outliner().Focus.CrossStack))
	}

	env.hop(t)
	// The destination is the target's own project in All: none of the origin's
	// narrowings traveled with it.
	if env.outliner().Focus.Zoom.Active() || len(env.outliner().Focus.CrossStack) != 0 {
		t.Fatal("the hop carried the origin's narrowings into the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().Focus.Zoom.Active() || len(env.outliner().Focus.CrossStack) != 1 ||
		env.outliner().Scope.ProjectID != env.originProjectID {
		t.Fatalf("the frame did not hand the whole projection back: zoom=%v focus=%d scope=%q",
			env.outliner().Focus.Zoom.Active(), len(env.outliner().Focus.CrossStack),
			env.outliner().Scope.ProjectID)
	}

	env.press(t, "esc")
	if len(env.outliner().Focus.CrossStack) != 0 {
		t.Fatal("Esc did not pop the focus level first")
	}
	if !env.outliner().Focus.Zoom.Active() {
		t.Fatal("popping the focus also exited the zoom")
	}

	env.press(t, "esc")
	if env.outliner().Focus.Zoom.Active() {
		t.Fatal("Esc did not exit the zoom")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("exiting the zoom also popped the scope")
	}

	env.settle(t, "esc")
	if env.outliner().IsProjectScoped() {
		t.Fatal("Esc did not pop the project scope")
	}
	if env.outliner().PrimaryMode != chrome.ModeAll {
		t.Fatalf("the unwound view is in %s, want the launch mode", env.outliner().PrimaryMode)
	}
}

// TestEscSpendsTheTrailBeforeARestoredSearch pins the one ordering the wide leg
// cannot show: a search projection and a live lens never coexist, because
// publishing a lens clears the projection under it. Search is therefore paired
// with the trail above it and the scope below it.
func TestEscSpendsTheTrailBeforeARestoredSearch(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.armScope(t)
	env.press(t, "/", "i", "m", "p", "o", "r", "t", "e", "r", "enter")
	if !env.outliner().HasSearchProjection() {
		t.Fatal("the query did not project")
	}

	env.hop(t)
	if env.outliner().HasSearchProjection() {
		t.Fatal("the hop carried the origin's search into the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().HasSearchProjection() {
		t.Fatal("the return did not restore the search projection")
	}

	env.press(t, "esc")
	if env.outliner().HasSearchProjection() {
		t.Fatal("Esc did not clear the search")
	}
	if !env.outliner().IsProjectScoped() {
		t.Fatal("clearing the search also popped the scope")
	}

	env.settle(t, "esc")
	if env.outliner().IsProjectScoped() {
		t.Fatal("Esc did not pop the project scope")
	}
}

// TestEscSpendsTheTrailBeforeARestoredLens is the lens leg. The hop suspends the
// lens and the trail hands it back whole, so dismissing it is a second press —
// and dismissing it is what restores the baseline underneath.
func TestEscSpendsTheTrailBeforeARestoredLens(t *testing.T) {
	env := newRelationLifecycleEnv(t)
	env.press(t, "\\")
	env.selectPickerRow(t, "Save current view…")
	env.settle(t, "enter")
	env.press(t, "C", "a", "s", "c", "a", "d", "e")
	env.settle(t, "enter")
	if !env.outliner().HasActiveLens() {
		t.Fatal("saving the current view did not make it live")
	}

	env.hop(t)
	if env.outliner().HasActiveLens() {
		t.Fatal("the hop left the lens applied over the destination")
	}

	env.settle(t, "esc")
	if env.outliner().HasRelationTrail() {
		t.Fatal("Esc did not pop the trail")
	}
	if !env.outliner().HasActiveLens() {
		t.Fatal("the return did not put the suspended lens back")
	}

	env.settle(t, "esc")
	if env.outliner().HasActiveLens() {
		t.Fatal("Esc did not dismiss the lens")
	}
	if env.outliner().PrimaryMode != chrome.ModeAll {
		t.Fatalf("the unwound view is in %s, want the launch mode", env.outliner().PrimaryMode)
	}
}

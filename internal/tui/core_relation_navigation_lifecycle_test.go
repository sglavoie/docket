package tui

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

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
				env.settle(t, "w", "d")
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
				env.press(t, "space")
				sheet := env.driver.App().operations.ActionSheet
				if sheet == nil || !sheet.Surface.SetCursorToID("task-action:tui.task.delete") {
					t.Fatal("task action sheet is missing Delete")
				}
				env.settle(t, "enter", "y")
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
	env.selectPickerRow(t, "Save Current View…")
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

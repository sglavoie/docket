package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestPlanningToggleUndoRedoKeyFlow drives the Next/Today planning toggles and their
// u / ctrl+r undo-redo through the real AppModel key path (app.Update) from the
// Outliner scopes where q and w actually work — project and global — not
// by calling the toggle helpers directly. It is the FR-007-001 regression: the
// bespoke atomic planning undo used to be reachable only inside the retired planner,
// where q and w are disabled, so an immediate u/ctrl+r in the view that created
// the toggle fell through to ordinary project-history undo and could not restore
// the Next order index or Daily Plan entry as one logical action.
func TestPlanningToggleUndoRedoKeyFlow(t *testing.T) {
	ctrlR := syntheticCtrlKey('r')

	t.Run("outliner queue toggle restores state and order index", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewCrossProjectOutliner)

		app = sendAppKey(app, syntheticKey('n'))
		assertQueueKeyFlow(t, app, task.ID, true)

		app = sendAppKey(app, syntheticKey('u'))
		assertQueueKeyFlow(t, app, task.ID, false)

		app = sendAppKey(app, ctrlR)
		assertQueueKeyFlow(t, app, task.ID, true)
	})

	t.Run("global outliner daily-plan toggle restores plan entry", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewCrossProjectOutliner)

		app = sendAppKey(app, syntheticKey('t'))
		assertDailyPlanKeyFlow(t, app, task.ID, true)

		app = sendAppKey(app, syntheticKey('u'))
		assertDailyPlanKeyFlow(t, app, task.ID, false)

		app = sendAppKey(app, ctrlR)
		assertDailyPlanKeyFlow(t, app, task.ID, true)
	})
}

// assertQueueKeyFlow checks that the queue toggle moved both the workflow state
// (StateNext membership) and the Next order index together.
func assertQueueKeyFlow(t *testing.T, app *AppModel, taskID string, inQueue bool) {
	t.Helper()

	fresh := app.currentProject.GetTaskByID(taskID)
	if fresh == nil {
		t.Fatal("task missing after toggle key flow")
	}
	if got := fresh.State() == models.StateNext; got != inQueue {
		t.Fatalf("StateNext membership = %v, want %v", got, inQueue)
	}
	identity := TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.focusQueueIDs[identity] != inQueue {
		t.Fatalf("order-index membership = %v, want %v", app.focusQueueIDs[identity], inQueue)
	}
	if got := taskMembershipCount(app.focusQueue, taskID); got != boolCount(inQueue) {
		t.Fatalf("order-index slice count = %d, want %d", got, boolCount(inQueue))
	}
}

// assertDailyPlanKeyFlow checks that the daily-plan toggle moved the Today plan
// entry (both the id set and the ordered slice).
func assertDailyPlanKeyFlow(t *testing.T, app *AppModel, taskID string, inPlan bool) {
	t.Helper()

	identity := TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.dailyPlanIDs[identity] != inPlan {
		t.Fatalf("daily-plan membership = %v, want %v", app.dailyPlanIDs[identity], inPlan)
	}
	if got := taskMembershipCount(app.dailyPlan, taskID); got != boolCount(inPlan) {
		t.Fatalf("daily-plan slice count = %d, want %d", got, boolCount(inPlan))
	}
}

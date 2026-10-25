package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/taskref"
)

// TestPlanningToggleUndoRedoKeyFlow drives the Next/Today planning toggles and their
// u / ctrl+r undo-redo through the real AppModel key path (app.Update) from the
// Outliner scope where the state chord actually works, rather than
// by calling the toggle helpers directly. It is the FR-007-001 regression: the
// bespoke atomic planning undo used to be reachable only inside the retired planner,
// so an immediate u/ctrl+r in the view that created
// the toggle fell through to ordinary project-history undo and could not restore
// the Next order index or Daily Plan entry as one logical action.
func TestPlanningToggleUndoRedoKeyFlow(t *testing.T) {
	ctrlR := syntheticCtrlKey('r')

	t.Run("outliner queue toggle restores state and order index", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)

		app = sendAppKey(app, syntheticKey('w'))
		app = sendAppKey(app, syntheticKey('n'))
		assertQueueKeyFlow(t, app, task.ID, true)

		app = sendAppKey(app, syntheticKey('u'))
		assertQueueKeyFlow(t, app, task.ID, false)

		app = sendAppKey(app, ctrlR)
		assertQueueKeyFlow(t, app, task.ID, true)
	})

	t.Run("global outliner daily-plan toggle restores plan entry", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t)

		app = sendAppKey(app, syntheticKey('w'))
		app = sendAppKey(app, syntheticKey('t'))
		assertTodayKeyFlow(t, app, task.ID, true)

		app = sendAppKey(app, syntheticKey('u'))
		assertTodayKeyFlow(t, app, task.ID, false)

		app = sendAppKey(app, ctrlR)
		assertTodayKeyFlow(t, app, task.ID, true)
	})
}

// assertQueueKeyFlow checks explicit Next membership in both hydrated indexes.
func assertQueueKeyFlow(t *testing.T, app *AppModel, taskID string, inQueue bool) {
	t.Helper()

	fresh := app.currentProject.GetTaskByID(taskID)
	if fresh == nil {
		t.Fatal("task missing after toggle key flow")
	}
	if fresh.State() != models.StateBacklog {
		t.Fatalf("queue toggle changed lifecycle to %s, want backlog", fresh.State())
	}
	identity := taskref.TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.nextOrderIDs[identity] != inQueue {
		t.Fatalf("order-index membership = %v, want %v", app.nextOrderIDs[identity], inQueue)
	}
	if got := taskMembershipCount(app.nextOrder, taskID); got != boolCount(inQueue) {
		t.Fatalf("order-index slice count = %d, want %d", got, boolCount(inQueue))
	}
}

// assertTodayKeyFlow checks that the daily-plan toggle moved the Today plan
// entry (both the id set and the ordered slice).
func assertTodayKeyFlow(t *testing.T, app *AppModel, taskID string, inPlan bool) {
	t.Helper()

	identity := taskref.TaskIdentity{ProjectID: app.currentProject.ID, TaskID: taskID}
	if app.todayIDs[identity] != inPlan {
		t.Fatalf("daily-plan membership = %v, want %v", app.todayIDs[identity], inPlan)
	}
	if got := taskMembershipCount(app.today, taskID); got != boolCount(inPlan) {
		t.Fatalf("daily-plan slice count = %d, want %d", got, boolCount(inPlan))
	}
}

package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestFocusBoardBacklogPromotionFlow is the FR-007-002 regression: the Planner
// (ViewFocusBoard) is the consolidated planning surface, but it used to be
// unable to move ranked Backlog work into either planning horizon. The empty
// Today "Plan" CTA opened the Next shortlist instead of the ranked Backlog, and
// q/w were suppressed for the entire Planner with no replacement action, so a
// selected Backlog row could not be promoted to Today or Next.
//
// The fix routes the Plan CTA to the Backlog tab and lets the Planner own q/w as
// its promotion keys, dispatched through the same undoable toggle boundary
// (toggleQueue/toggleDailyPlan) as every other task view — so promotion
// preserves Today/Next exclusivity and is atomically undoable/redoable.
func TestFocusBoardBacklogPromotionFlow(t *testing.T) {
	t.Run("empty Today plan action opens the ranked Backlog", func(t *testing.T) {
		m := cockpitModel(nil, nil, nil, nil, OpenLoops{HasTasks: true}, boardTabToday)

		for _, action := range []CockpitAction{actionPlanToday(), actionPlanMore()} {
			_, cmd := m.executeCockpitAction(action)
			if cmd == nil {
				t.Fatalf("plan action %q produced no command", action.Command)
			}
			msg, ok := cmd().(ShowFocusBoardMsg)
			if !ok {
				t.Fatalf("plan action %q emitted %T, want ShowFocusBoardMsg", action.Command, cmd())
			}
			if !msg.SetTab || msg.Tab != boardTabBacklog {
				t.Fatalf("plan action %q opened tab %d (SetTab=%v), want the ranked Backlog tab",
					action.Command, msg.Tab, msg.SetTab)
			}
		}
	})

	t.Run("q sends a Backlog task to Next from the Planner", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewFocusBoard)

		app = sendAppKey(app, syntheticKey('q'))
		assertQueueKeyFlow(t, app, task.ID, true)
	})

	t.Run("w sends a Backlog task to Today from the Planner", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewFocusBoard)

		app = sendAppKey(app, syntheticKey('w'))
		assertDailyPlanKeyFlow(t, app, task.ID, true)
	})

	t.Run("promotion preserves Today/Next exclusivity", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewFocusBoard)

		app = sendAppKey(app, syntheticKey('q'))
		assertExclusiveMembership(t, app, task.ID, true, false)

		// Promoting the same task to Today must drop it out of Next.
		app = sendAppKey(app, syntheticKey('w'))
		assertExclusiveMembership(t, app, task.ID, false, true)
	})

	t.Run("promotion uses the undoable planning boundary", func(t *testing.T) {
		app, task := newFocusToggleTestAppState(t, models.StateReady, ViewFocusBoard)

		app = sendAppKey(app, syntheticKey('q'))
		assertQueueKeyFlow(t, app, task.ID, true)

		// u undoes the promotion (state + order index) as one logical action,
		// without leaving the Planner.
		app = sendAppKey(app, syntheticKey('u'))
		assertQueueKeyFlow(t, app, task.ID, false)

		// ctrl+r replays it.
		app = sendAppKey(app, syntheticCtrlKey('r'))
		assertQueueKeyFlow(t, app, task.ID, true)
	})
}

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/app"
	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/tui/operation"
	"github.com/sglavoie/docket/internal/util"
)

// Clearing a deferral preserves the lifecycle the deferral left behind: it
// un-hides the task, it does not resurrect Doing, and it invents no planning
// membership on the way back.
func TestClearingADeferralPreservesTheLifecycleTheDeferralLeft(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "hidden then revealed")
	task.Start()
	env := newDurableDatePickerEnv(t, []*models.Task{task})

	env.crossField(t, false)
	env.press(t, "1")
	if stored := env.storedTask(t, 0); stored.DeferredUntil == nil || stored.State() != models.StateBacklog {
		t.Fatalf("setup deferral = %v in %s, want a deferral in Backlog", stored.DeferredUntil, stored.State())
	}

	env.reopen(t)
	env.crossField(t, false)
	env.press(t, "0")

	stored := env.storedTask(t, 0)
	if stored.DeferredUntil != nil {
		t.Fatalf("`0` in the defer field left deferral %s standing", stored.DeferredUntil)
	}
	if got := stored.State(); got != models.StateBacklog {
		t.Fatalf("clearing the deferral moved the task to %s, want the Backlog it was left in", got)
	}
	if env.plannedToday(t) {
		t.Fatal("clearing a deferral planned the task for today")
	}

	// A revealed task is plannable again, which is the point of clearing.
	env.reopen(t)
	env.press(t, "t")
	if !env.plannedToday(t) {
		t.Fatal("a task whose deferral was cleared cannot be planned for today")
	}
}

// A selection mixing a terminal task with an open one cannot be deferred, and
// the whole defer field says so rather than deferring the half that qualifies.
// Due stays reachable on that same selection, which is the point of refusing
// per field instead of per picker.
func TestAMixedTerminalSelectionRefusesTheDeferralWithoutAPartialWrite(t *testing.T) {
	week := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	open := newBulkPickerTestTask(t, 1, "still open")
	done := newBulkPickerTestTask(t, 2, "already done")
	done.Complete()
	env := newDurableDatePickerEnv(t, []*models.Task{open, done})

	env.crossField(t, false)
	if reason := env.pressRefusedRow(t, "m"); !strings.Contains(reason, "cannot defer a done task") {
		t.Fatalf("the defer field refused with %q, want the done task named", reason)
	}
	for index := range env.tasks {
		if stored := env.storedTask(t, index); stored.DeferredUntil != nil {
			t.Fatalf("the refused deferral still wrote %s to %q", stored.DeferredUntil, stored.Title)
		}
	}

	env.crossField(t, true)
	env.press(t, "w")
	for index := range env.tasks {
		stored := env.storedTask(t, index)
		if stored.DueDate == nil || !stored.DueDate.Equal(week) {
			t.Fatalf("%q stored due date %v, want one week out %s", stored.Title, stored.DueDate, week)
		}
	}
	if got := env.storedTask(t, 1).State(); got != models.StateDone {
		t.Fatalf("a deadline moved the terminal task to %s, want %s", got, models.StateDone)
	}
}

// The request froze its targets when the picker opened. A target that no longer
// resolves is the one authoritative refusal, and the task the cursor happens to
// name is not a substitute for it.
func TestATargetRemovedWhileThePickerIsOpenRefusesInsteadOfFallingBackToTheCursor(t *testing.T) {
	doomed := newBulkPickerTestTask(t, 1, "removed underneath")
	bystander := newBulkPickerTestTask(t, 2, "never selected")
	env, view := newFrozenSelectionDateEnv(
		t, operation.ValueProviderDate, 1, []*models.Task{doomed, bystander},
	)

	// The cursor walks onto the unselected row while the picker is open, which is
	// exactly the fallback this refusal must not take.
	view.Cursor = 1
	env.removeFromWorkspace(t, 0)

	err := env.pressRefusal(t, "w")
	if !errors.Is(err, app.ErrTaskNotFound) {
		t.Fatalf("refusal = %v, want %v", err, app.ErrTaskNotFound)
	}
	if stored := env.storedTask(t, 1); stored.DueDate != nil {
		t.Fatalf("the refused commit dated the cursor's task instead: %s", stored.DueDate)
	}
}

// A row the picker drew as available is not a promise. A target that finished
// while the picker was open is re-judged against a fresh snapshot at commit
// time, and one ineligible member refuses the whole batch: the open task beside
// it keeps its undeferred state rather than taking half the keystroke.
func TestATargetCompletedWhileThePickerIsOpenRefusesTheWholeDeferral(t *testing.T) {
	stillOpen := newBulkPickerTestTask(t, 1, "still open at commit time")
	finished := newBulkPickerTestTask(t, 2, "finished under the picker")
	env := newDurableDatePickerEnv(t, []*models.Task{stillOpen, finished})

	// The defer rows were built while both tasks qualified, so the refusal cannot
	// come from the picker: only the application layer sees the completion.
	env.crossField(t, false)
	env.completeInWorkspace(t, 1)

	err := env.pressRefusal(t, "m")
	if !strings.Contains(err.Error(), "cannot defer a done task") {
		t.Fatalf("refusal = %v, want the done task named", err)
	}
	for index := range env.tasks {
		if stored := env.storedTask(t, index); stored.DeferredUntil != nil {
			t.Fatalf("the refused batch still deferred %q until %s", stored.Title, stored.DeferredUntil)
		}
	}
	if got := env.storedTask(t, 0).State(); got != models.StateBacklog {
		t.Fatalf("the open member is in %s, want the Backlog a refused deferral leaves it in", got)
	}
}

// One keystroke over a selection spanning two projects is one transaction: both
// projects are written, the cursor's unselected task is not, and one ledger
// entry takes the whole thing back.
func TestACrossProjectPresetKeyCommitsTheFrozenSelectionAfterTheCursorMoves(t *testing.T) {
	week := util.StartOfDay(time.Now()).AddDate(0, 0, 7)
	first := newBulkPickerTestTask(t, 1, "first project")
	second := newBulkPickerTestTask(t, 1, "second project")
	bystander := newBulkPickerTestTask(t, 2, "unselected neighbor")
	// The selection is the first two seeded tasks, one per project; the bystander
	// shares the second project and is only ever under the cursor.
	env, view := newFrozenSelectionDateEnv(
		t, operation.ValueProviderDate, 2,
		[]*models.Task{first}, []*models.Task{second, bystander},
	)

	view.Cursor = 2
	batch := env.pressBatch(t, "w")
	if len(batch.TouchedProjectIDs) != 2 {
		t.Fatalf("touched projects = %v, want both selected projects", batch.TouchedProjectIDs)
	}
	for _, index := range []int{0, 1} {
		stored := env.storedTask(t, index)
		if stored.DueDate == nil || !stored.DueDate.Equal(week) {
			t.Fatalf("%q stored due date %v, want one week out %s", stored.Title, stored.DueDate, week)
		}
	}
	if stored := env.storedTask(t, 2); stored.DueDate != nil {
		t.Fatalf("the unselected task under the cursor was dated %s", stored.DueDate)
	}

	env.env.undo(t)
	for _, index := range []int{0, 1} {
		if stored := env.storedTask(t, index); stored.DueDate != nil {
			t.Fatalf("one undo left %q dated %s, so the two projects were not one entry", stored.Title, stored.DueDate)
		}
	}
}

package coordinator

import (
	"fmt"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/app"
)

func locator(projectID, taskID string) app.TaskLocator {
	return app.TaskLocator{ProjectID: projectID, TaskID: taskID}
}

func at(hour, minute int) *time.Time {
	moment := time.Date(2026, 9, 11, hour, minute, 0, 0, time.Local)
	return &moment
}

// clearedStep is the outcome a step that walked a task out of a timed deadline
// into unset reports: a Before carrying the wall clock and no After.
func clearedStep(target app.TaskLocator, hour, minute int) app.DueStepOutcome {
	return app.DueStepOutcome{Target: target, Before: at(hour, minute)}
}

func rememberedClock(t *testing.T, controller *DueClockController, target app.TaskLocator) (app.DueStepClock, bool) {
	t.Helper()
	clock, ok := controller.RememberedClocks(nil)[target]
	return clock, ok
}

func assertRemembered(t *testing.T, controller *DueClockController, target app.TaskLocator, hour, minute int) {
	t.Helper()
	clock, ok := rememberedClock(t, controller, target)
	if !ok {
		t.Fatalf("expected a remembered clock for %v", target)
	}
	if clock.Hour != hour || clock.Minute != minute {
		t.Fatalf("remembered %02d:%02d for %v, want %02d:%02d", clock.Hour, clock.Minute, target, hour, minute)
	}
}

func assertForgotten(t *testing.T, controller *DueClockController, target app.TaskLocator) {
	t.Helper()
	if clock, ok := rememberedClock(t, controller, target); ok {
		t.Fatalf("expected no remembered clock for %v, got %02d:%02d", target, clock.Hour, clock.Minute)
	}
}

// One UUID can name two different tasks in two different projects, so memory
// keyed by anything less than the qualified address would cross them.
func TestDueClockControllerKeysByQualifiedAddress(t *testing.T) {
	shared := "11111111-1111-1111-1111-111111111111"
	left, right := locator("alpha", shared), locator("beta", shared)

	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(left, 14, 30)}})

	assertRemembered(t, &controller, left, 14, 30)
	assertForgotten(t, &controller, right)

	controller.ObserveCommit("t2", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(right, 9, 5)}})
	assertRemembered(t, &controller, left, 14, 30)
	assertRemembered(t, &controller, right, 9, 5)

	// Forgetting one address leaves the other alone.
	controller.ObserveCommit("t3", CommittedDueEffect{Edits: []app.TaskLocator{left}})
	assertForgotten(t, &controller, left)
	assertRemembered(t, &controller, right, 9, 5)
}

// A step that landed back on a deadline has put the clock on the task, so there
// is nothing left to remember for it.
func TestDueClockControllerClearAndRestoreChain(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController

	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 16, 45)}})
	assertRemembered(t, &controller, target, 16, 45)

	restored := at(16, 45)
	controller.ObserveCommit("t2", CommittedDueEffect{Steps: []app.DueStepOutcome{{Target: target, After: restored}}})
	assertForgotten(t, &controller, target)

	controller.ObserveCommit("t3", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 16, 45)}})
	assertRemembered(t, &controller, target, 16, 45)
}

// Local midnight is the durable date-only signal, so clearing a date-only
// deadline remembers nothing: restoring it would invent a clock.
func TestDueClockControllerIgnoresDateOnlyDeadline(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController

	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 0, 0)}})
	assertForgotten(t, &controller, target)

	// A date-only clear must also retire a clock an earlier step remembered.
	controller.ObserveCommit("t2", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 8, 15)}})
	assertRemembered(t, &controller, target, 8, 15)
	controller.ObserveCommit("t3", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 0, 0)}})
	assertForgotten(t, &controller, target)
}

// An explicit deadline instruction overrides what was remembered; a transaction
// that named no deadline at all leaves memory alone.
func TestDueClockControllerExplicitEditOverridesAndUnrelatedEditDoesNot(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 11, 20)}})

	// A title-only or comment-only edit reports an empty effect.
	controller.ObserveCommit("t2", CommittedDueEffect{})
	assertRemembered(t, &controller, target, 11, 20)

	controller.ObserveCommit("t3", CommittedDueEffect{Edits: []app.TaskLocator{target}})
	assertForgotten(t, &controller, target)
}

// A refused mutation never reports a committed effect, so memory is untouched.
func TestDueClockControllerRefusedOutcomeLeavesMemoryIntact(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 7, 0)}})

	// Nothing is observed at all: the refusal resolved to an error message.
	assertRemembered(t, &controller, target, 7, 0)

	// A restore that failed reports no transaction either.
	controller.ObserveRestore("", false)
	assertRemembered(t, &controller, target, 7, 0)
}

// A move carries memory to the new address, and an undo carries it back.
func TestDueClockControllerMoveUndoRedo(t *testing.T) {
	source, destination := locator("alpha", "task-1"), locator("beta", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(source, 13, 0)}})

	controller.ObserveCommit("t2", CommittedDueEffect{
		Moves: []app.TaskAddressChange{{From: source, To: destination}},
	})
	assertForgotten(t, &controller, source)
	assertRemembered(t, &controller, destination, 13, 0)

	controller.ObserveRestore("t2", false)
	assertRemembered(t, &controller, source, 13, 0)
	assertForgotten(t, &controller, destination)

	controller.ObserveRestore("t2", true)
	assertForgotten(t, &controller, source)
	assertRemembered(t, &controller, destination, 13, 0)
}

// A descendant travels inside its ancestor's subtree without ever being named by
// the request, so the address map has to name it too.
func TestDueClockControllerMoveCarriesDescendants(t *testing.T) {
	root, child := locator("alpha", "root"), locator("alpha", "child")
	movedRoot, movedChild := locator("beta", "root"), locator("beta", "child")

	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedStep(root, 10, 0),
		clearedStep(child, 17, 30),
	}})
	controller.ObserveCommit("t2", CommittedDueEffect{Moves: []app.TaskAddressChange{
		{From: root, To: movedRoot},
		{From: child, To: movedChild},
	}})

	assertRemembered(t, &controller, movedRoot, 10, 0)
	assertRemembered(t, &controller, movedChild, 17, 30)
	assertForgotten(t, &controller, root)
	assertForgotten(t, &controller, child)
}

// One transaction can both relocate a task and name its deadline. The override
// wins, and the undo has to put the memory back at the source address.
func TestDueClockControllerMoveAndEditInOneTransaction(t *testing.T) {
	source, destination := locator("alpha", "task-1"), locator("beta", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(source, 21, 15)}})

	controller.ObserveCommit("t2", CommittedDueEffect{
		Moves: []app.TaskAddressChange{{From: source, To: destination}},
		Edits: []app.TaskLocator{destination},
	})
	assertForgotten(t, &controller, source)
	assertForgotten(t, &controller, destination)

	controller.ObserveRestore("t2", false)
	assertRemembered(t, &controller, source, 21, 15)
	assertForgotten(t, &controller, destination)
}

// A delete drops the address outright, and its undo puts the clock back on the
// task it belonged to rather than on whoever holds that address next.
func TestDueClockControllerDeletionAndUndo(t *testing.T) {
	target, bystander := locator("alpha", "task-1"), locator("alpha", "task-2")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedStep(target, 12, 40),
		clearedStep(bystander, 6, 10),
	}})

	controller.ObserveCommit("t2", CommittedDueEffect{Removed: []app.TaskLocator{target}})
	assertForgotten(t, &controller, target)
	assertRemembered(t, &controller, bystander, 6, 10)

	controller.ObserveRestore("t2", false)
	assertRemembered(t, &controller, target, 12, 40)
	assertRemembered(t, &controller, bystander, 6, 10)
}

// A deadline the shell observed on a reload retires the clock it superseded,
// whoever wrote it. A target the shell could not read says nothing, and a target
// still without a deadline keeps what it had.
func TestDueClockControllerReconcileObservedDeadlines(t *testing.T) {
	external, unavailable, untouched := locator("alpha", "a"), locator("alpha", "b"), locator("alpha", "c")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedStep(external, 8, 0),
		clearedStep(unavailable, 9, 0),
		clearedStep(untouched, 10, 0),
	}})

	// The unavailable target is simply absent from the observation list.
	controller.ReconcileObservedDeadlines([]ObservedDeadline{
		{Target: external, HasDeadline: true},
		{Target: untouched, HasDeadline: false},
	})

	assertForgotten(t, &controller, external)
	assertRemembered(t, &controller, unavailable, 9, 0)
	assertRemembered(t, &controller, untouched, 10, 0)
}

// A reconcile is an observation, not a transaction: no ledger restore can take
// it back, so it records no delta an undo could replay.
func TestDueClockControllerReconcileIsNotUndoable(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 15, 45)}})
	controller.ReconcileObservedDeadlines([]ObservedDeadline{{Target: target, HasDeadline: true}})
	assertForgotten(t, &controller, target)

	if len(controller.undo) != 1 || controller.undo[0].transactionID != "t1" {
		t.Fatalf("reconcile wrote a history entry: %v", controller.undo)
	}
	if len(controller.redo) != 0 {
		t.Fatalf("reconcile touched the redo branch: %v", controller.redo)
	}

	// Undoing the step that remembered the clock is still exactly reversible.
	controller.ObserveRestore("t1", false)
	assertForgotten(t, &controller, target)
	controller.ObserveRestore("t1", true)
	assertRemembered(t, &controller, target, 15, 45)
}

// A forward commit abandons the ledger's redo branch, so the deltas that branch
// owned are pruned with it instead of being replayed onto a history that no
// longer leads there.
func TestDueClockControllerForwardCommitPrunesRedoBranch(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 18, 0)}})
	controller.ObserveRestore("t1", false)
	assertForgotten(t, &controller, target)

	controller.ObserveCommit("t2", CommittedDueEffect{})
	// t1 is no longer reachable: its redo delta went with the abandoned branch.
	controller.ObserveRestore("t1", true)
	assertForgotten(t, &controller, target)
}

// A transaction that committed nothing durable has no ledger entry to be undone
// with, so its memory effect applies and is simply not reversible.
func TestDueClockControllerUnrecordedTransactionStillApplies(t *testing.T) {
	target := locator("alpha", "task-1")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 19, 30)}})

	controller.ObserveCommit("", CommittedDueEffect{Edits: []app.TaskLocator{target}})
	assertForgotten(t, &controller, target)

	// t1 is still the top of the undo stack, because the no-op edit recorded none.
	controller.ObserveRestore("t1", false)
	assertForgotten(t, &controller, target)
}

// A restart starts from the zero value, so a ledger transaction it never saw
// restores durable state without inventing a remembered clock for it.
func TestDueClockControllerRestartKnowsNoTransaction(t *testing.T) {
	target := locator("alpha", "task-1")
	var before DueClockController
	before.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(target, 20, 5)}})
	assertRemembered(t, &before, target, 20, 5)

	var after DueClockController
	if got := after.RememberedClocks(nil); len(got) != 0 {
		t.Fatalf("a restarted session remembered %d clock(s)", len(got))
	}
	after.ObserveRestore("t1", false)
	assertForgotten(t, &after, target)
}

// The history is capped at the ledger's own depth, so a delta older than any
// entry the user can reach is dropped rather than kept forever.
func TestDueClockControllerHistoryIsCappedAtLedgerDepth(t *testing.T) {
	oldest := locator("alpha", "task-0")
	var controller DueClockController
	controller.ObserveCommit("t0", CommittedDueEffect{Steps: []app.DueStepOutcome{clearedStep(oldest, 1, 1)}})
	for i := 1; i <= dueClockHistoryLimit; i++ {
		target := locator("alpha", fmt.Sprintf("task-%d", i))
		controller.ObserveCommit(fmt.Sprintf("t%d", i), CommittedDueEffect{
			Steps: []app.DueStepOutcome{clearedStep(target, 2, 2)},
		})
	}
	if len(controller.undo) != dueClockHistoryLimit {
		t.Fatalf("undo history holds %d entries, want %d", len(controller.undo), dueClockHistoryLimit)
	}
	if controller.undo[0].transactionID != "t1" {
		t.Fatalf("oldest retained delta is %q, want %q", controller.undo[0].transactionID, "t1")
	}
}

// RememberedClocks answers for exactly the targets it was asked about, and hands
// back a map detached from the controller's own.
func TestDueClockControllerRememberedClocksIsScopedAndDetached(t *testing.T) {
	first, second, absent := locator("alpha", "a"), locator("alpha", "b"), locator("alpha", "c")
	var controller DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Steps: []app.DueStepOutcome{
		clearedStep(first, 3, 15),
		clearedStep(second, 4, 45),
	}})

	scoped := controller.RememberedClocks([]app.TaskLocator{first, absent})
	if len(scoped) != 1 {
		t.Fatalf("scoped lookup returned %d clock(s), want 1", len(scoped))
	}
	if _, ok := scoped[first]; !ok {
		t.Fatalf("scoped lookup lost the target it was asked about")
	}

	all := controller.RememberedClocks(nil)
	delete(all, first)
	all[absent] = app.DueStepClock{Hour: 23}
	assertRemembered(t, &controller, first, 3, 15)
	assertForgotten(t, &controller, absent)
}

// Every method tolerates the nil receiver a surface without clock memory holds.
func TestDueClockControllerNilReceiver(t *testing.T) {
	var controller *DueClockController
	controller.ObserveCommit("t1", CommittedDueEffect{Edits: []app.TaskLocator{locator("alpha", "a")}})
	controller.ObserveRestore("t1", false)
	controller.ReconcileObservedDeadlines([]ObservedDeadline{{Target: locator("alpha", "a"), HasDeadline: true}})
	if got := controller.RememberedClocks(nil); got != nil {
		t.Fatalf("nil controller remembered %v", got)
	}
}

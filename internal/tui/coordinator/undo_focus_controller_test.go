package coordinator

import (
	"fmt"
	"testing"

	"github.com/sglavoie/docket/internal/app"
)

func TestFocusTargetFromSourceFollowsTheCommittedMove(t *testing.T) {
	source := locator("alpha", "one")
	moves := []app.TaskAddressChange{
		{From: locator("alpha", "child"), To: locator("beta", "child")},
		{From: source, To: locator("beta", "one")},
	}
	target := FocusTargetFromSource(source, moves)
	if target.Before != source || target.After != locator("beta", "one") {
		t.Fatalf("moved target = %+v, want alpha/one -> beta/one", target)
	}

	stayed := FocusTargetFromSource(source, []app.TaskAddressChange{
		{From: locator("alpha", "child"), To: locator("beta", "child")},
	})
	if stayed.Before != source || stayed.After != source {
		t.Fatalf("unmoved target = %+v, want alpha/one on both sides", stayed)
	}

	if got := FocusTargetFromSource(app.TaskLocator{}, moves); got.Valid() {
		t.Fatalf("unaddressed source = %+v, want the zero pair", got)
	}
}

func TestFocusTargetFromResultWalksTheMoveMapBackwards(t *testing.T) {
	result := locator("beta", "one")
	target := FocusTargetFromResult(result, []app.TaskAddressChange{
		{From: locator("alpha", "one"), To: result},
	})
	if target.Before != locator("alpha", "one") || target.After != result {
		t.Fatalf("moved target = %+v, want alpha/one -> beta/one", target)
	}
}

func TestUndoFocusRestoreReturnsTheAddressEachDirectionPutsTheTaskBackAt(t *testing.T) {
	var controller UndoFocusController
	controller.ObserveCommit("tx-move", CommittedFocusTarget{
		Before: locator("alpha", "one"),
		After:  locator("beta", "one"),
	})

	address, ok := controller.ObserveRestore("tx-move", false)
	if !ok || address != locator("alpha", "one") {
		t.Fatalf("undo address = %+v/%v, want alpha/one", address, ok)
	}
	address, ok = controller.ObserveRestore("tx-move", true)
	if !ok || address != locator("beta", "one") {
		t.Fatalf("redo address = %+v/%v, want beta/one", address, ok)
	}
	// The entry is back on the undo stack, so the pair is repeatable.
	if address, ok := controller.ObserveRestore("tx-move", false); !ok || address != locator("alpha", "one") {
		t.Fatalf("second undo address = %+v/%v, want alpha/one", address, ok)
	}
}

func TestUndoFocusLeavesUncorrelatedTransactionsAlone(t *testing.T) {
	var controller UndoFocusController
	controller.ObserveCommit("tx-single", CommittedFocusTarget{
		Before: locator("alpha", "one"), After: locator("alpha", "one"),
	})
	// A bulk gesture names no single target, so it records nothing and the stack
	// stays a subsequence of the ledger's.
	controller.ObserveCommit("tx-bulk", CommittedFocusTarget{})

	if _, ok := controller.ObserveRestore("tx-bulk", false); ok {
		t.Fatal("bulk transaction correlated a focus target")
	}
	// Undoing the bulk entry must not have spent the single-task one below it.
	if address, ok := controller.ObserveRestore("tx-single", false); !ok || address != locator("alpha", "one") {
		t.Fatalf("single-task address = %+v/%v, want alpha/one", address, ok)
	}
	if _, ok := controller.ObserveRestore("tx-unknown", false); ok {
		t.Fatal("unknown transaction correlated a focus target")
	}
	if _, ok := controller.ObserveRestore("", false); ok {
		t.Fatal("empty transaction id correlated a focus target")
	}
}

func TestUndoFocusCreationCorrelatesOnlyTheRedoDirection(t *testing.T) {
	var controller UndoFocusController
	controller.ObserveCommit("tx-create", CommittedFocusTarget{After: locator("alpha", "one")})

	if _, ok := controller.ObserveRestore("tx-create", false); ok {
		t.Fatal("undoing a creation named an address to focus")
	}
	// The entry still moved to the redo stack, so replaying the creation finds it.
	if address, ok := controller.ObserveRestore("tx-create", true); !ok || address != locator("alpha", "one") {
		t.Fatalf("redo address = %+v/%v, want alpha/one", address, ok)
	}
}

func TestUndoFocusForwardCommitAbandonsTheRedoBranch(t *testing.T) {
	var controller UndoFocusController
	controller.ObserveCommit("tx-one", CommittedFocusTarget{Before: locator("alpha", "one"), After: locator("alpha", "one")})
	if _, ok := controller.ObserveRestore("tx-one", false); !ok {
		t.Fatal("undo did not correlate the recorded transaction")
	}
	controller.ObserveCommit("tx-two", CommittedFocusTarget{Before: locator("alpha", "two"), After: locator("alpha", "two")})
	if _, ok := controller.ObserveRestore("tx-one", true); ok {
		t.Fatal("a redo branch survived a forward commit")
	}
}

func TestUndoFocusHistoryStaysBounded(t *testing.T) {
	var controller UndoFocusController
	for i := range undoFocusHistoryLimit + 10 {
		controller.ObserveCommit(
			fmt.Sprintf("tx-%d", i),
			CommittedFocusTarget{Before: locator("alpha", "one"), After: locator("alpha", "one")},
		)
	}
	if len(controller.undo) != undoFocusHistoryLimit {
		t.Fatalf("undo history depth = %d, want %d", len(controller.undo), undoFocusHistoryLimit)
	}
}

func TestUndoFocusNilReceiverIsInert(t *testing.T) {
	var controller *UndoFocusController
	controller.ObserveCommit("tx", CommittedFocusTarget{Before: locator("alpha", "one")})
	if _, ok := controller.ObserveRestore("tx", false); ok {
		t.Fatal("nil controller correlated a transaction")
	}
}

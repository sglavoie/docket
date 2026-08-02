package models

import (
	"testing"
	"time"
)

// TestAddChild_NilChildReturnsError verifies AddChild rejects nil children.
// Before the fix, AddChild(nil) panicked on child.Type access in CanAddChild.
func TestAddChild_NilChildReturnsError(t *testing.T) {
	parent, err := NewGenericTask("p", 1, "Parent")
	if err != nil {
		t.Fatal(err)
	}
	if err := parent.AddChild(nil); err == nil {
		t.Error("expected error for nil child, got nil")
	}
	if len(parent.Children) != 0 {
		t.Errorf("expected Children unchanged, got %d", len(parent.Children))
	}
}

// TestMutateTask_NilFnReturnsFalse verifies MutateTask rejects a nil callback.
// Before the fix, calling MutateTask(id, nil) on a matching task panicked
// when fn was invoked, leaving the iteration in an inconsistent state.
func TestMutateTask_NilFnReturnsFalse(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewGenericTask("t1", 1, "T")
	if err != nil {
		t.Fatal(err)
	}
	p.Tasks = append(p.Tasks, task)
	originalUpdatedAt := task.UpdatedAt

	if p.MutateTask("t1", nil) {
		t.Error("expected false for nil fn, got true")
	}
	if !task.UpdatedAt.Equal(originalUpdatedAt) {
		t.Error("UpdatedAt should not change when fn is nil")
	}
}

// TestMoveTaskUp_UpdatesBothTimestamps verifies that swapping two tasks via
// MoveTaskUp touches both tasks' UpdatedAt. Before the fix, only the moved
// task's timestamp was bumped, leaving the displaced sibling with a stale
// modification time despite its position changing — the same hazard that
// previously affected SwapTasks.
func TestMoveTaskUp_UpdatesBothTimestamps(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewGenericTask("a", 1, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewGenericTask("b", 2, "B")
	if err != nil {
		t.Fatal(err)
	}
	// Backdate UpdatedAt so we can detect a refresh.
	past := time.Now().Add(-time.Hour)
	a.UpdatedAt = past
	b.UpdatedAt = past
	p.Tasks = []*Task{a, b}

	if err := p.MoveTaskUp(b); err != nil {
		t.Fatalf("MoveTaskUp: %v", err)
	}
	if p.Tasks[0].ID != "b" || p.Tasks[1].ID != "a" {
		t.Fatalf("expected order [b, a], got [%s, %s]", p.Tasks[0].ID, p.Tasks[1].ID)
	}
	if !b.UpdatedAt.After(past) {
		t.Error("moved task b should have refreshed UpdatedAt")
	}
	if !a.UpdatedAt.After(past) {
		t.Error("displaced sibling a should have refreshed UpdatedAt")
	}
}

// TestMoveTaskDown_UpdatesBothTimestamps mirrors TestMoveTaskUp_UpdatesBothTimestamps
// for MoveTaskDown.
func TestMoveTaskDown_UpdatesBothTimestamps(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	a, err := NewGenericTask("a", 1, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewGenericTask("b", 2, "B")
	if err != nil {
		t.Fatal(err)
	}
	past := time.Now().Add(-time.Hour)
	a.UpdatedAt = past
	b.UpdatedAt = past
	p.Tasks = []*Task{a, b}

	if err := p.MoveTaskDown(a); err != nil {
		t.Fatalf("MoveTaskDown: %v", err)
	}
	if p.Tasks[0].ID != "b" || p.Tasks[1].ID != "a" {
		t.Fatalf("expected order [b, a], got [%s, %s]", p.Tasks[0].ID, p.Tasks[1].ID)
	}
	if !a.UpdatedAt.After(past) {
		t.Error("moved task a should have refreshed UpdatedAt")
	}
	if !b.UpdatedAt.After(past) {
		t.Error("displaced sibling b should have refreshed UpdatedAt")
	}
}

// TestSwapTasks_UpdatesBothTimestamps verifies that SwapTasks updates the
// UpdatedAt of BOTH swapped tasks, not just task1. Before the fix only
// task1.UpdatedAt was set, which left task2 with a stale modification
// timestamp despite the fact that its position in the Tasks slice changed.
// The markdown round-trip orders tasks by slice position, so a stale
// UpdatedAt on task2 obscures when its location last changed.
func TestSwapTasks_UpdatesBothTimestamps(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	task1, err := NewGenericTask("a", 1, "A")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewGenericTask("b", 2, "B")
	if err != nil {
		t.Fatal(err)
	}
	// Backdate both tasks so we can detect a fresh UpdatedAt.
	stale := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	task1.UpdatedAt = stale
	task2.UpdatedAt = stale
	p.Tasks = []*Task{task1, task2}

	p.SwapTasks(task1, task2)

	if task1.UpdatedAt.Equal(stale) {
		t.Error("task1.UpdatedAt should be refreshed after swap")
	}
	if task2.UpdatedAt.Equal(stale) {
		t.Error("task2.UpdatedAt should be refreshed after swap (regression: only task1 was touched)")
	}
}

// TestWouldCreateCycle_DownstreamCycleDoesNotHang verifies WouldCreateCycle
// terminates when proposedParentID lies inside a pre-existing cycle that does
// not involve taskID (e.g. A->B->C->A). Before the fix, the walk would loop
// indefinitely through the downstream cycle, hanging the call.
func TestWouldCreateCycle_DownstreamCycleDoesNotHang(t *testing.T) {
	bID := "b"
	cID := "c"
	aID := "a"
	taskA := &Task{ID: aID, ParentID: &bID}
	taskB := &Task{ID: bID, ParentID: &cID}
	taskC := &Task{ID: cID, ParentID: &aID}
	taskMap := map[string]*Task{aID: taskA, bID: taskB, cID: taskC}

	done := make(chan bool, 1)
	go func() {
		// taskID "outside" is not part of the A-B-C cycle; the walk should
		// detect the visited-loop and return false rather than spinning.
		_ = WouldCreateCycle(taskMap, "outside", aID)
		done <- true
	}()

	select {
	case <-done:
		// Completed in time.
	case <-time.After(2 * time.Second):
		t.Fatal("WouldCreateCycle hung on downstream cycle (infinite loop)")
	}
}

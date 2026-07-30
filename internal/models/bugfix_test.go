package models

import (
	"testing"
	"time"
)

// TestGetBlockingTasks_NilTaskNoPanic verifies the nil-task guard.
// Before the fix, GetBlockingTasks(nil) panicked while reading task.DependsOn.
func TestGetBlockingTasks_NilTaskNoPanic(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	result := p.GetBlockingTasks(nil)
	if result != nil {
		t.Errorf("expected nil for nil task, got %v", result)
	}
}

// TestGetBlockedByTasks_NilTaskNoPanic verifies the nil-task guard.
// Before the fix, GetBlockedByTasks(nil) panicked when reading task.ID
// (only visible when the project had tasks to iterate).
func TestGetBlockedByTasks_NilTaskNoPanic(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	// Add a task so the iteration is non-trivial; without the guard,
	// reading task.ID inside HasDependency would panic.
	task, err := NewGenericTask("t1", 1, "T")
	if err != nil {
		t.Fatal(err)
	}
	p.Tasks = append(p.Tasks, task)

	result := p.GetBlockedByTasks(nil)
	if result != nil {
		t.Errorf("expected nil for nil task, got %v", result)
	}
}

// TestHasIncompleteDependencies_NilTaskNoPanic verifies the nil-task guard.
// Before the fix, HasIncompleteDependencies(nil) panicked on task.DependsOn.
func TestHasIncompleteDependencies_NilTaskNoPanic(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	if p.HasIncompleteDependencies(nil) {
		t.Errorf("expected false for nil task, got true")
	}
}

// TestGetIncompleteDependencies_NilTaskNoPanic verifies the nil-task guard.
// Before the fix, GetIncompleteDependencies(nil) panicked on task.DependsOn.
func TestGetIncompleteDependencies_NilTaskNoPanic(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	result := p.GetIncompleteDependencies(nil)
	if result != nil {
		t.Errorf("expected nil for nil task, got %v", result)
	}
}

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

// TestDeferRecurrence_RejectsNonPositiveDays verifies that DeferRecurrence is a
// no-op when called with zero or negative days. The function's contract is to
// "push the due date forward"; before the fix, a negative argument silently
// rewound the due date (and a zero argument touched UpdatedAt without changing
// the date), both of which could corrupt a user's schedule via a single
// off-by-sign bug in a caller.
func TestDeferRecurrence_RejectsNonPositiveDays(t *testing.T) {
	makeRecurringTask := func() *Task {
		task, err := NewGenericTask("id", 1, "T")
		if err != nil {
			t.Fatal(err)
		}
		due := time.Date(2026, 6, 10, 0, 0, 0, 0, time.Local)
		task.DueDate = &due
		if err := task.SetRecurrence(&RecurrencePattern{Unit: RecurrenceDaily, Interval: 1}); err != nil {
			t.Fatal(err)
		}
		return task
	}

	t.Run("negative days is a no-op", func(t *testing.T) {
		task := makeRecurringTask()
		originalDue := *task.DueDate
		originalUpdatedAt := task.UpdatedAt

		task.DeferRecurrence(-5)

		if !task.DueDate.Equal(originalDue) {
			t.Errorf("expected DueDate unchanged, got %v (was %v)", *task.DueDate, originalDue)
		}
		if !task.UpdatedAt.Equal(originalUpdatedAt) {
			t.Error("expected UpdatedAt unchanged for non-positive defer")
		}
	})

	t.Run("zero days is a no-op", func(t *testing.T) {
		task := makeRecurringTask()
		originalDue := *task.DueDate
		originalUpdatedAt := task.UpdatedAt

		task.DeferRecurrence(0)

		if !task.DueDate.Equal(originalDue) {
			t.Errorf("expected DueDate unchanged, got %v (was %v)", *task.DueDate, originalDue)
		}
		if !task.UpdatedAt.Equal(originalUpdatedAt) {
			t.Error("expected UpdatedAt unchanged for non-positive defer")
		}
	})

	t.Run("positive days still works", func(t *testing.T) {
		task := makeRecurringTask()
		task.DeferRecurrence(3)
		expected := time.Date(2026, 6, 13, 0, 0, 0, 0, time.Local)
		if !task.DueDate.Equal(expected) {
			t.Errorf("expected %v, got %v", expected, *task.DueDate)
		}
	})
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

// TestHasIncompleteDependencies_DanglingContract documents and enforces the
// dangling-deps contract: a depID that points to a task that no longer
// exists (deleted, never existed) must NOT block completion. The function
// returns false for such refs (treats them as resolved).
func TestHasIncompleteDependencies_DanglingContract(t *testing.T) {
	makeProject := func(t *testing.T) *Project {
		t.Helper()
		p, err := NewProject("p", "P", "/")
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	makeTask := func(t *testing.T, id string, shortID int) *Task {
		t.Helper()
		task, err := NewGenericTask(id, shortID, "T"+id)
		if err != nil {
			t.Fatal(err)
		}
		return task
	}

	t.Run("empty deps returns false", func(t *testing.T) {
		p := makeProject(t)
		task := makeTask(t, "t1", 1)
		p.Tasks = []*Task{task}
		if p.HasIncompleteDependencies(task) {
			t.Error("empty deps must not block")
		}
	})

	t.Run("all resolved returns false", func(t *testing.T) {
		p := makeProject(t)
		b1 := makeTask(t, "b1", 1)
		b2 := makeTask(t, "b2", 2)
		task := makeTask(t, "t1", 3)
		_ = task.AddDependency(b1.ID)
		_ = task.AddDependency(b2.ID)
		b1.Complete()
		b2.Cancel()
		p.Tasks = []*Task{b1, b2, task}
		if p.HasIncompleteDependencies(task) {
			t.Error("all-resolved deps must not block (done and canceled both count as complete)")
		}
	})

	t.Run("mix of complete and incomplete returns true", func(t *testing.T) {
		p := makeProject(t)
		b1 := makeTask(t, "b1", 1)
		b2 := makeTask(t, "b2", 2)
		task := makeTask(t, "t1", 3)
		_ = task.AddDependency(b1.ID)
		_ = task.AddDependency(b2.ID)
		b1.Complete()
		// b2 still todo
		p.Tasks = []*Task{b1, b2, task}
		if !p.HasIncompleteDependencies(task) {
			t.Error("mix of complete/incomplete must block")
		}
	})

	t.Run("single dangling dep returns false", func(t *testing.T) {
		p := makeProject(t)
		task := makeTask(t, "t1", 1)
		task.DependsOn = []string{"nonexistent-id"}
		p.Tasks = []*Task{task}
		if p.HasIncompleteDependencies(task) {
			t.Error("dangling dep must NOT block (contract: dangling = resolved)")
		}
	})

	t.Run("all-dangling deps returns false", func(t *testing.T) {
		p := makeProject(t)
		task := makeTask(t, "t1", 1)
		task.DependsOn = []string{"ghost-1", "ghost-2", "ghost-3"}
		p.Tasks = []*Task{task}
		if p.HasIncompleteDependencies(task) {
			t.Error("all-dangling deps must NOT block")
		}
	})

	t.Run("dangling + resolved returns false", func(t *testing.T) {
		p := makeProject(t)
		b1 := makeTask(t, "b1", 1)
		task := makeTask(t, "t1", 2)
		_ = task.AddDependency(b1.ID)
		task.DependsOn = append(task.DependsOn, "ghost")
		b1.Complete()
		p.Tasks = []*Task{b1, task}
		if p.HasIncompleteDependencies(task) {
			t.Error("dangling + resolved must NOT block")
		}
	})

	t.Run("dangling + incomplete returns true", func(t *testing.T) {
		p := makeProject(t)
		b1 := makeTask(t, "b1", 1)
		task := makeTask(t, "t1", 2)
		_ = task.AddDependency(b1.ID)
		task.DependsOn = append(task.DependsOn, "ghost")
		// b1 still todo
		p.Tasks = []*Task{b1, task}
		if !p.HasIncompleteDependencies(task) {
			t.Error("dangling + incomplete must block (incomplete wins)")
		}
	})

	t.Run("self-reference (incomplete) returns true", func(t *testing.T) {
		// Direct slice mutation can bypass AddDependency's self-ref guard.
		// If the task depends on itself and is not yet complete, the lookup
		// finds the task, sees it's incomplete, and reports blocked. This
		// is consistent self-loop behavior.
		p := makeProject(t)
		task := makeTask(t, "t1", 1)
		task.DependsOn = []string{task.ID}
		p.Tasks = []*Task{task}
		if !p.HasIncompleteDependencies(task) {
			t.Error("incomplete self-reference must block (the task is not complete)")
		}
	})

	t.Run("self-reference (after Complete) returns false", func(t *testing.T) {
		// Once the task is marked complete, the self-reference resolves
		// (the dep target = the task = complete). No blocking.
		p := makeProject(t)
		task := makeTask(t, "t1", 1)
		task.DependsOn = []string{task.ID}
		task.Complete()
		p.Tasks = []*Task{task}
		if p.HasIncompleteDependencies(task) {
			t.Error("completed self-reference must NOT block (the task itself counts as complete)")
		}
	})
}

package models

import (
	"testing"
	"time"
)

// TestSetSection_RejectsNonGTDName verifies SetSection silently drops custom
// grouping names. Custom grouping now lives in tags, not section state.
func TestSetSection_RejectsNonGTDName(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"space", "has space"},
		{"tab", "has\ttab"},
		{"newline", "has\nnewline"},
		{"semicolon", "has;semi"},
		{"colon", "has:colon"},
		{"whitespace-only", "   "},
		{"valid hyphen", "Sprint-1"},
		{"valid camel", "BackLog"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task, err := NewTask("id", 1, TaskTypeTask, "T")
			if err != nil {
				t.Fatal(err)
			}
			task.SetSection(tc.input)
			if task.Section != "" {
				t.Errorf("expected Section empty for non-GTD name %q, got %q", tc.input, task.Section)
			}
		})
	}
}

// TestSetSection_InvalidNamePreservesPriorState ensures a rejected SetSection
// call does not wipe a previously valid section assignment.
func TestSetSection_InvalidNamePreservesPriorState(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection(SectionInbox)
	if task.Section != SectionInbox {
		t.Fatalf("precondition: expected Inbox, got %q", task.Section)
	}
	// Attempt to set an invalid name; should be a no-op leaving prior state.
	task.SetSection("bad name")
	if task.Section != SectionInbox {
		t.Errorf("expected Section unchanged after invalid SetSection, got %q", task.Section)
	}
}

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
	task, err := NewTask("t1", 1, TaskTypeTask, "T")
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

// TestSkipRecurrence_NoDueDateUsesStartOfDay verifies that skipping a
// recurring task with no DueDate produces a midnight-aligned next date
// when HasTime is false. Before the fix, the next date inherited the
// current wall-clock time, silently corrupting the implicit "no time of
// day" semantic — a later toggle of HasTime would expose garbage minutes.
func TestSkipRecurrence_NoDueDateUsesStartOfDay(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.SetRecurrence(&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}); err != nil {
		t.Fatal(err)
	}
	task.HasTime = false
	task.DueDate = nil

	task.SkipRecurrence()

	if task.DueDate == nil {
		t.Fatal("expected DueDate to be set after skip")
	}
	if task.DueDate.Hour() != 0 || task.DueDate.Minute() != 0 || task.DueDate.Second() != 0 || task.DueDate.Nanosecond() != 0 {
		t.Errorf("expected midnight-aligned next date (HasTime=false), got %v (hour=%d min=%d sec=%d nsec=%d)",
			*task.DueDate, task.DueDate.Hour(), task.DueDate.Minute(), task.DueDate.Second(), task.DueDate.Nanosecond())
	}
}

// TestSkipRecurrence_PreservesTimeOfDay verifies that when an existing
// DueDate carries a meaningful time-of-day (HasTime=true), SkipRecurrence
// preserves the clock component. This is the complementary case to the
// no-DueDate fix: we must only normalize when the user has no time set.
func TestSkipRecurrence_PreservesTimeOfDay(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	if err := task.SetRecurrence(&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}); err != nil {
		t.Fatal(err)
	}
	due := time.Date(2026, 1, 15, 9, 30, 0, 0, time.UTC)
	task.DueDate = &due
	task.HasTime = true

	task.SkipRecurrence()

	if task.DueDate == nil {
		t.Fatal("expected DueDate to be set after skip")
	}
	if task.DueDate.Hour() != 9 || task.DueDate.Minute() != 30 {
		t.Errorf("expected time-of-day preserved (9:30), got %v", *task.DueDate)
	}
}

// TestAddChild_NilChildReturnsError verifies AddChild rejects nil children.
// Before the fix, AddChild(nil) panicked on child.Type access in CanAddChild.
func TestAddChild_NilChildReturnsError(t *testing.T) {
	parent, err := NewTask("p", 1, TaskTypeEpic, "Parent")
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
	task, err := NewTask("t1", 1, TaskTypeTask, "T")
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
	a, err := NewTask("a", 1, TaskTypeTask, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTask("b", 2, TaskTypeTask, "B")
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
	a, err := NewTask("a", 1, TaskTypeTask, "A")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewTask("b", 2, TaskTypeTask, "B")
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

// TestRecurrenceValidate_RejectsWeekdaysOnNonWeekly verifies that Weekdays
// may only be set when Unit is RecurrenceWeekly. Before the fix, Validate
// accepted patterns like {Unit: monthly, Weekdays: [Mon]} even though
// calculateNextDate silently ignores Weekdays for non-weekly units —
// meaning the data round-tripped through storage indefinitely while
// having no runtime effect.
func TestRecurrenceValidate_RejectsWeekdaysOnNonWeekly(t *testing.T) {
	units := []RecurrenceUnit{RecurrenceDaily, RecurrenceMonthly, RecurrenceYearly}
	for _, u := range units {
		p := &RecurrencePattern{Interval: 1, Unit: u, Weekdays: []int{1}}
		if err := p.Validate(); err == nil {
			t.Errorf("expected error for Weekdays on unit %q, got nil", u)
		}
	}
	// Weekly with Weekdays remains valid.
	p := &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 3}}
	if err := p.Validate(); err != nil {
		t.Errorf("weekly with weekdays should validate, got %v", err)
	}
}

// TestRecurrenceValidate_RejectsDuplicateWeekdays verifies that duplicate
// entries in Weekdays are rejected. Duplicates would skew findNextWeekday's
// iteration semantics and produce confusing strings ("on Mon, Mon and Wed").
func TestRecurrenceValidate_RejectsDuplicateWeekdays(t *testing.T) {
	p := &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 1}}
	if err := p.Validate(); err == nil {
		t.Error("expected error for duplicate weekdays, got nil")
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
		task, err := NewTask("id", 1, TaskTypeTask, "T")
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
	task1, err := NewTask("a", 1, TaskTypeTask, "A")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := NewTask("b", 2, TaskTypeTask, "B")
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

// TestEnsureSection_RejectsCaseVariantGTDCollision verifies EnsureSection does
// not add a case-variant of a GTD section name (e.g. "inbox" when "Inbox"
// already exists). Before the fix, lowercase "inbox" passed validation and
// was appended alongside the canonical "Inbox", producing two sections that
// look identical to the user but are distinct in storage.
func TestEnsureSection_RejectsCaseVariantGTDCollision(t *testing.T) {
	p, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	p.EnsureDefaultSections()
	originalLen := len(p.Sections)

	for _, name := range []string{"inbox", "INBOX", "Waiting ", "someday", "review", "Standby", "COMPLETED", "NeXt"} {
		// Strip whitespace test inputs that ValidateSectionName itself rejects.
		// "Waiting " has trailing space, which the existing validator catches
		// before our case-collision check; skip those so we focus on the new
		// guard.
		if name == "Waiting " {
			continue
		}
		p.EnsureSection(name)
	}

	if len(p.Sections) != originalLen {
		t.Errorf("EnsureSection added case-variant GTD section names: got %v", p.Sections)
	}

	// Exact-case GTD names still work (idempotently).
	p.EnsureSection(SectionInbox)
	if len(p.Sections) != originalLen {
		t.Errorf("EnsureSection should be idempotent for canonical Inbox; got %v", p.Sections)
	}

	// Non-GTD section names are ignored; flexible grouping lives in tags.
	p.EnsureSection("Backlog")
	if p.HasSection("Backlog") {
		t.Error("EnsureSection should not add non-GTD sections")
	}
}

// TestSetSection_RejectsCaseVariantGTDName verifies SetSection refuses a
// case-variant of a GTD section name.
func TestSetSection_RejectsCaseVariantGTDName(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionInbox

	task.SetSection("inbox")
	// Section must remain unchanged when the call is rejected; otherwise the
	// task silently drops out of its current bucket on invalid input.
	if task.Section != SectionInbox {
		t.Errorf("Section should be preserved when SetSection rejects input, got %q", task.Section)
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
// returns false for such refs (treats them as resolved). This contract is
// the symmetric counterpart to CheckAndAutoUnblock — if either side flips,
// blocked tasks and "can I complete?" disagree.
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
		task, err := NewTask(id, shortID, TaskTypeTask, "T"+id)
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

// TestCheckAndAutoUnblock_DanglingDepDoesNotBlock verifies CheckAndAutoUnblock
// matches the HasIncompleteDependencies contract: a blocked task whose
// only outstanding dep is a dangling reference must be auto-unblocked.
// Before the fix, CheckAndAutoUnblock treated `dep == nil` as incomplete and
// left the task forever blocked, contradicting HasIncompleteDependencies which
// would happily allow completion.
func TestCheckAndAutoUnblock_DanglingDepDoesNotBlock(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("t1", 1, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	// Inject a dangling dep + blocked status directly (bypassing AddDependency
	// so we test what happens when storage delivers a stale ref).
	task.DependsOn = []string{"ghost-id"}
	task.Status = StatusBlocked
	p.Tasks = []*Task{task}

	p.CheckAndAutoUnblock()

	if task.Status != StatusTodo {
		t.Errorf("expected dangling-only dep to auto-unblock to todo, got %s", task.Status)
	}
}

// TestCheckAndAutoUnblock_DanglingMixedWithIncompleteStaysBlocked ensures
// the dangling pass-through does not accidentally unblock when a real
// incomplete blocker is still present alongside the ghost.
func TestCheckAndAutoUnblock_DanglingMixedWithIncompleteStaysBlocked(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := NewTask("b1", 1, TaskTypeTask, "Real Blocker")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("t1", 2, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	_ = task.AddDependency(blocker.ID)
	task.DependsOn = append(task.DependsOn, "ghost-id")
	task.Status = StatusBlocked
	p.Tasks = []*Task{blocker, task}

	p.CheckAndAutoUnblock()

	if task.Status != StatusBlocked {
		t.Errorf("expected task to remain blocked while real blocker is incomplete, got %s", task.Status)
	}
}

// TestCheckAndAutoUnblock_DanglingPlusResolvedUnblocks verifies that with
// one dangling ref and one resolved (completed) blocker, the task is
// unblocked — the dangling ref must not contribute an "incomplete" vote.
func TestCheckAndAutoUnblock_DanglingPlusResolvedUnblocks(t *testing.T) {
	p, err := NewProject("p", "P", "/")
	if err != nil {
		t.Fatal(err)
	}
	blocker, err := NewTask("b1", 1, TaskTypeTask, "Real Blocker")
	if err != nil {
		t.Fatal(err)
	}
	task, err := NewTask("t1", 2, TaskTypeTask, "Blocked Task")
	if err != nil {
		t.Fatal(err)
	}
	_ = task.AddDependency(blocker.ID)
	task.DependsOn = append(task.DependsOn, "ghost-id")
	task.Status = StatusBlocked
	p.Tasks = []*Task{blocker, task}

	blocker.Complete()
	p.CheckAndAutoUnblock()

	if task.Status != StatusTodo {
		t.Errorf("expected task to be unblocked (resolved blocker + dangling ref), got %s", task.Status)
	}
}

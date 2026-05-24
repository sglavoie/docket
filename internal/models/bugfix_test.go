package models

import (
	"testing"
	"time"
)

// TestSetSection_RejectsInvalidCustomName verifies SetSection silently drops
// custom section names that fail ValidateSectionName. Before the fix,
// SetSection accepted any non-GTD string, allowing names containing spaces,
// semicolons, or colons to land in CustomSection — which then corrupt the
// markdown round-trip (";" and ":" delimit section-color metadata).
func TestSetSection_RejectsInvalidCustomName(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantClear bool // when true, CustomSection must remain empty
	}{
		{"space", "has space", true},
		{"tab", "has\ttab", true},
		{"newline", "has\nnewline", true},
		{"semicolon", "has;semi", true},
		{"colon", "has:colon", true},
		{"whitespace-only", "   ", true},
		{"valid hyphen", "Sprint-1", false},
		{"valid camel", "BackLog", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task, err := NewTask("id", 1, TaskTypeTask, "T")
			if err != nil {
				t.Fatal(err)
			}
			task.SetSection(tc.input)
			if tc.wantClear {
				if task.CustomSection != "" {
					t.Errorf("expected CustomSection empty for invalid name %q, got %q", tc.input, task.CustomSection)
				}
				if task.Section != "" {
					t.Errorf("expected Section empty for invalid name %q, got %q", tc.input, task.Section)
				}
			} else {
				if task.CustomSection != tc.input {
					t.Errorf("expected CustomSection=%q, got %q", tc.input, task.CustomSection)
				}
			}
		})
	}
}

// TestSetSection_InvalidNamePreservesPriorState ensures a rejected SetSection
// call does not wipe a previously valid Section/CustomSection assignment.
func TestSetSection_InvalidNamePreservesPriorState(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.SetSection("Sprint-1")
	if task.CustomSection != "Sprint-1" {
		t.Fatalf("precondition: expected Sprint-1, got %q", task.CustomSection)
	}
	// Attempt to set an invalid name; should be a no-op leaving prior state.
	task.SetSection("bad name")
	if task.CustomSection != "Sprint-1" {
		t.Errorf("expected CustomSection unchanged after invalid SetSection, got %q", task.CustomSection)
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

	// Non-GTD custom sections still work.
	p.EnsureSection("Backlog")
	if !p.HasSection("Backlog") {
		t.Error("EnsureSection should still add non-colliding custom section")
	}
}

// TestRenameSection_RejectsCaseVariantGTDCollision verifies RenameSection
// refuses to rename a custom section to a case-variant of a GTD section name.
// Before the fix, renaming "MyStuff" to "inbox" would succeed despite the
// canonical "Inbox" GTD section already existing.
func TestRenameSection_RejectsCaseVariantGTDCollision(t *testing.T) {
	p, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}
	p.EnsureDefaultSections()
	p.EnsureSection("MyStuff")

	err = p.RenameSection("MyStuff", "inbox")
	if err == nil {
		t.Fatal("expected error renaming custom section to case-variant of GTD name, got nil")
	}
	if !p.HasSection("MyStuff") {
		t.Error("MyStuff section should remain unchanged after rejected rename")
	}
	if p.HasSection("inbox") {
		t.Error("case-variant 'inbox' should not have been added to Sections")
	}
}

// TestSetSection_RejectsCaseVariantGTDName verifies SetSection refuses to land
// a case-variant of a GTD section name in CustomSection. Before the fix, a
// task could carry CustomSection="inbox" while the project's GTD "Inbox"
// section also existed, scattering tasks across two visually identical
// buckets.
func TestSetSection_RejectsCaseVariantGTDName(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.Section = SectionInbox

	task.SetSection("inbox")
	if task.CustomSection != "" {
		t.Errorf("SetSection should not store case-variant GTD name in CustomSection, got %q", task.CustomSection)
	}
	// Section must remain unchanged when the call is rejected; otherwise the
	// task silently drops out of its current bucket on invalid input.
	if task.Section != SectionInbox {
		t.Errorf("Section should be preserved when SetSection rejects input, got %q", task.Section)
	}
}

// TestSetCustomSection_RejectsCaseVariantGTDName verifies SetCustomSection
// refuses to store a case-variant of a GTD section name. Before the fix,
// only the exact-case GTD names were rejected, allowing "inbox" or "next"
// to bypass the check.
func TestSetCustomSection_RejectsCaseVariantGTDName(t *testing.T) {
	task, err := NewTask("id", 1, TaskTypeTask, "T")
	if err != nil {
		t.Fatal(err)
	}
	task.SetCustomSection("inbox")
	if task.CustomSection != "" {
		t.Errorf("SetCustomSection should reject case-variant GTD name, got %q", task.CustomSection)
	}
	task.SetCustomSection("NEXT")
	if task.CustomSection != "" {
		t.Errorf("SetCustomSection should reject 'NEXT' (case-variant of GTD Next), got %q", task.CustomSection)
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

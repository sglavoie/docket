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

package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCloneRecurrenceDeepCopiesWeekdays guards against an aliasing bug where
// the cloned RecurrencePattern shared its Weekdays slice with the original.
// Mutating the clone (e.g. via the task form draft) must not leak back into
// the source task's recurrence pattern.
func TestCloneRecurrenceDeepCopiesWeekdays(t *testing.T) {
	src := &models.RecurrencePattern{
		Interval: 2,
		Unit:     models.RecurrenceWeekly,
		Weekdays: []int{1, 3, 5},
	}

	clone := cloneRecurrence(src)
	if clone == nil {
		t.Fatal("cloneRecurrence returned nil for non-nil input")
	}
	if clone == src {
		t.Fatal("cloneRecurrence must return a new pointer")
	}

	// Mutate the clone's Weekdays. With a shallow copy this would overwrite
	// src.Weekdays[0] because both slices share the same backing array.
	if len(clone.Weekdays) > 0 {
		clone.Weekdays[0] = 99
	}

	if src.Weekdays[0] != 1 {
		t.Errorf("source Weekdays was mutated via the clone: got %d, want 1", src.Weekdays[0])
	}

	// Appending past the original length must not stomp into src.Weekdays.
	src.Weekdays = src.Weekdays[:cap(src.Weekdays)]
	clone.Weekdays = append(clone.Weekdays, 6)
	if len(src.Weekdays) > 0 && src.Weekdays[len(src.Weekdays)-1] == 6 {
		t.Errorf("source Weekdays grew due to shared backing array")
	}
}

// TestCloneRecurrenceNilSafe verifies cloneRecurrence returns nil for nil input.
func TestCloneRecurrenceNilSafe(t *testing.T) {
	if cloneRecurrence(nil) != nil {
		t.Error("cloneRecurrence(nil) should return nil")
	}
}

// TestCloneRecurrenceEmptyWeekdays verifies clones without weekday data still
// return a fresh struct with no Weekdays slice attached.
func TestCloneRecurrenceEmptyWeekdays(t *testing.T) {
	src := &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceDaily,
	}
	clone := cloneRecurrence(src)
	if clone == nil {
		t.Fatal("cloneRecurrence returned nil for non-nil input")
	}
	if len(clone.Weekdays) != 0 {
		t.Errorf("clone.Weekdays should be empty for empty source, got %v", clone.Weekdays)
	}
	if clone.Interval != src.Interval || clone.Unit != src.Unit {
		t.Errorf("clone fields do not match source: got Interval=%d, Unit=%s; want Interval=%d, Unit=%s",
			clone.Interval, clone.Unit, src.Interval, src.Unit)
	}
}

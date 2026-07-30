package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCloneRecurrenceReturnsIndependentCanonicalPattern(t *testing.T) {
	src := &models.RecurrencePattern{
		Interval: 2,
		Unit:     models.RecurrenceWeekly,
	}

	clone := cloneRecurrence(src)
	if clone == nil {
		t.Fatal("cloneRecurrence returned nil for non-nil input")
	}
	if clone == src {
		t.Fatal("cloneRecurrence must return a new pointer")
	}

	clone.Interval = 9
	if src.Interval != 2 {
		t.Errorf("source Interval was mutated via the clone: got %d, want 2", src.Interval)
	}
}

// TestCloneRecurrenceNilSafe verifies cloneRecurrence returns nil for nil input.
func TestCloneRecurrenceNilSafe(t *testing.T) {
	if cloneRecurrence(nil) != nil {
		t.Error("cloneRecurrence(nil) should return nil")
	}
}

func TestCloneRecurrencePreservesCanonicalFields(t *testing.T) {
	src := &models.RecurrencePattern{
		Interval: 1,
		Unit:     models.RecurrenceDaily,
	}
	clone := cloneRecurrence(src)
	if clone == nil {
		t.Fatal("cloneRecurrence returned nil for non-nil input")
	}
	if clone.Interval != src.Interval || clone.Unit != src.Unit {
		t.Errorf("clone fields do not match source: got Interval=%d, Unit=%s; want Interval=%d, Unit=%s",
			clone.Interval, clone.Unit, src.Interval, src.Unit)
	}
}

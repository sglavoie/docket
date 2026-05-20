package planner

import (
	"math"
	"testing"
	"time"
)

// TestDaysToDuration_OverflowClamps guards against int64 overflow in
// time.Duration arithmetic when astronomically large day counts are passed
// (e.g. velocity is microscopic and remainingDays is huge). Previously the
// implementation would silently wrap to negative durations, producing past
// dates from forward-looking schedules.
func TestDaysToDuration_OverflowClamps(t *testing.T) {
	// math.MaxFloat64 days would overflow int64 nanoseconds many times over.
	d := daysToduration(math.MaxFloat64)
	if d <= 0 {
		t.Errorf("expected positive duration on overflow, got %v", d)
	}
	// Adding it to time.Now() must remain after now.
	now := time.Now()
	if !now.Add(d).After(now) {
		t.Errorf("expected now+d to be after now; got d=%v", d)
	}
}

// TestDaysToDuration_NegativeOverflowClamps ensures negative day counts that
// would overflow are clamped to a finite minimum.
func TestDaysToDuration_NegativeOverflowClamps(t *testing.T) {
	d := daysToduration(-math.MaxFloat64)
	if d >= 0 {
		t.Errorf("expected negative duration on negative overflow, got %v", d)
	}
}

// TestDaysToDuration_NormalRange verifies typical inputs work unchanged.
func TestDaysToDuration_NormalRange(t *testing.T) {
	d := daysToduration(2)
	want := 48 * time.Hour
	if d != want {
		t.Errorf("expected %v, got %v", want, d)
	}
}

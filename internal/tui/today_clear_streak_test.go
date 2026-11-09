package tui

import (
	"testing"
	"time"
)

// Clearing Today names the run of days that finished something once it is
// longer than today alone, and says nothing more on a first day.
func TestTodayClearNoteNamesAStreak(t *testing.T) {
	now := time.Date(2026, 11, 8, 17, 0, 0, 0, time.Local)
	day := func(offset int) time.Time { return time.Date(2026, 11, 8+offset, 0, 0, 0, 0, time.Local) }
	if got := todayClearWithStreak(map[time.Time]int{day(0): 1}, now); got != todayClearNote {
		t.Fatalf("first day = %q, want %q", got, todayClearNote)
	}
	history := map[time.Time]int{day(0): 2, day(-1): 1, day(-2): 3}
	if got, want := todayClearWithStreak(history, now), "Today is clear · 3-day streak"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

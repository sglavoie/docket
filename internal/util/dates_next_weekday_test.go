package util

import (
	"testing"
	"time"
)

// NextWeekdayAfter never answers today: the weekend the date picker offers on a
// Saturday or a Sunday is the next one, and Monday on a Monday is a week away.
func TestNextWeekdayAfterSkipsToday(t *testing.T) {
	// 2026-10-12 is a Monday.
	monday := time.Date(2026, 10, 12, 15, 4, 5, 0, time.Local)
	cases := []struct {
		offset      int
		wantSat     string
		wantMonday  string
		description string
	}{
		{0, "2026-10-17", "2026-10-19", "Monday"},
		{3, "2026-10-17", "2026-10-19", "Thursday"},
		{4, "2026-10-17", "2026-10-19", "Friday"},
		{5, "2026-10-24", "2026-10-19", "Saturday"},
		{6, "2026-10-24", "2026-10-19", "Sunday"},
	}
	for _, testCase := range cases {
		now := monday.AddDate(0, 0, testCase.offset)
		sat := NextWeekdayAfter(now, time.Saturday)
		mon := NextWeekdayAfter(now, time.Monday)
		if got := sat.Format(time.DateOnly); got != testCase.wantSat || !sat.Equal(StartOfDay(sat)) {
			t.Errorf("%s: next Saturday = %s, want %s at midnight", testCase.description, sat, testCase.wantSat)
		}
		if got := mon.Format(time.DateOnly); got != testCase.wantMonday || !mon.Equal(StartOfDay(mon)) {
			t.Errorf("%s: next Monday = %s, want %s at midnight", testCase.description, mon, testCase.wantMonday)
		}
	}
}

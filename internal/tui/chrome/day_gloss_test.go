package chrome

import (
	"testing"
	"time"
)

func TestDayGlossNamesDistance(t *testing.T) {
	now := time.Date(2026, 9, 26, 15, 0, 0, 0, time.UTC) // a Saturday
	day := func(offset int) time.Time { return time.Date(2026, 9, 26+offset, 0, 0, 0, 0, time.UTC) }
	for offset, want := range map[int]string{
		0:  "today",
		1:  "tomorrow",
		-1: "yesterday",
		6:  "in 6 days",
		-3: "3 days ago",
	} {
		if got := DayGloss(day(offset), now); got != want {
			t.Errorf("DayGloss(%+d) = %q, want %q", offset, got, want)
		}
	}
}

func TestFormatWeekdayDateAddsYearAndClockOnlyWhenOwed(t *testing.T) {
	now := time.Date(2026, 12, 31, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name  string
		value time.Time
		timed bool
		want  string
	}{
		{"date only", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), false, "Fri Jan 2"},
		{"timed", time.Date(2026, 12, 31, 15, 30, 0, 0, time.UTC), true, "Thu Dec 31 15:30"},
		{"timed at midnight", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC), true, "Thu Dec 31 00:00"},
		{"clock ignored when not timed", time.Date(2026, 12, 31, 15, 30, 0, 0, time.UTC), false, "Thu Dec 31"},
		{"next year", time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), false, "Fri Jan 1 2027"},
		{"previous year timed", time.Date(2025, 12, 31, 9, 5, 0, 0, time.UTC), true, "Wed Dec 31 2025 09:05"},
	} {
		if got := FormatWeekdayDate(tc.value, now, tc.timed); got != tc.want {
			t.Errorf("%s: FormatWeekdayDate = %q, want %q", tc.name, got, tc.want)
		}
	}
}

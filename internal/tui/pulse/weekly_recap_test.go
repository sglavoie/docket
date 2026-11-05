package pulse

import (
	"testing"
	"time"
)

func TestWeekStartIsTheMondayAtMidnight(t *testing.T) {
	cases := map[string]string{
		"2026-11-02T09:00:00": "2026-11-02", // Monday
		"2026-11-04T23:59:00": "2026-11-02", // Wednesday
		"2026-11-08T00:30:00": "2026-11-02", // Sunday
	}
	for in, want := range cases {
		at, _ := time.ParseInLocation("2006-01-02T15:04:05", in, time.Local)
		if got := WeekStart(at).Format(time.DateOnly); got != want {
			t.Errorf("WeekStart(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestOpensNewWeek(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local) // Wednesday
	for greeted, want := range map[string]bool{
		"":           true,
		"garbage":    true,
		"2026-11-01": true,  // the Sunday before
		"2026-11-02": false, // this Monday
		"2026-11-03": false,
	} {
		if got := OpensNewWeek(greeted, now); got != want {
			t.Errorf("OpensNewWeek(%q) = %v, want %v", greeted, got, want)
		}
	}
}

func TestWeeklyRecapCountsThePreviousWeekAndNamesItsBusiestDay(t *testing.T) {
	now := time.Date(2026, 11, 4, 9, 0, 0, 0, time.Local) // week of Mon Nov 2
	day := func(d int) time.Time { return time.Date(2026, 10, d, 0, 0, 0, 0, time.Local) }
	daily := map[time.Time]int{
		day(25): 9, // Sunday before last week: excluded
		day(26): 2, // Monday
		day(27): 5, // Tuesday
		day(30): 1, // Friday
		time.Date(2026, 11, 1, 0, 0, 0, 0, time.Local): 1, // Sunday: last day of last week
		time.Date(2026, 11, 2, 0, 0, 0, 0, time.Local): 4, // this week: excluded
	}
	if got, want := WeeklyRecap(daily, now), "last week 9 done, most on Tuesday"; got != want {
		t.Fatalf("WeeklyRecap = %q, want %q", got, want)
	}
	if got, want := WeeklyRecap(map[time.Time]int{day(28): 3}, now), "last week 3 done"; got != want {
		t.Fatalf("one active day names no busiest day: got %q, want %q", got, want)
	}
	if got := WeeklyRecap(map[time.Time]int{day(25): 3}, now); got != "" {
		t.Fatalf("a week with nothing Done must say nothing, got %q", got)
	}
}

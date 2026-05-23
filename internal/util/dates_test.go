package util

import (
	"strings"
	"testing"
	"time"
)

func TestParseRelativeDate(t *testing.T) {
	today := StartOfDay(time.Now())

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{"today", "today", today, false},
		{"tomorrow", "tomorrow", today.AddDate(0, 0, 1), false},
		{"yesterday", "yesterday", today.AddDate(0, 0, -1), false},
		{"next week is today+7", "next week", today.AddDate(0, 0, 7), false},
		{"next month is same day next month", "next month", AddMonthsClamped(today, 1), false},
		{"relative +3d", "+3d", today.AddDate(0, 0, 3), false},
		{"relative +2w", "+2w", today.AddDate(0, 0, 14), false},
		{"relative +1m", "+1m", AddMonthsClamped(today, 1), false},
		{"in 5 days", "in 5 days", today.AddDate(0, 0, 5), false},
		{"in 2 weeks", "in 2 weeks", today.AddDate(0, 0, 14), false},
		{"in 3 months", "in 3 months", AddMonthsClamped(today, 3), false},
		{"exact date", "2025-06-15", time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local), false},
		{"invalid", "gibberish", time.Time{}, true},
		// eod
		{"eod is today", "eod", today, false},
		// eom
		{"eom is last day of month", "eom", time.Date(today.Year(), today.Month()+1, 0, 0, 0, 0, 0, today.Location()), false},
		// eoy
		{"eoy is dec 31", "eoy", time.Date(today.Year(), 12, 31, 0, 0, 0, 0, today.Location()), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRelativeDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRelativeDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseRelativeDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRelativeDateEow(t *testing.T) {
	today := StartOfDay(time.Now())

	eowFor := func(weekEndDay string, wd time.Weekday) time.Time {
		daysUntil := int(wd) - int(today.Weekday())
		if daysUntil < 0 {
			daysUntil += 7
		}
		return today.AddDate(0, 0, daysUntil)
	}

	tests := []struct {
		name       string
		weekEndDay string
		want       time.Time
	}{
		{"eow sun (default)", "sun", eowFor("sun", time.Sunday)},
		{"eow sat", "sat", eowFor("sat", time.Saturday)},
		{"eow fri", "fri", eowFor("fri", time.Friday)},
		{"eow empty defaults to sun", "", eowFor("", time.Sunday)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRelativeDateWithConfig("eow", tt.weekEndDay)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(tt.want) {
				t.Errorf("eow with weekEndDay=%q = %v, want %v", tt.weekEndDay, got, tt.want)
			}
		})
	}
}

func TestParseRelativeDateThisWeekday(t *testing.T) {
	today := StartOfDay(time.Now())
	todayISO := isoWeekdayNum(today.Weekday())

	// "this <today>" → today
	todayName := strings.ToLower(today.Weekday().String()[:3])
	t.Run("this today's weekday = today", func(t *testing.T) {
		got, err := ParseRelativeDate("this " + todayName)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !got.Equal(today) {
			t.Errorf("this %s = %v, want %v", todayName, got, today)
		}
	})

	// "this <a future weekday this ISO week>" → today + N
	// Pick a day that is 1 ISO step ahead; if today is Sunday (iso=7) there is no future day → skip.
	if todayISO < 7 {
		nextWD := time.Weekday((int(today.Weekday()) % 7) + 1) // next Go weekday (may wrap Sun→Mon)
		// Recalculate to ensure it's truly next in ISO order
		nextISO := todayISO + 1
		// Map nextISO (1..7) back to time.Weekday: iso 7 = Sunday(0), else iso = int(wd)
		var targetWD time.Weekday
		if nextISO == 7 {
			targetWD = time.Sunday
		} else {
			targetWD = time.Weekday(nextISO)
		}
		_ = nextWD
		targetName := strings.ToLower(targetWD.String()[:3])
		t.Run("this next-ISO-weekday = today+1", func(t *testing.T) {
			got, err := ParseRelativeDate("this " + targetName)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !got.Equal(today.AddDate(0, 0, 1)) {
				t.Errorf("this %s = %v, want %v", targetName, got, today.AddDate(0, 0, 1))
			}
		})
	}

	// "this <a past weekday this ISO week>" → error
	// Pick a day that is 1 ISO step before; if today is Monday (iso=1) there is no past day → skip.
	if todayISO > 1 {
		prevISO := todayISO - 1
		var prevWD time.Weekday
		if prevISO == 7 {
			prevWD = time.Sunday
		} else {
			prevWD = time.Weekday(prevISO)
		}
		prevName := strings.ToLower(prevWD.String()[:3])
		t.Run("this past-ISO-weekday = error", func(t *testing.T) {
			_, err := ParseRelativeDate("this " + prevName)
			if err == nil {
				t.Errorf("expected error for 'this %s' (past in ISO week)", prevName)
			}
			if err != nil && !strings.Contains(err.Error(), "in the past") {
				t.Errorf("expected 'in the past' in error, got: %v", err)
			}
		})
	}
}

func TestParseDateWithTime(t *testing.T) {
	today := StartOfDay(time.Now())

	at := func(base time.Time, h, m int) time.Time {
		return time.Date(base.Year(), base.Month(), base.Day(), h, m, 0, 0, base.Location())
	}

	tests := []struct {
		name     string
		input    string
		wantTime bool
		wantT    time.Time
		wantErr  bool
	}{
		// Pure time tokens (date = today)
		{"5pm", "5pm", true, at(today, 17, 0), false},
		{"9am", "9am", true, at(today, 9, 0), false},
		{"5:30pm", "5:30pm", true, at(today, 17, 30), false},
		{"17:00", "17:00", true, at(today, 17, 0), false},
		{"08:30", "08:30", true, at(today, 8, 30), false},
		{"12am is midnight", "12am", true, at(today, 0, 0), false},
		{"12pm is noon", "12pm", true, at(today, 12, 0), false},
		// Combined date + time
		{"today 5pm", "today 5pm", true, at(today, 17, 0), false},
		{"tomorrow 9am", "tomorrow 9am", true, at(today.AddDate(0, 0, 1), 9, 0), false},
		{"eod 23:59", "eod 23:59", true, at(today, 23, 59), false},
		{"+2d 14:30", "+2d 14:30", true, at(today.AddDate(0, 0, 2), 14, 30), false},
		// No time suffix → hasTime=false
		{"today alone", "today", false, today, false},
		{"tomorrow alone", "tomorrow", false, today.AddDate(0, 0, 1), false},
		// Invalid time values
		{"25:00 invalid hour", "25:00", false, time.Time{}, true},
		{"13pm invalid", "13pm", false, time.Time{}, true},
		{"0am invalid", "0am", false, time.Time{}, true},
		// Invalid date part with a valid time suffix
		{"gibberish date with time", "gibberish 5pm", false, time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, hasTime, err := ParseRelativeDateWithTime(tt.input, "sun")
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseRelativeDateWithTime(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if hasTime != tt.wantTime {
				t.Errorf("hasTime = %v, want %v", hasTime, tt.wantTime)
			}
			if !got.Equal(tt.wantT) {
				t.Errorf("got %v, want %v", got, tt.wantT)
			}
		})
	}
}

func TestParseNaturalLanguageDates(t *testing.T) {
	today := StartOfDay(time.Now())

	nextOccurrence := func(m time.Month, day int) time.Time {
		candidate := time.Date(today.Year(), m, day, 0, 0, 0, 0, today.Location())
		if !candidate.After(today) {
			candidate = time.Date(today.Year()+1, m, day, 0, 0, 0, 0, today.Location())
		}
		return candidate
	}

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		{"dec 15", "dec 15", nextOccurrence(time.December, 15), false},
		{"jan 3", "jan 3", nextOccurrence(time.January, 3), false},
		{"15 dec", "15 dec", nextOccurrence(time.December, 15), false},
		{"3 jan", "3 jan", nextOccurrence(time.January, 3), false},
		{"sep 1", "sep 1", nextOccurrence(time.September, 1), false},
		{"feb 30 is invalid", "feb 30", time.Time{}, true},
		{"feb 0 is invalid", "feb 0", time.Time{}, true},
		{"next year", "next year", AddMonthsClamped(today, 12), false},
		{"in 1 year", "in 1 year", AddMonthsClamped(today, 12), false},
		{"in 3 years", "in 3 years", AddMonthsClamped(today, 36), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRelativeDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRelativeDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseRelativeDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestAddMonthsClampsEndOfMonth(t *testing.T) {
	// Standard Go AddDate(0, 1, 0) on Jan 31 silently overflows to Mar 3
	// (Feb has only 28 days). When parsing a user-supplied "+Nm" or
	// "in N months" offset, the expected behavior is that the result lands
	// in the next month rather than skipping ahead, so we clamp to the
	// target month's last day.
	tests := []struct {
		name   string
		base   time.Time
		months int
		want   time.Time
	}{
		{"jan 31 + 1m clamps to feb 28 in 2025", time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC), 1, time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)},
		{"jan 31 + 1m clamps to feb 29 in leap year 2024", time.Date(2024, 1, 31, 0, 0, 0, 0, time.UTC), 1, time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)},
		{"may 31 + 1m clamps to jun 30", time.Date(2025, 5, 31, 0, 0, 0, 0, time.UTC), 1, time.Date(2025, 6, 30, 0, 0, 0, 0, time.UTC)},
		{"jan 30 + 1m clamps to feb 28 in 2025", time.Date(2025, 1, 30, 0, 0, 0, 0, time.UTC), 1, time.Date(2025, 2, 28, 0, 0, 0, 0, time.UTC)},
		{"jan 15 + 1m stays jan 15->feb 15", time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), 1, time.Date(2025, 2, 15, 0, 0, 0, 0, time.UTC)},
		{"dec 31 + 1m crosses year to jan 31", time.Date(2024, 12, 31, 0, 0, 0, 0, time.UTC), 1, time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := AddMonthsClamped(tc.base, tc.months)
			if !got.Equal(tc.want) {
				t.Errorf("AddMonthsClamped(%v, %d) = %v, want %v", tc.base, tc.months, got, tc.want)
			}
		})
	}
}

func TestDaysBetween(t *testing.T) {
	a := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	b := time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC)
	if got := DaysBetween(a, b); got != 9 {
		t.Errorf("DaysBetween(Jan 1, Jan 10) = %d, want 9", got)
	}
	if got := DaysBetween(b, a); got != -9 {
		t.Errorf("DaysBetween(Jan 10, Jan 1) = %d, want -9", got)
	}
	if got := DaysBetween(a, a); got != 0 {
		t.Errorf("DaysBetween(same day) = %d, want 0", got)
	}
}

func TestParseFlexibleDate(t *testing.T) {
	today := StartOfDay(time.Now())

	tests := []struct {
		name    string
		input   string
		want    time.Time
		wantErr bool
	}{
		// Bare shorthand (new in ParseFlexibleDate)
		{"3d = 3 days", "3d", today.AddDate(0, 0, 3), false},
		{"7d = 7 days", "7d", today.AddDate(0, 0, 7), false},
		{"2w = 2 weeks", "2w", today.AddDate(0, 0, 14), false},
		{"1m = 1 month", "1m", AddMonthsClamped(today, 1), false},
		// Standard inputs still work
		{"tomorrow", "tomorrow", today.AddDate(0, 0, 1), false},
		{"today", "today", today, false},
		{"+3d", "+3d", today.AddDate(0, 0, 3), false},
		{"+2w", "+2w", today.AddDate(0, 0, 14), false},
		{"in 5 days", "in 5 days", today.AddDate(0, 0, 5), false},
		// Invalid
		{"gibberish", "gibberish", time.Time{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseFlexibleDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseFlexibleDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && !got.Equal(tt.want) {
				t.Errorf("ParseFlexibleDate(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

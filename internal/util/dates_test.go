package util

import (
	"testing"
	"time"
)

func TestFormatDate(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"standard date", time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC), "2024-03-15"},
		{"new year", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "2024-01-01"},
		{"end of year", time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC), "2024-12-31"},
		{"single digit month/day", time.Date(2024, 5, 7, 0, 0, 0, 0, time.UTC), "2024-05-07"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDate(tt.time)
			if result != tt.expected {
				t.Errorf("FormatDate(%v) = %q, want %q", tt.time, result, tt.expected)
			}
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		name     string
		time     time.Time
		expected string
	}{
		{"standard datetime", time.Date(2024, 3, 15, 10, 30, 0, 0, time.UTC), "2024-03-15 10:30"},
		{"midnight", time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), "2024-01-01 00:00"},
		{"end of day", time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC), "2024-12-31 23:59"},
		{"single digit hour", time.Date(2024, 5, 7, 9, 5, 0, 0, time.UTC), "2024-05-07 09:05"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDateTime(tt.time)
			if result != tt.expected {
				t.Errorf("FormatDateTime(%v) = %q, want %q", tt.time, result, tt.expected)
			}
		})
	}
}

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
		{"next month is same day next month", "next month", today.AddDate(0, 1, 0), false},
		{"relative +3d", "+3d", today.AddDate(0, 0, 3), false},
		{"relative +2w", "+2w", today.AddDate(0, 0, 14), false},
		{"relative +1m", "+1m", today.AddDate(0, 1, 0), false},
		{"in 5 days", "in 5 days", today.AddDate(0, 0, 5), false},
		{"in 2 weeks", "in 2 weeks", today.AddDate(0, 0, 14), false},
		{"in 3 months", "in 3 months", today.AddDate(0, 3, 0), false},
		{"exact date", "2025-06-15", time.Date(2025, 6, 15, 0, 0, 0, 0, time.Local), false},
		{"invalid", "gibberish", time.Time{}, true},
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

func TestParseDate(t *testing.T) {
	tests := []struct {
		name        string
		value       string
		expectOk    bool
		expectYear  int
		expectMonth time.Month
		expectDay   int
		expectHour  int
		expectMin   int
	}{
		{"date only", "2024-03-15", true, 2024, 3, 15, 0, 0},
		{"datetime", "2024-03-15 10:30", true, 2024, 3, 15, 10, 30},
		{"midnight datetime", "2024-01-01 00:00", true, 2024, 1, 1, 0, 0},
		{"end of day", "2024-12-31 23:59", true, 2024, 12, 31, 23, 59},
		{"invalid format", "15-03-2024", false, 0, 0, 0, 0, 0},
		{"invalid date", "2024-13-45", false, 0, 0, 0, 0, 0},
		{"empty string", "", false, 0, 0, 0, 0, 0},
		{"partial date", "2024-03", false, 0, 0, 0, 0, 0},
		{"with seconds", "2024-03-15 10:30:00", false, 0, 0, 0, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, ok := ParseDate(tt.value)
			if ok != tt.expectOk {
				t.Errorf("ParseDate(%q) ok = %v, want %v", tt.value, ok, tt.expectOk)
				return
			}
			if !tt.expectOk {
				return
			}
			if result.Year() != tt.expectYear {
				t.Errorf("ParseDate(%q) year = %d, want %d", tt.value, result.Year(), tt.expectYear)
			}
			if result.Month() != tt.expectMonth {
				t.Errorf("ParseDate(%q) month = %d, want %d", tt.value, result.Month(), tt.expectMonth)
			}
			if result.Day() != tt.expectDay {
				t.Errorf("ParseDate(%q) day = %d, want %d", tt.value, result.Day(), tt.expectDay)
			}
			if result.Hour() != tt.expectHour {
				t.Errorf("ParseDate(%q) hour = %d, want %d", tt.value, result.Hour(), tt.expectHour)
			}
			if result.Minute() != tt.expectMin {
				t.Errorf("ParseDate(%q) minute = %d, want %d", tt.value, result.Minute(), tt.expectMin)
			}
		})
	}
}

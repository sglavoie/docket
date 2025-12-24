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

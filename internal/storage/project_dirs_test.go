package storage

import (
	"testing"
	"time"
)

func TestFormatRelativeTimeSubDay(t *testing.T) {
	// Sub-day labels are driven by real elapsed time, so raw durations are
	// deterministic regardless of the wall-clock time of day.
	tests := []struct {
		name     string
		age      time.Duration
		expected string
	}{
		{"just now: 30s", 30 * time.Second, "just now"},
		{"just now: 59s", 59 * time.Second, "just now"},
		{"1 minute ago: 60s", 60 * time.Second, "1 minute ago"},
		{"2 minutes ago: 2m", 2 * time.Minute, "2 minutes ago"},
		{"1 hour ago", 60 * time.Minute, "1 hour ago"},
		{"2 hours ago", 2 * time.Hour, "2 hours ago"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			past := time.Now().Add(-tc.age)
			got := FormatRelativeTime(past)
			if got != tc.expected {
				t.Errorf("FormatRelativeTime(%v ago) = %q, want %q", tc.age, got, tc.expected)
			}
		})
	}
}

func TestFormatRelativeTimeCalendarDayBoundary(t *testing.T) {
	// FormatRelativeTime labels day-scale ages by calendar day, not by raw
	// hours, so a fixed hour offset like "25h ago" is "yesterday" or "2 days
	// ago" depending on the current time of day. Anchor these cases with
	// AddDate so the expected calendar label is deterministic and this test
	// establishes a nonflaky baseline instead of failing near midnight.
	tests := []struct {
		name     string
		daysAgo  int
		expected string
	}{
		{"yesterday", 1, "yesterday"},
		{"2 days ago", 2, "2 days ago"},
		{"3 days ago", 3, "3 days ago"},
		{"7 days ago", 7, "7 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			past := time.Now().AddDate(0, 0, -tc.daysAgo)
			got := FormatRelativeTime(past)
			if got != tc.expected {
				t.Errorf("FormatRelativeTime(%d calendar days ago) = %q, want %q", tc.daysAgo, got, tc.expected)
			}
		})
	}
}

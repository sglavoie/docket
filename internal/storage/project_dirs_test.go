package storage

import (
	"testing"
	"time"
)

func TestFormatRelativeTimeCalendarDayBoundary(t *testing.T) {
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
		{"yesterday: 25h", 25 * time.Hour, "yesterday"},
		// Exactly 47h59m is calendar "yesterday" only when now and then
		// share the same day-boundary relationship, but FormatRelativeTime
		// must use calendar-day comparison so that 47h ago is NOT "yesterday"
		// — it is 2 calendar days ago.
		{"2 days ago: 47h", 47 * time.Hour, "2 days ago"},
		{"2 days ago: 48h", 48 * time.Hour, "2 days ago"},
		{"3 days ago: 72h", 72 * time.Hour, "3 days ago"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			past := now.Add(-tc.age)
			got := FormatRelativeTime(past)
			if got != tc.expected {
				t.Errorf("FormatRelativeTime(%v ago) = %q, want %q", tc.age, got, tc.expected)
			}
		})
	}
}

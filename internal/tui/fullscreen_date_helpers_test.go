package tui

import (
	"strings"
	"testing"
	"time"
)

func TestFormatRelativeDueVerbose(t *testing.T) {
	now := time.Now()

	makeDate := func(daysOffset int) *time.Time {
		d := now.AddDate(0, 0, daysOffset)
		return &d
	}

	tests := []struct {
		name      string
		dueDate   *time.Time
		want      string
		wantExact bool // if true, want exact match; otherwise substring
	}{
		{"nil", nil, "", true},
		{"yesterday", makeDate(-1), "yesterday", true},
		{"2 days ago", makeDate(-2), "2 days ago", true},
		{"10 days ago", makeDate(-10), "10 days ago", true},
		{"today", makeDate(0), "today", true},
		{"tomorrow", makeDate(1), "tomorrow", true},
		{"in 3 days", makeDate(3), "in 3 days", true},
		{"next week", makeDate(10), "next week", true},
		{"in 2 weeks", makeDate(15), "in 2 weeks", true},
		{"in 3 weeks", makeDate(22), "in 3 weeks", true},
		{"in 1 month singular", makeDate(30), "in 1 month", true},
		{"in 2 months plural", makeDate(60), "in 2 months", true},
		{"in 3 months plural", makeDate(90), "in 3 months", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRelativeDueVerbose(tt.dueDate)
			if tt.wantExact {
				if got != tt.want {
					t.Errorf("formatRelativeDueVerbose() = %q, want %q", got, tt.want)
				}
			} else {
				if !strings.Contains(got, tt.want) {
					t.Errorf("formatRelativeDueVerbose() = %q, want to contain %q", got, tt.want)
				}
			}
		})
	}
}

func TestFormatRelativeDueVerboseSingularPlural(t *testing.T) {
	now := time.Now()

	// Verify singular forms never produce "1 months" or "1 weeks"
	for days := 1; days <= 365; days++ {
		d := now.AddDate(0, 0, days)
		result := formatRelativeDueVerbose(&d)
		// Check for exact "1 months" or "1 weeks" (not "11 months" etc.)
		if result == "in 1 months" {
			t.Errorf("days=%d produced %q (should use singular 'month')", days, result)
		}
		if result == "in 1 weeks" {
			t.Errorf("days=%d produced %q (should use singular 'week')", days, result)
		}
	}
}

package util

import (
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

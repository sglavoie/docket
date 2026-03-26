package models

import (
	"testing"
)

func TestRecurrencePatternString(t *testing.T) {
	tests := []struct {
		name    string
		pattern *RecurrencePattern
		want    string
	}{
		{"nil pattern", nil, ""},
		{"daily", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, "daily"},
		{"weekly", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}, "weekly"},
		{"monthly", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, "monthly"},
		{"yearly", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, "yearly"},
		{"every 2 days", &RecurrencePattern{Interval: 2, Unit: RecurrenceDaily}, "every 2 days"},
		{"every 2 weeks", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}, "every 2 weeks"},
		{"every 3 months", &RecurrencePattern{Interval: 3, Unit: RecurrenceMonthly}, "every 3 months"},
		{"weekly on Mon", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1}}, "weekly on Mon"},
		{"weekly on Mon and Fri", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}}, "weekly on Mon and Fri"},
		{"weekly on Mon, Wed, and Fri", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 5}}, "weekly on Mon, Wed, and Fri"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pattern.String()
			if got != tc.want {
				t.Errorf("RecurrencePattern.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecurrencePatternStringEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		pattern *RecurrencePattern
		want    string
	}{
		{
			"empty weekdays slice",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{}},
			"weekly",
		},
		{
			"unsorted weekdays preserves order",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{5, 1, 3}}, // Fri, Mon, Wed
			"weekly on Fri, Mon, and Wed",                                                     // Current behavior: preserves input order
		},
		{
			"single weekday",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{3}},
			"weekly on Wed",
		},
		{
			"two weekdays",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}},
			"weekly on Mon and Fri",
		},
		{
			"biweekly includes weekdays in string",
			&RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 5}},
			"every 2 weeks on Mon, Wed, and Fri",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.pattern.String()
			if got != tc.want {
				t.Errorf("RecurrencePattern.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestWeekdaysToStringInvalidValues(t *testing.T) {
	tests := []struct {
		name     string
		weekdays []int
		want     string
	}{
		{"negative value filtered", []int{-1, 1, 3}, "Mon and Wed"},
		{"value above 6 filtered", []int{1, 7, 3}, "Mon and Wed"},
		{"large value filtered", []int{100, 1}, "Mon"},
		{"all invalid", []int{-1, 7, 100}, ""},
		{"mix valid invalid", []int{-5, 0, 8, 6}, "Sun and Sat"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := weekdaysToString(tc.weekdays)
			if got != tc.want {
				t.Errorf("weekdaysToString(%v) = %q, want %q", tc.weekdays, got, tc.want)
			}
		})
	}
}

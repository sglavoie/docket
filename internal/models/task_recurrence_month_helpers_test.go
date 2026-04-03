package models

import (
	"testing"
	"time"
)

func TestAddMonths(t *testing.T) {
	tests := []struct {
		name   string
		base   time.Time
		months int
		want   time.Time
	}{
		{
			"standard",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 + 1m = Feb 29 (leap)",
			time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 + 1m = Feb 28 (non-leap)",
			time.Date(2023, 1, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2023, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 + 2m = Mar 31",
			time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2024, 3, 31, 10, 0, 0, 0, time.UTC),
		},
		{
			"Mar 31 + 1m = Apr 30",
			time.Date(2024, 3, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 4, 30, 10, 0, 0, 0, time.UTC),
		},
		{
			"Dec 15 + 1m = Jan 15 next year",
			time.Date(2024, 12, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"Oct 31 + 1m = Nov 30",
			time.Date(2024, 10, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 11, 30, 10, 0, 0, 0, time.UTC),
		},
		{
			"preserves time component",
			time.Date(2024, 1, 15, 14, 30, 45, 123, time.UTC),
			1,
			time.Date(2024, 2, 15, 14, 30, 45, 123, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := addMonths(tc.base, tc.months, 0)
			if !got.Equal(tc.want) {
				t.Errorf("addMonths(%v, %d, 0) = %v, want %v", tc.base, tc.months, got, tc.want)
			}
		})
	}
}

func TestAddMonthsLargeValues(t *testing.T) {
	tests := []struct {
		name   string
		base   time.Time
		months int
		want   time.Time
	}{
		{
			"24 months = 2 years",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			24,
			time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"25 months with day clamping",
			time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
			25,
			time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"36 months = 3 years",
			time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC),
			36,
			time.Date(2027, 6, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 28 non-leap + 12 months",
			time.Date(2023, 2, 28, 10, 0, 0, 0, time.UTC),
			12,
			time.Date(2024, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"Dec 31 + 2 months",
			time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2025, 2, 28, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := addMonths(tc.base, tc.months, 0)
			if !got.Equal(tc.want) {
				t.Errorf("addMonths(%v, %d, 0) = %v, want %v", tc.base, tc.months, got, tc.want)
			}
		})
	}
}

func TestDaysInMonth(t *testing.T) {
	tests := []struct {
		year  int
		month time.Month
		want  int
	}{
		{2024, time.January, 31},
		{2024, time.February, 29},
		{2023, time.February, 28},
		{2024, time.March, 31},
		{2024, time.April, 30},
		{2024, time.May, 31},
		{2024, time.June, 30},
		{2024, time.July, 31},
		{2024, time.August, 31},
		{2024, time.September, 30},
		{2024, time.October, 31},
		{2024, time.November, 30},
		{2024, time.December, 31},
	}

	for _, tc := range tests {
		t.Run(tc.month.String(), func(t *testing.T) {
			got := daysInMonth(tc.year, tc.month)
			if got != tc.want {
				t.Errorf("daysInMonth(%d, %s) = %d, want %d", tc.year, tc.month, got, tc.want)
			}
		})
	}
}

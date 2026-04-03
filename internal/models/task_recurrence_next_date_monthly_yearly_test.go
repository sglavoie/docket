package models

import (
	"testing"
	"time"
)

func TestCalculateNextDateMonthly(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Time
		interval int
		want     time.Time
	}{
		{
			"standard monthly",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 2, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 to Feb (leap year) - clamp to 29",
			time.Date(2024, 1, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Jan 31 to Feb (non-leap year) - clamp to 28",
			time.Date(2023, 1, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2023, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"Mar 31 to Apr - clamp to 30",
			time.Date(2024, 3, 31, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 4, 30, 10, 0, 0, 0, time.UTC),
		},
		{
			"quarterly (3 months)",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			3,
			time.Date(2024, 4, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"crosses year boundary",
			time.Date(2024, 12, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"Nov 30 to Dec - keeps 30",
			time.Date(2024, 11, 30, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 12, 30, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceMonthly}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCalculateNextDateYearly(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Time
		interval int
		want     time.Time
	}{
		{
			"standard yearly",
			time.Date(2024, 3, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2025, 3, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"leap year Feb 29 to non-leap year - clamp to Feb 28",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2025, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"every 2 years",
			time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 leap to leap year (4 years)",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			4,
			time.Date(2028, 2, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 + 2 years - clamp to Feb 28",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 + 3 years - clamp to Feb 28",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			3,
			time.Date(2027, 2, 28, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceYearly}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCalculateNextDateMonthlyFromLeapDay(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Time
		interval int
		want     time.Time
	}{
		{
			"Feb 29 + 1m = Mar 29",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 3, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 + 12m (to non-leap year) = Feb 28",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			12,
			time.Date(2025, 2, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 + 48m (to leap year) = Feb 29",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			48,
			time.Date(2028, 2, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Feb 29 + 2m = Apr 29",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2024, 4, 29, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceMonthly}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

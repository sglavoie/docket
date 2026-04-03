package models

import (
	"testing"
	"time"
)

func TestFindNextWeekday(t *testing.T) {
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	wednesday := time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC)
	friday := time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC)
	sunday := time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		weekdays []int
		interval int
		want     time.Time
	}{
		{
			"Monday with Mon,Wed,Fri - next is Wed",
			monday,
			[]int{1, 3, 5},
			1,
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
		},
		{
			"Wednesday with Mon,Wed,Fri - next is Fri",
			wednesday,
			[]int{1, 3, 5},
			1,
			time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC),
		},
		{
			"Friday with Mon,Wed,Fri - next is Mon next week",
			friday,
			[]int{1, 3, 5},
			1,
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			"Friday with Mon,Fri biweekly - next is Mon 2 weeks later",
			friday,
			[]int{1, 5},
			2,
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Sunday with Tue,Thu - next is Tue",
			sunday,
			[]int{2, 4},
			1,
			time.Date(2024, 1, 23, 10, 0, 0, 0, time.UTC),
		},
		{
			"empty weekdays falls back to weekly",
			monday,
			[]int{},
			1,
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findNextWeekday(tc.base, tc.weekdays, tc.interval)
			if !got.Equal(tc.want) {
				t.Errorf("findNextWeekday() = %v (%s), want %v (%s)",
					got, got.Weekday(), tc.want, tc.want.Weekday())
			}
		})
	}
}

func TestFindNextWeekdayEdgeCases(t *testing.T) {
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	sunday := time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		weekdays []int
		interval int
		want     time.Time
	}{
		{
			"Monday with Mon only - next is Mon next week",
			monday,
			[]int{1},
			1,
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			"Monday with Mon only biweekly - next is Mon 2 weeks later",
			monday,
			[]int{1},
			2,
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"Sunday with Sun only - next is Sun next week",
			sunday,
			[]int{0},
			1,
			time.Date(2024, 1, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"unsorted weekdays Fri,Mon,Wed",
			monday,
			[]int{5, 1, 3},
			1,
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := findNextWeekday(tc.base, tc.weekdays, tc.interval)
			if !got.Equal(tc.want) {
				t.Errorf("findNextWeekday() = %v (%s), want %v (%s)",
					got, got.Weekday(), tc.want, tc.want.Weekday())
			}
		})
	}
}

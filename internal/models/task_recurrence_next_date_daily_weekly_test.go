package models

import (
	"testing"
	"time"
)

func TestCalculateNextDateDaily(t *testing.T) {
	tests := []struct {
		name     string
		base     time.Time
		interval int
		want     time.Time
	}{
		{
			"daily interval 1",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
		},
		{
			"daily interval 3",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			3,
			time.Date(2024, 1, 18, 10, 0, 0, 0, time.UTC),
		},
		{
			"crosses month boundary",
			time.Date(2024, 1, 30, 10, 0, 0, 0, time.UTC),
			3,
			time.Date(2024, 2, 2, 10, 0, 0, 0, time.UTC),
		},
		{
			"crosses year boundary",
			time.Date(2024, 12, 30, 10, 0, 0, 0, time.UTC),
			5,
			time.Date(2025, 1, 4, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceDaily}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCalculateNextDateWeekly(t *testing.T) {
	// 2024-01-15 is a Monday
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// 2024-01-19 is a Friday
	friday := time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		interval int
		weekdays []int
		want     time.Time
	}{
		{
			"simple weekly",
			monday,
			1,
			nil,
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			"biweekly",
			monday,
			2,
			nil,
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC),
		},
		{
			"weekly mon,wed,fri from Monday - next is Wed",
			monday,
			1,
			[]int{1, 3, 5},
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
		},
		{
			"weekly mon,wed,fri from Friday - next is Mon",
			friday,
			1,
			[]int{1, 3, 5},
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			"biweekly mon,fri from Friday - next is Mon 2 weeks later",
			friday,
			2,
			[]int{1, 5},
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceWeekly, Weekdays: tc.weekdays}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v (%s), want %v (%s)",
					got, got.Weekday(), tc.want, tc.want.Weekday())
			}
		})
	}
}

func TestCalculateNextDateWeeklyEdgeCases(t *testing.T) {
	// 2024-01-20 is Saturday (weekday 6)
	saturday := time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC)
	// 2024-01-21 is Sunday (weekday 0)
	sunday := time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		interval int
		weekdays []int
		want     time.Time
	}{
		{
			"Saturday with Mon-Fri pattern - next is Mon",
			saturday,
			1,
			[]int{1, 2, 3, 4, 5},
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC),
		},
		{
			"Sunday with Tue,Thu pattern - next is Tue",
			sunday,
			1,
			[]int{2, 4},
			time.Date(2024, 1, 23, 10, 0, 0, 0, time.UTC),
		},
		{
			"Sunday with Sun pattern - next is Sun next week",
			sunday,
			1,
			[]int{0},
			time.Date(2024, 1, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			"all 7 days from Monday",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			1,
			[]int{0, 1, 2, 3, 4, 5, 6},
			time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC),
		},
		{
			"all 7 days from Saturday",
			saturday,
			1,
			[]int{0, 1, 2, 3, 4, 5, 6},
			time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC),
		},
		{
			"Saturday with weekend pattern - next is Sun",
			saturday,
			1,
			[]int{0, 6},
			time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC),
		},
		{
			"Sunday with weekend pattern - next is Sat next week",
			sunday,
			1,
			[]int{0, 6},
			time.Date(2024, 1, 27, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{Interval: tc.interval, Unit: RecurrenceWeekly, Weekdays: tc.weekdays}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v (%s), want %v (%s)",
					got, got.Weekday(), tc.want, tc.want.Weekday())
			}
		})
	}
}

func TestCalculateNextDateLargeIntervals(t *testing.T) {
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		pattern *RecurrencePattern
		want    time.Time
	}{
		{
			"52 weeks",
			&RecurrencePattern{Interval: 52, Unit: RecurrenceWeekly},
			time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC),
		},
		{
			"365 days",
			&RecurrencePattern{Interval: 365, Unit: RecurrenceDaily},
			time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC),
		},
		{
			"12 months",
			&RecurrencePattern{Interval: 12, Unit: RecurrenceMonthly},
			time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
		},
		{
			"10 years",
			&RecurrencePattern{Interval: 10, Unit: RecurrenceYearly},
			time.Date(2034, 1, 15, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := calculateNextDate(base, tc.pattern)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCalculateNextDatePreservesTimeComponents(t *testing.T) {
	base := time.Date(2024, 1, 15, 14, 30, 45, 123456789, time.UTC)

	patterns := []*RecurrencePattern{
		{Interval: 1, Unit: RecurrenceDaily},
		{Interval: 1, Unit: RecurrenceWeekly},
		{Interval: 1, Unit: RecurrenceMonthly},
		{Interval: 1, Unit: RecurrenceYearly},
	}

	for _, p := range patterns {
		t.Run(string(p.Unit), func(t *testing.T) {
			got := calculateNextDate(base, p)
			if got.Hour() != 14 {
				t.Errorf("hour not preserved: got %d, want 14", got.Hour())
			}
			if got.Minute() != 30 {
				t.Errorf("minute not preserved: got %d, want 30", got.Minute())
			}
			if got.Second() != 45 {
				t.Errorf("second not preserved: got %d, want 45", got.Second())
			}
			if got.Nanosecond() != 123456789 {
				t.Errorf("nanosecond not preserved: got %d, want 123456789", got.Nanosecond())
			}
		})
	}
}

func TestCalculateNextDatePreservesTimezone(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skip("timezone not available")
	}

	base := time.Date(2024, 1, 15, 14, 30, 0, 0, loc)

	patterns := []*RecurrencePattern{
		{Interval: 1, Unit: RecurrenceDaily},
		{Interval: 1, Unit: RecurrenceWeekly},
		{Interval: 1, Unit: RecurrenceMonthly},
		{Interval: 1, Unit: RecurrenceYearly},
	}

	for _, p := range patterns {
		t.Run(string(p.Unit), func(t *testing.T) {
			got := calculateNextDate(base, p)
			if got.Location().String() != loc.String() {
				t.Errorf("timezone not preserved: got %s, want %s", got.Location(), loc)
			}
		})
	}
}

func TestCalculateNextDateWeeklyMultiWeekWrap(t *testing.T) {
	friday := time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		base     time.Time
		interval int
		weekdays []int
		want     time.Time
	}{
		{
			"Friday with 3-week interval, Mon+Fri pattern",
			friday,
			3,
			[]int{1, 5},
			time.Date(2024, 2, 5, 10, 0, 0, 0, time.UTC),
		},
		{
			"Friday with 4-week interval, Mon only",
			friday,
			4,
			[]int{1},
			time.Date(2024, 2, 12, 10, 0, 0, 0, time.UTC),
		},
		{
			"Monday with 3-week interval, Wed+Fri pattern - same week Wed first",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			3,
			[]int{3, 5},
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC),
		},
		{
			"Friday with 2-week interval, Sat pattern - same week Sat first",
			friday,
			2,
			[]int{6},
			time.Date(2024, 1, 20, 10, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &RecurrencePattern{
				Interval: tc.interval,
				Unit:     RecurrenceWeekly,
				Weekdays: tc.weekdays,
			}
			got := calculateNextDate(tc.base, r)
			if !got.Equal(tc.want) {
				t.Errorf("calculateNextDate() = %v (%s), want %v (%s)",
					got, got.Weekday(), tc.want, tc.want.Weekday())
			}
		})
	}
}

func TestCalculateNextDateWeeklyMultiWeekSequence(t *testing.T) {
	r := &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}}
	base := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	expected := []time.Time{
		time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 2, 10, 0, 0, 0, time.UTC),
		time.Date(2024, 2, 12, 10, 0, 0, 0, time.UTC),
	}

	current := base
	for i, want := range expected {
		got := calculateNextDate(current, r)
		if !got.Equal(want) {
			t.Errorf("step %d from %v (%s): got %v (%s), want %v (%s)",
				i, current, current.Weekday(), got, got.Weekday(), want, want.Weekday())
		}
		current = got
	}
}

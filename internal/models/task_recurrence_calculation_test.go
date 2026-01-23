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
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC), // next Monday
		},
		{
			"biweekly",
			monday,
			2,
			nil,
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC), // Monday 2 weeks later
		},
		{
			"weekly mon,wed,fri from Monday - next is Wed",
			monday,
			1,
			[]int{1, 3, 5}, // Mon, Wed, Fri
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC), // Wednesday same week
		},
		{
			"weekly mon,wed,fri from Friday - next is Mon",
			friday,
			1,
			[]int{1, 3, 5}, // Mon, Wed, Fri
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC), // Monday next week
		},
		{
			"biweekly mon,fri from Friday - next is Mon 2 weeks later",
			friday,
			2,
			[]int{1, 5}, // Mon, Fri
			time.Date(2024, 1, 29, 10, 0, 0, 0, time.UTC), // Monday 2 weeks later
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
			[]int{1, 2, 3, 4, 5}, // Mon-Fri
			time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC), // Monday
		},
		{
			"Sunday with Tue,Thu pattern - next is Tue",
			sunday,
			1,
			[]int{2, 4},
			time.Date(2024, 1, 23, 10, 0, 0, 0, time.UTC), // Tuesday
		},
		{
			"Sunday with Sun pattern - next is Sun next week",
			sunday,
			1,
			[]int{0}, // Sunday only
			time.Date(2024, 1, 28, 10, 0, 0, 0, time.UTC), // next Sunday
		},
		{
			"all 7 days from Monday",
			time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC), // Monday
			1,
			[]int{0, 1, 2, 3, 4, 5, 6},
			time.Date(2024, 1, 16, 10, 0, 0, 0, time.UTC), // Tuesday
		},
		{
			"all 7 days from Saturday",
			saturday,
			1,
			[]int{0, 1, 2, 3, 4, 5, 6},
			time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC), // Sunday
		},
		{
			"Saturday with weekend pattern - next is Sun",
			saturday,
			1,
			[]int{0, 6}, // Sat, Sun
			time.Date(2024, 1, 21, 10, 0, 0, 0, time.UTC), // Sunday
		},
		{
			"Sunday with weekend pattern - next is Sat next week",
			sunday,
			1,
			[]int{0, 6}, // Sat, Sun
			time.Date(2024, 1, 27, 10, 0, 0, 0, time.UTC), // next Saturday
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
			"leap year Feb 29 to non-leap year",
			time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC),
			1,
			time.Date(2025, 3, 1, 10, 0, 0, 0, time.UTC), // Go's AddDate behavior
		},
		{
			"every 2 years",
			time.Date(2024, 6, 1, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2026, 6, 1, 10, 0, 0, 0, time.UTC),
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
			time.Date(2025, 1, 13, 10, 0, 0, 0, time.UTC), // 52 weeks later
		},
		{
			"365 days",
			&RecurrencePattern{Interval: 365, Unit: RecurrenceDaily},
			time.Date(2025, 1, 14, 10, 0, 0, 0, time.UTC), // 365 days later (2024 is leap year, so 366 days in year)
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
	// Time with specific hour, minute, second, nanosecond
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

func TestFindNextWeekday(t *testing.T) {
	// 2024-01-15 is Monday (weekday 1)
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// 2024-01-17 is Wednesday (weekday 3)
	wednesday := time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC)
	// 2024-01-19 is Friday (weekday 5)
	friday := time.Date(2024, 1, 19, 10, 0, 0, 0, time.UTC)
	// 2024-01-21 is Sunday (weekday 0)
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
	// 2024-01-15 is Monday (weekday 1)
	monday := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	// 2024-01-21 is Sunday (weekday 0)
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
			[]int{5, 1, 3}, // out of order
			1,
			time.Date(2024, 1, 17, 10, 0, 0, 0, time.UTC), // Wednesday (sorts and finds next)
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
			got := addMonths(tc.base, tc.months)
			if !got.Equal(tc.want) {
				t.Errorf("addMonths(%v, %d) = %v, want %v", tc.base, tc.months, got, tc.want)
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
			time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC), // Feb 2026 is not leap
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
			time.Date(2024, 2, 28, 10, 0, 0, 0, time.UTC), // 2024 is leap but 28 fits
		},
		{
			"Dec 31 + 2 months",
			time.Date(2024, 12, 31, 10, 0, 0, 0, time.UTC),
			2,
			time.Date(2025, 2, 28, 10, 0, 0, 0, time.UTC), // Feb 2025 clamped
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := addMonths(tc.base, tc.months)
			if !got.Equal(tc.want) {
				t.Errorf("addMonths(%v, %d) = %v, want %v", tc.base, tc.months, got, tc.want)
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
		{2024, time.February, 29}, // leap year
		{2023, time.February, 28}, // non-leap year
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

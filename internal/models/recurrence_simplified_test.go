package models

import (
	"testing"
	"time"
)

func TestRecurrencePatternCanonicalShape(t *testing.T) {
	for _, pattern := range []*RecurrencePattern{
		{Interval: 1, Unit: RecurrenceDaily},
		{Interval: 2, Unit: RecurrenceWeekly},
		{Interval: 3, Unit: RecurrenceMonthly},
	} {
		if err := pattern.Validate(); err != nil {
			t.Errorf("Validate(%+v): %v", pattern, err)
		}
	}

	invalid := []*RecurrencePattern{
		nil,
		{Interval: 0, Unit: RecurrenceDaily},
		{Interval: MaxRecurrenceInterval + 1, Unit: RecurrenceDaily},
		{Interval: 1, Unit: RecurrenceUnit("year")},
	}
	for _, pattern := range invalid {
		if err := pattern.Validate(); err == nil {
			t.Errorf("Validate(%+v) unexpectedly succeeded", pattern)
		}
	}
}

func TestSetRecurrenceStoresAnIndependentCanonicalPattern(t *testing.T) {
	task := &Task{}
	pattern := &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}
	if err := task.SetRecurrence(pattern); err != nil {
		t.Fatalf("SetRecurrence: %v", err)
	}
	pattern.Interval = 99
	if task.Recurrence == pattern || task.Recurrence.Interval != 2 || task.Recurrence.Unit != RecurrenceWeekly {
		t.Fatalf("stored recurrence = %+v, want independent canonical 2-week pattern", task.Recurrence)
	}
	before := task.Recurrence
	if err := task.SetRecurrence(&RecurrencePattern{Interval: 1, Unit: RecurrenceUnit("year")}); err == nil {
		t.Fatal("SetRecurrence accepted yearly pattern")
	}
	if task.Recurrence != before {
		t.Fatal("rejected SetRecurrence changed the existing pattern")
	}
}

func TestParseRecurrenceAcceptsOnlyDayWeekMonthIntervals(t *testing.T) {
	tests := map[string]*RecurrencePattern{
		"daily":    {Interval: 1, Unit: RecurrenceDaily},
		"weekly":   {Interval: 1, Unit: RecurrenceWeekly},
		"biweekly": {Interval: 2, Unit: RecurrenceWeekly},
		"monthly":  {Interval: 1, Unit: RecurrenceMonthly},
		"3d":       {Interval: 3, Unit: RecurrenceDaily},
		"4w":       {Interval: 4, Unit: RecurrenceWeekly},
		"5m":       {Interval: 5, Unit: RecurrenceMonthly},
	}
	for input, want := range tests {
		got := ParseRecurrence(input)
		if got == nil || got.Interval != want.Interval || got.Unit != want.Unit {
			t.Errorf("ParseRecurrence(%q) = %+v, want %+v", input, got, want)
		}
	}
	for _, input := range []string{"", "yearly", "1y", "weekly:mon", "daily~", "0d", "1D"} {
		if got := ParseRecurrence(input); got != nil {
			t.Errorf("ParseRecurrence(%q) = %+v, want nil", input, got)
		}
	}
}

func TestRecurrencePatternString(t *testing.T) {
	tests := []struct {
		pattern *RecurrencePattern
		want    string
	}{
		{nil, ""},
		{&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, "daily"},
		{&RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}, "weekly"},
		{&RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, "monthly"},
		{&RecurrencePattern{Interval: 2, Unit: RecurrenceDaily}, "every 2 days"},
		{&RecurrencePattern{Interval: 3, Unit: RecurrenceWeekly}, "every 3 weeks"},
		{&RecurrencePattern{Interval: 4, Unit: RecurrenceMonthly}, "every 4 months"},
	}
	for _, test := range tests {
		if got := test.pattern.String(); got != test.want {
			t.Errorf("String(%+v) = %q, want %q", test.pattern, got, test.want)
		}
	}
}

func TestNextDueDateAfterPreservesCadenceAndCatchesUp(t *testing.T) {
	tests := []struct {
		name       string
		due        *time.Time
		completed  time.Time
		recurrence RecurrencePattern
		want       time.Time
	}{
		{
			name:       "late daily",
			due:        datePointer(2026, time.July, 1),
			completed:  dateAtNoon(2026, time.July, 5),
			recurrence: RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			want:       date(2026, time.July, 6),
		},
		{
			name:       "multi interval catch up",
			due:        datePointer(2026, time.January, 1),
			completed:  dateAtNoon(2026, time.January, 20),
			recurrence: RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly},
			want:       date(2026, time.January, 29),
		},
		{
			name:       "no due uses completion",
			completed:  dateAtNoon(2026, time.March, 10),
			recurrence: RecurrencePattern{Interval: 3, Unit: RecurrenceDaily},
			want:       date(2026, time.March, 13),
		},
		{
			name:       "monthly end of month",
			due:        datePointer(2024, time.January, 31),
			completed:  dateAtNoon(2024, time.February, 29),
			recurrence: RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
			want:       date(2024, time.March, 31),
		},
		{
			name:       "monthly multi interval catch up",
			due:        datePointer(2024, time.January, 31),
			completed:  dateAtNoon(2024, time.August, 1),
			recurrence: RecurrencePattern{Interval: 2, Unit: RecurrenceMonthly},
			want:       date(2024, time.September, 30),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			task := &Task{DueDate: test.due, Recurrence: &test.recurrence}
			got := task.NextDueDateAfter(test.completed)
			if got == nil || !got.Equal(test.want) {
				t.Fatalf("NextDueDateAfter() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestNextNDueDatesKeepsMonthlyEndOfMonth(t *testing.T) {
	completed := dateAtNoon(2024, time.January, 31)
	due := date(2023, time.December, 31)
	task := &Task{
		DueDate:     &due,
		CompletedAt: &completed,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
	}
	got := task.NextNDueDates(3)
	want := []time.Time{
		date(2024, time.February, 29),
		date(2024, time.March, 31),
		date(2024, time.April, 30),
	}
	if len(got) != len(want) {
		t.Fatalf("NextNDueDates() = %v, want %v", got, want)
	}
	for i := range want {
		if !got[i].Equal(want[i]) {
			t.Errorf("date %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func date(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func dateAtNoon(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, 12, 0, 0, 0, time.UTC)
}

func datePointer(year int, month time.Month, day int) *time.Time {
	value := date(year, month, day)
	return &value
}

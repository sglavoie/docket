package models

import (
	"testing"
	"time"
)

func TestParseRecurrence(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RecurrencePattern
		wantNil bool
	}{
		// Keywords
		{"daily keyword", "daily", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, false},
		{"weekly keyword", "weekly", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly}, false},
		{"biweekly keyword", "biweekly", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly}, false},
		{"monthly keyword", "monthly", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, false},
		{"yearly keyword", "yearly", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, false},

		// Interval format
		{"1d", "1d", &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily}, false},
		{"2d", "2d", &RecurrencePattern{Interval: 2, Unit: RecurrenceDaily}, false},
		{"3w", "3w", &RecurrencePattern{Interval: 3, Unit: RecurrenceWeekly}, false},
		{"1m", "1m", &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly}, false},
		{"2m", "2m", &RecurrencePattern{Interval: 2, Unit: RecurrenceMonthly}, false},
		{"1y", "1y", &RecurrencePattern{Interval: 1, Unit: RecurrenceYearly}, false},
		{"2y", "2y", &RecurrencePattern{Interval: 2, Unit: RecurrenceYearly}, false},
		{"10d", "10d", &RecurrencePattern{Interval: 10, Unit: RecurrenceDaily}, false},

		// Weekly with days
		{"weekly:mon", "weekly:mon", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1}}, false},
		{"weekly:mon,wed,fri", "weekly:mon,wed,fri", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 5}}, false},
		{"2w:mon,fri", "2w:mon,fri", &RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 5}}, false},
		{"weekly:sunday", "weekly:sunday", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{0}}, false},
		{"weekly:tue,thu", "weekly:tue,thu", &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly, Weekdays: []int{2, 4}}, false},

		// Invalid input
		{"empty string", "", nil, true},
		{"invalid keyword", "invalid", nil, true},
		{"0d", "0d", nil, true},
		{"negative -1d", "-1d", nil, true},
		{"letters only", "abc", nil, true},
		{"no number", "d", nil, true},
		{"invalid unit", "2x", nil, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseRecurrence(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Errorf("ParseRecurrence(%q) = %+v, want nil", tc.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseRecurrence(%q) = nil, want %+v", tc.input, tc.want)
			}
			if got.Interval != tc.want.Interval {
				t.Errorf("ParseRecurrence(%q).Interval = %d, want %d", tc.input, got.Interval, tc.want.Interval)
			}
			if got.Unit != tc.want.Unit {
				t.Errorf("ParseRecurrence(%q).Unit = %s, want %s", tc.input, got.Unit, tc.want.Unit)
			}
			if len(got.Weekdays) != len(tc.want.Weekdays) {
				t.Errorf("ParseRecurrence(%q).Weekdays = %v, want %v", tc.input, got.Weekdays, tc.want.Weekdays)
			} else {
				for i, wd := range got.Weekdays {
					if wd != tc.want.Weekdays[i] {
						t.Errorf("ParseRecurrence(%q).Weekdays[%d] = %d, want %d", tc.input, i, wd, tc.want.Weekdays[i])
					}
				}
			}
		})
	}
}

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

func TestTaskNextDueDateNilCases(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		recurrence *RecurrencePattern
		dueDate    *time.Time
		wantNil    bool
	}{
		{
			"no recurrence",
			nil,
			&dueDate,
			true,
		},
		{
			"no due date",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			nil,
			true,
		},
		{
			"both nil",
			nil,
			nil,
			true,
		},
		{
			"both set",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			&dueDate,
			false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &Task{
				Recurrence: tc.recurrence,
				DueDate:    tc.dueDate,
			}
			got := task.NextDueDate()
			if tc.wantNil && got != nil {
				t.Errorf("NextDueDate() = %v, want nil", got)
			}
			if !tc.wantNil && got == nil {
				t.Error("NextDueDate() = nil, want non-nil")
			}
		})
	}
}

func TestTaskNextDueDateCalculation(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	task := &Task{
		DueDate:    &dueDate,
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
	}

	got := task.NextDueDate()
	want := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)

	if got == nil {
		t.Fatal("NextDueDate() = nil, want non-nil")
	}
	if !got.Equal(want) {
		t.Errorf("NextDueDate() = %v, want %v", *got, want)
	}
}

func TestGenerateRecurringInstances(t *testing.T) {
	t.Run("completed recurring task generates new instance", func(t *testing.T) {
		dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		task := &Task{
			ID:         "task-1",
			ShortID:    1,
			Type:       TaskTypeTask,
			Title:      "Recurring Task",
			Status:     StatusDone,
			Priority:   2,
			Section:    SectionBacklog,
			Tags:       []string{"test", "recurring"},
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		}

		project := &Project{
			Name:        "Test Project",
			Tasks:       []*Task{task},
			NextShortID: 2,
		}

		count := project.GenerateRecurringInstances()

		if count != 1 {
			t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
		}
		if len(project.Tasks) != 2 {
			t.Fatalf("len(Tasks) = %d, want 2", len(project.Tasks))
		}

		// New instance should be first in the list
		newTask := project.Tasks[0]
		if newTask.Status != StatusTodo {
			t.Errorf("new task Status = %s, want todo", newTask.Status)
		}
		if newTask.Section != SectionInbox {
			t.Errorf("new task Section = %s, want inbox", newTask.Section)
		}
		if newTask.Title != "Recurring Task" {
			t.Errorf("new task Title = %s, want 'Recurring Task'", newTask.Title)
		}
		if newTask.Priority != 2 {
			t.Errorf("new task Priority = %d, want 2", newTask.Priority)
		}
		if len(newTask.Tags) != 2 || newTask.Tags[0] != "test" || newTask.Tags[1] != "recurring" {
			t.Errorf("new task Tags = %v, want [test recurring]", newTask.Tags)
		}
		if newTask.Recurrence == nil {
			t.Error("new task Recurrence should not be nil")
		}
		expectedDue := time.Date(2024, 1, 22, 10, 0, 0, 0, time.UTC)
		if newTask.DueDate == nil || !newTask.DueDate.Equal(expectedDue) {
			t.Errorf("new task DueDate = %v, want %v", newTask.DueDate, expectedDue)
		}

		// Original task should have recurrence cleared
		originalTask := project.Tasks[1]
		if originalTask.Recurrence != nil {
			t.Error("original task Recurrence should be nil after generation")
		}
	})

	t.Run("non-recurring completed task does not generate", func(t *testing.T) {
		task := &Task{
			ID:     "task-1",
			Status: StatusDone,
		}

		project := &Project{
			Tasks: []*Task{task},
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
		if len(project.Tasks) != 1 {
			t.Errorf("len(Tasks) = %d, want 1", len(project.Tasks))
		}
	})

	t.Run("recurring but not completed does not generate", func(t *testing.T) {
		dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		task := &Task{
			ID:         "task-1",
			Status:     StatusTodo,
			DueDate:    &dueDate,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}

		project := &Project{
			Tasks: []*Task{task},
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
		if len(project.Tasks) != 1 {
			t.Errorf("len(Tasks) = %d, want 1", len(project.Tasks))
		}
		if task.Recurrence == nil {
			t.Error("task Recurrence should still be set")
		}
	})

	t.Run("multiple recurring tasks completed", func(t *testing.T) {
		due1 := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		due2 := time.Date(2024, 2, 1, 10, 0, 0, 0, time.UTC)

		task1 := &Task{
			ID:         "task-1",
			ShortID:    1,
			Title:      "Task 1",
			Status:     StatusDone,
			DueDate:    &due1,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
		}
		task2 := &Task{
			ID:         "task-2",
			ShortID:    2,
			Title:      "Task 2",
			Status:     StatusDone,
			DueDate:    &due2,
			Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceMonthly},
		}
		task3 := &Task{
			ID:      "task-3",
			ShortID: 3,
			Title:   "Task 3",
			Status:  StatusDone,
			// No recurrence
		}

		project := &Project{
			Tasks:       []*Task{task1, task2, task3},
			NextShortID: 4,
		}

		count := project.GenerateRecurringInstances()

		if count != 2 {
			t.Errorf("GenerateRecurringInstances() = %d, want 2", count)
		}
		if len(project.Tasks) != 5 {
			t.Errorf("len(Tasks) = %d, want 5", len(project.Tasks))
		}
	})
}

func TestParseWeekdays(t *testing.T) {
	tests := []struct {
		input string
		want  []int
	}{
		{"mon", []int{1}},
		{"mon,wed,fri", []int{1, 3, 5}},
		{"sunday,saturday", []int{0, 6}},
		{"tue, thu", []int{2, 4}}, // with spaces
		{"MON,WED", []int{1, 3}},  // uppercase
		{"Mon,Mon", []int{1}},     // duplicates removed
		{"invalid", []int{}},
		{"", []int{}},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := parseWeekdays(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("parseWeekdays(%q) = %v, want %v", tc.input, got, tc.want)
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("parseWeekdays(%q)[%d] = %d, want %d", tc.input, i, v, tc.want[i])
				}
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

func TestGenerateRecurringInstancesNoDueDate(t *testing.T) {
	// Recurring task without due date should not generate new instance
	task := &Task{
		ID:         "task-1",
		ShortID:    1,
		Title:      "No Due Date Recurring",
		Status:     StatusDone,
		DueDate:    nil, // No due date
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	count := project.GenerateRecurringInstances()

	// Should still generate (NextDueDate returns nil, but instance is created)
	// Looking at the implementation, it will create instance with nil DueDate
	if count != 1 {
		t.Errorf("GenerateRecurringInstances() = %d, want 1", count)
	}
	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2", len(project.Tasks))
	}

	newTask := project.Tasks[0]
	if newTask.DueDate != nil {
		t.Errorf("new task DueDate should be nil when original had no due date")
	}
	if newTask.Recurrence == nil {
		t.Error("new task should still have recurrence pattern")
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
			"biweekly ignores weekdays in string",
			&RecurrencePattern{Interval: 2, Unit: RecurrenceWeekly, Weekdays: []int{1, 3, 5}},
			"every 2 weeks", // Current behavior: interval > 1 doesn't show weekdays
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

func TestParseWeekdaysEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []int
	}{
		{"mixed valid/invalid", "mon,invalid,fri", []int{1, 5}},
		{"extra whitespace", "mon , wed,  fri", []int{1, 3, 5}},
		{"tabs", "mon\t,\twed", []int{1, 3}},
		{"leading/trailing commas", ",mon,wed,", []int{1, 3}},
		{"multiple commas", "mon,,wed", []int{1, 3}},
		{"all invalid", "foo,bar,baz", []int{}},
		{"all full names", "monday,tuesday,wednesday", []int{1, 2, 3}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseWeekdays(tc.input)
			if len(got) != len(tc.want) {
				t.Errorf("parseWeekdays(%q) = %v, want %v", tc.input, got, tc.want)
				return
			}
			for i, v := range got {
				if v != tc.want[i] {
					t.Errorf("parseWeekdays(%q)[%d] = %d, want %d", tc.input, i, v, tc.want[i])
				}
			}
		})
	}
}

func TestGenerateRecurringInstancesDoesNotCopyAllFields(t *testing.T) {
	parentID := "parent-1"
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	task := &Task{
		ID:          "task-1",
		ShortID:     1,
		Type:        TaskTypeTask,
		Title:       "Recurring Task",
		Description: "This description should not be copied",
		Status:      StatusDone,
		Priority:    2,
		Section:     SectionBacklog,
		Tags:        []string{"test"},
		DueDate:     &dueDate,
		Recurrence:  &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
		ParentID:    &parentID,
		DependsOn:   []string{"dep-1", "dep-2"},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	project.GenerateRecurringInstances()

	newTask := project.Tasks[0]

	// Fields that SHOULD be copied
	if newTask.Title != "Recurring Task" {
		t.Errorf("Title should be copied, got %q", newTask.Title)
	}
	if newTask.Priority != 2 {
		t.Errorf("Priority should be copied, got %d", newTask.Priority)
	}
	if len(newTask.Tags) != 1 || newTask.Tags[0] != "test" {
		t.Errorf("Tags should be copied, got %v", newTask.Tags)
	}

	// Fields that should NOT be copied
	if newTask.Description != "" {
		t.Errorf("Description should not be copied, got %q", newTask.Description)
	}
	if newTask.ParentID != nil {
		t.Errorf("ParentID should not be copied, got %v", newTask.ParentID)
	}
	if len(newTask.DependsOn) != 0 {
		t.Errorf("DependsOn should not be copied, got %v", newTask.DependsOn)
	}
}

func TestGenerateRecurringInstancesIdempotency(t *testing.T) {
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	task := &Task{
		ID:         "task-1",
		ShortID:    1,
		Title:      "Recurring Task",
		Status:     StatusDone,
		DueDate:    &dueDate,
		Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
	}

	project := &Project{
		Name:        "Test Project",
		Tasks:       []*Task{task},
		NextShortID: 2,
	}

	// First call
	count1 := project.GenerateRecurringInstances()
	if count1 != 1 {
		t.Errorf("first call: count = %d, want 1", count1)
	}

	// Second call should not generate more (original task has recurrence cleared)
	count2 := project.GenerateRecurringInstances()
	if count2 != 0 {
		t.Errorf("second call: count = %d, want 0 (idempotent)", count2)
	}

	if len(project.Tasks) != 2 {
		t.Errorf("len(Tasks) = %d, want 2 after both calls", len(project.Tasks))
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

func TestParseRecurrenceCaseSensitive(t *testing.T) {
	// Document that ParseRecurrence is case-sensitive for keywords
	tests := []struct {
		input   string
		wantNil bool
	}{
		{"DAILY", true},
		{"Daily", true},
		{"WEEKLY", true},
		{"Weekly", true},
		{"MONTHLY", true},
		{"Monthly", true},
		{"YEARLY", true},
		{"Yearly", true},
		{"BIWEEKLY", true},
		{"Biweekly", true},
		{"2D", true},     // uppercase unit
		{"3W", true},     // uppercase unit
		{"1M", true},     // uppercase unit
		{"2Y", true},     // uppercase unit
		{"daily", false}, // lowercase works
		{"2d", false},    // lowercase unit works
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := ParseRecurrence(tc.input)
			if tc.wantNil && got != nil {
				t.Errorf("ParseRecurrence(%q) = %+v, want nil (case-sensitive)", tc.input, got)
			}
			if !tc.wantNil && got == nil {
				t.Errorf("ParseRecurrence(%q) = nil, want non-nil", tc.input)
			}
		})
	}
}

func TestParseRecurrenceMoreEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		wantInt int
	}{
		{"large interval 999d", "999d", false, 999},
		{"large interval 100y", "100y", false, 100},
		{"leading space", " daily", true, 0},
		{"trailing space", "daily ", true, 0},
		{"colon with no days", "1w:", true, 0},
		{"weekly colon no days", "weekly:", true, 0},
		{"just colon", ":", true, 0},
		{"number only", "123", true, 0},
		{"very long number", "99999999d", false, 99999999},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ParseRecurrence(tc.input)
			if tc.wantNil {
				if got != nil {
					t.Errorf("ParseRecurrence(%q) = %+v, want nil", tc.input, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("ParseRecurrence(%q) = nil, want non-nil", tc.input)
			}
			if got.Interval != tc.wantInt {
				t.Errorf("ParseRecurrence(%q).Interval = %d, want %d", tc.input, got.Interval, tc.wantInt)
			}
		})
	}
}

func TestGenerateRecurringInstancesNonDoneStatuses(t *testing.T) {
	// Only StatusDone should trigger generation
	statuses := []TaskStatus{StatusTodo, StatusInProgress, StatusBlocked, StatusCancelled}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			task := &Task{
				ID:         "task-1",
				ShortID:    1,
				Title:      "Non-done recurring",
				Status:     status,
				DueDate:    &dueDate,
				Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			}

			project := &Project{
				Tasks:       []*Task{task},
				NextShortID: 2,
			}

			count := project.GenerateRecurringInstances()

			if count != 0 {
				t.Errorf("GenerateRecurringInstances() with status %s = %d, want 0", status, count)
			}
			if len(project.Tasks) != 1 {
				t.Errorf("len(Tasks) = %d, want 1 (no new task)", len(project.Tasks))
			}
			if task.Recurrence == nil {
				t.Errorf("recurrence should NOT be cleared for non-done task")
			}
		})
	}
}

func TestGenerateRecurringInstancesPreservesTaskType(t *testing.T) {
	types := []TaskType{TaskTypeBug, TaskTypeIdea, TaskTypeEpic, TaskTypeStory}
	dueDate := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	for _, taskType := range types {
		t.Run(string(taskType), func(t *testing.T) {
			task := &Task{
				ID:         "task-1",
				ShortID:    1,
				Type:       taskType,
				Title:      "Recurring " + string(taskType),
				Status:     StatusDone,
				DueDate:    &dueDate,
				Recurrence: &RecurrencePattern{Interval: 1, Unit: RecurrenceWeekly},
			}

			project := &Project{
				Tasks:       []*Task{task},
				NextShortID: 2,
			}

			project.GenerateRecurringInstances()

			if len(project.Tasks) != 2 {
				t.Fatalf("len(Tasks) = %d, want 2", len(project.Tasks))
			}

			newTask := project.Tasks[0]
			if newTask.Type != taskType {
				t.Errorf("new task Type = %s, want %s", newTask.Type, taskType)
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

func TestGenerateRecurringInstancesEmptyProject(t *testing.T) {
	t.Run("nil tasks slice", func(t *testing.T) {
		project := &Project{
			Tasks:       nil,
			NextShortID: 1,
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
	})

	t.Run("empty tasks slice", func(t *testing.T) {
		project := &Project{
			Tasks:       []*Task{},
			NextShortID: 1,
		}

		count := project.GenerateRecurringInstances()

		if count != 0 {
			t.Errorf("GenerateRecurringInstances() = %d, want 0", count)
		}
	})
}

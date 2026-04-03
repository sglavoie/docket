package models

import (
	"testing"
	"time"
)

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
			"no due date but has recurrence",
			&RecurrencePattern{Interval: 1, Unit: RecurrenceDaily},
			nil,
			false,
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

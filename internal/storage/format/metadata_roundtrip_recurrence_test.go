package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_RecurrenceSubFields(t *testing.T) {
	task := &models.Task{
		ID:            "recur-sub-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      1,
		Recurrence: &models.RecurrencePattern{
			Interval:  1,
			Unit:      models.RecurrenceDaily,
			Skipped:   3,
			Streak:    5,
			MaxStreak: 10,
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}

	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Interval != 1 {
		t.Errorf("Interval = %d, want 1", parsed.Recurrence.Interval)
	}
	if parsed.Recurrence.Unit != models.RecurrenceDaily {
		t.Errorf("Unit = %s, want %s", parsed.Recurrence.Unit, models.RecurrenceDaily)
	}
	if parsed.Recurrence.Skipped != 3 {
		t.Errorf("Skipped = %d, want 3", parsed.Recurrence.Skipped)
	}
	if parsed.Recurrence.Streak != 5 {
		t.Errorf("Streak = %d, want 5", parsed.Recurrence.Streak)
	}
	if parsed.Recurrence.MaxStreak != 10 {
		t.Errorf("MaxStreak = %d, want 10", parsed.Recurrence.MaxStreak)
	}
}

func TestRoundTrip_RecurrenceFromCompletion(t *testing.T) {
	tests := []struct {
		name string
		r    *models.RecurrencePattern
	}{
		{
			name: "daily from-completion",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion},
		},
		{
			name: "every 3 days from-completion",
			r:    &models.RecurrencePattern{Interval: 3, Unit: models.RecurrenceDaily, Mode: models.RecurModeFromCompletion},
		},
		{
			name: "weekly:mon,fri from-completion",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceWeekly, Weekdays: []int{1, 5}, Mode: models.RecurModeFromCompletion},
		},
		{
			name: "monthly from-completion with streak",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceMonthly, Mode: models.RecurModeFromCompletion, Streak: 2, MaxStreak: 5},
		},
		// from-due (default) should NOT persist ~ suffix
		{
			name: "daily from-due (no tilde)",
			r:    &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			task := &models.Task{
				ID:            "rc-mode-1",
				Type:          models.TaskTypeTask,
				WorkflowState: models.StateReady,
				Priority:      1,
				Recurrence:    tc.r,
			}

			parsed := &models.Task{}
			warnings := parseWrittenMetadata(task, parsed)

			if len(warnings) != 0 {
				t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
			}
			if parsed.Recurrence == nil {
				t.Fatal("Recurrence is nil after round-trip")
			}
			if parsed.Recurrence.Interval != tc.r.Interval {
				t.Errorf("Interval = %d, want %d", parsed.Recurrence.Interval, tc.r.Interval)
			}
			if parsed.Recurrence.Unit != tc.r.Unit {
				t.Errorf("Unit = %q, want %q", parsed.Recurrence.Unit, tc.r.Unit)
			}
			if parsed.Recurrence.Mode != tc.r.Mode {
				t.Errorf("Mode = %q, want %q", parsed.Recurrence.Mode, tc.r.Mode)
			}
			if parsed.Recurrence.Streak != tc.r.Streak {
				t.Errorf("Streak = %d, want %d", parsed.Recurrence.Streak, tc.r.Streak)
			}
			if parsed.Recurrence.MaxStreak != tc.r.MaxStreak {
				t.Errorf("MaxStreak = %d, want %d", parsed.Recurrence.MaxStreak, tc.r.MaxStreak)
			}
		})
	}
}

func TestRoundTrip_RecurrenceMonthly(t *testing.T) {
	task := &models.Task{
		ID:            "monthly-task-1",
		Type:          models.TaskTypeTask,
		WorkflowState: models.StateReady,
		Priority:      2,
		Recurrence: &models.RecurrencePattern{
			Interval:  2,
			Unit:      models.RecurrenceMonthly,
			Skipped:   1,
			Streak:    4,
			MaxStreak: 8,
		},
	}

	parsed := &models.Task{}
	warnings := parseWrittenMetadata(task, parsed)

	if len(warnings) != 0 {
		t.Errorf("got %d warnings, want 0: %v", len(warnings), warnings)
	}
	if parsed.Recurrence == nil {
		t.Fatal("Recurrence is nil after round-trip")
	}
	if parsed.Recurrence.Unit != models.RecurrenceMonthly {
		t.Errorf("Unit = %q, want %q", parsed.Recurrence.Unit, models.RecurrenceMonthly)
	}
	if parsed.Recurrence.Interval != 2 {
		t.Errorf("Interval = %d, want 2", parsed.Recurrence.Interval)
	}
	if parsed.Recurrence.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", parsed.Recurrence.Skipped)
	}
	if parsed.Recurrence.Streak != 4 {
		t.Errorf("Streak = %d, want 4", parsed.Recurrence.Streak)
	}
	if parsed.Recurrence.MaxStreak != 8 {
		t.Errorf("MaxStreak = %d, want 8", parsed.Recurrence.MaxStreak)
	}
}

package format

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestRoundTrip_RecurrenceSubFields(t *testing.T) {
	task := &models.Task{
		ID:       "recur-sub-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 1,
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

func TestRoundTrip_RecurrenceMonthly(t *testing.T) {
	task := &models.Task{
		ID:       "monthly-task-1",
		Type:     models.TaskTypeTask,
		Status:   models.StatusTodo,
		Priority: 2,
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

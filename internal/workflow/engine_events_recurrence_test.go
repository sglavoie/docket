package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesEventTrigger_RecurrenceCompleted(t *testing.T) {
	tests := []struct {
		name     string
		prevTask *models.Task
		currTask *models.Task
		expected bool
	}{
		{
			"fires on done + recurrence cleared",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusDone},
			true,
		},
		{
			"fires on canceled + recurrence cleared",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusCanceled},
			true,
		},
		{
			"does not fire when still recurring",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}, Status: models.StatusDone},
			false,
		},
		{
			"does not fire when not complete",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily}},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusInProgress},
			false,
		},
		{
			"does not fire when no previous recurrence",
			&models.Task{ID: "1", Recurrence: nil},
			&models.Task{ID: "1", Recurrence: nil, Status: models.StatusDone},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := RuleTrigger{Field: FieldRecurrenceCompleted, Operation: OpEquals, Value: "true"}
			if got := matchesEventTrigger(tt.currTask, tt.prevTask, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesEventTrigger_StreakBroken(t *testing.T) {
	tests := []struct {
		name     string
		prevTask *models.Task
		currTask *models.Task
		expected bool
	}{
		{
			"streak >0 to 0 fires",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			true,
		},
		{
			"streak >0 to recurrence removed fires",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 3}},
			&models.Task{ID: "1", Recurrence: nil},
			true,
		},
		{
			"streak unchanged does not fire",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 5}},
			false,
		},
		{
			"no previous recurrence does not fire",
			&models.Task{ID: "1", Recurrence: nil},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			false,
		},
		{
			"streak was already 0 does not fire",
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			&models.Task{ID: "1", Recurrence: &models.RecurrencePattern{Interval: 1, Unit: models.RecurrenceDaily, Streak: 0}},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trigger := RuleTrigger{Field: FieldStreakBroken, Operation: OpEquals, Value: "true"}
			if got := matchesEventTrigger(tt.currTask, tt.prevTask, trigger); got != tt.expected {
				t.Errorf("matchesEventTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

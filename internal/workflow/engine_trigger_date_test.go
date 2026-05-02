package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_DueDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		dueDate  *time.Time
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "due in 2 days, less than 3",
			dueDate:  timePtr(now.AddDate(0, 0, 2)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: true,
		},
		{
			name:     "due in 5 days, not less than 3",
			dueDate:  timePtr(now.AddDate(0, 0, 5)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: false,
		},
		{
			name:     "overdue by 2 days (negative)",
			dueDate:  timePtr(now.AddDate(0, 0, -2)),
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "0"},
			expected: true,
		},
		{
			name:     "no due date",
			dueDate:  nil,
			trigger:  RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:      "task-1",
				DueDate: tt.dueDate,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_CreatedDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		trigger   RuleTrigger
		expected  bool
	}{
		{
			name:      "created 10 days ago, greater than 7",
			createdAt: now.AddDate(0, 0, -10),
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  true,
		},
		{
			name:      "created 3 days ago, not greater than 7",
			createdAt: now.AddDate(0, 0, -3),
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
		{
			name:      "created today, equals 0",
			createdAt: now,
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpEquals, Value: "0"},
			expected:  true,
		},
		{
			name:      "zero CreatedAt returns false",
			createdAt: time.Time{},
			trigger:   RuleTrigger{Field: FieldCreatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:        "task-1",
				CreatedAt: tt.createdAt,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_UpdatedDays(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		updatedAt time.Time
		trigger   RuleTrigger
		expected  bool
	}{
		{
			name:      "updated 10 days ago, greater than 7 (stale)",
			updatedAt: now.AddDate(0, 0, -10),
			trigger:   RuleTrigger{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  true,
		},
		{
			name:      "updated 3 days ago, not greater than 7",
			updatedAt: now.AddDate(0, 0, -3),
			trigger:   RuleTrigger{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
		{
			name:      "zero UpdatedAt returns false",
			updatedAt: time.Time{},
			trigger:   RuleTrigger{Field: FieldUpdatedDays, Operation: OpGreaterThan, Value: "7"},
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:        "task-1",
				UpdatedAt: tt.updatedAt,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

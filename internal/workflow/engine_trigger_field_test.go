package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_Status(t *testing.T) {
	task := &models.Task{
		ID:     "task-1",
		Status: models.StatusBlocked,
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "status equals match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			expected: true,
		},
		{
			name:     "status equals no match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			expected: false,
		},
		{
			name:     "status not equals match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpNotEquals, Value: "todo"},
			expected: true,
		},
		{
			name:     "status not equals no match",
			trigger:  RuleTrigger{Field: FieldStatus, Operation: OpNotEquals, Value: "blocked"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_Priority(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Priority: 1,
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "priority equals match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
			expected: true,
		},
		{
			name:     "priority greater than match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpLessThan, Value: "3"},
			expected: true,
		},
		{
			name:     "priority greater than no match",
			trigger:  RuleTrigger{Field: FieldPriority, Operation: OpGreaterThan, Value: "1"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_Tags(t *testing.T) {
	task := &models.Task{
		ID:   "task-1",
		Tags: []string{"backend", "urgent"},
	}

	tests := []struct {
		name     string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "tag contains match",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "backend"},
			expected: true,
		},
		{
			name:     "tag contains no match",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpContains, Value: "frontend"},
			expected: false,
		},
		{
			name:     "tag not equals match (tag not present)",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpNotEquals, Value: "frontend"},
			expected: true,
		},
		{
			name:     "tag not equals no match (tag present)",
			trigger:  RuleTrigger{Field: FieldTags, Operation: OpNotEquals, Value: "backend"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

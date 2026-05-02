package workflow

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_HasDueDate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		dueDate  *time.Time
		value    string
		expected bool
	}{
		{"with due date, trigger true", timePtr(now), "true", true},
		{"with due date, trigger false", timePtr(now), "false", false},
		{"without due date, trigger true", nil, "true", false},
		{"without due date, trigger false", nil, "false", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", DueDate: tt.dueDate}
			trigger := RuleTrigger{Field: FieldHasDueDate, Operation: OpEquals, Value: tt.value}
			if got := matchesTrigger(task, trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_HasTags(t *testing.T) {
	tests := []struct {
		name     string
		tags     []string
		value    string
		expected bool
	}{
		{"with tags, trigger true", []string{"backend"}, "true", true},
		{"with tags, trigger false", []string{"backend"}, "false", false},
		{"without tags, trigger true", []string{}, "true", false},
		{"without tags, trigger false", []string{}, "false", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", Tags: tt.tags}
			trigger := RuleTrigger{Field: FieldHasTags, Operation: OpEquals, Value: tt.value}
			if got := matchesTrigger(task, trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesTrigger_DescriptionEmpty(t *testing.T) {
	tests := []struct {
		name        string
		description string
		value       string
		expected    bool
	}{
		{"empty description, trigger true", "", "true", true},
		{"empty description, trigger false", "", "false", false},
		{"non-empty description, trigger true", "Some description", "true", false},
		{"non-empty description, trigger false", "Some description", "false", true},
		{"whitespace-only description, trigger true", "   ", "true", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", Description: tt.description}
			trigger := RuleTrigger{Field: FieldDescriptionEmpty, Operation: OpEquals, Value: tt.value}
			if got := matchesTrigger(task, trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

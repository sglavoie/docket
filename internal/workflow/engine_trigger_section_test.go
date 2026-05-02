package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_SectionWithCustomSection(t *testing.T) {
	tests := []struct {
		name          string
		section       string
		customSection string
		trigger       RuleTrigger
		expected      bool
	}{
		{
			name:          "GTD section matches Section field",
			section:       "Inbox",
			customSection: "",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
			expected:      true,
		},
		{
			name:          "custom section matches CustomSection field",
			section:       "",
			customSection: "Sprint-1",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Sprint-1"},
			expected:      true,
		},
		{
			name:          "custom section case-insensitive",
			section:       "",
			customSection: "Sprint-1",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "sprint-1"},
			expected:      true,
		},
		{
			name:          "non-GTD value does not match GTD section",
			section:       "Inbox",
			customSection: "",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Sprint-1"},
			expected:      false,
		},
		{
			name:          "GTD value does not match custom section",
			section:       "",
			customSection: "Sprint-1",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
			expected:      false,
		},
		{
			name:          "empty trigger does not match task in custom section",
			section:       "",
			customSection: "Sprint-1",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""},
			expected:      false,
		},
		{
			name:          "empty trigger matches task in Active view",
			section:       "",
			customSection: "",
			trigger:       RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:            "task-1",
				Section:       tt.section,
				CustomSection: tt.customSection,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

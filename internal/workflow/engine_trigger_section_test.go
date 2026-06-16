package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesTrigger_Section(t *testing.T) {
	tests := []struct {
		name     string
		section  string
		trigger  RuleTrigger
		expected bool
	}{
		{
			name:     "GTD section matches Section field",
			section:  models.SectionInbox,
			trigger:  RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Inbox"},
			expected: true,
		},
		{
			name:     "case-insensitive section match",
			section:  models.SectionInbox,
			trigger:  RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "inbox"},
			expected: true,
		},
		{
			name:     "non-matching section",
			section:  models.SectionInbox,
			trigger:  RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: "Next"},
			expected: false,
		},
		{
			name:     "empty trigger matches Active view",
			section:  "",
			trigger:  RuleTrigger{Field: FieldSection, Operation: OpEquals, Value: ""},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:      "task-1",
				Section: tt.section,
			}
			if got := matchesTrigger(task, tt.trigger); got != tt.expected {
				t.Errorf("matchesTrigger() = %v, want %v", got, tt.expected)
			}
		})
	}
}

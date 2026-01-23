package workflow

import (
	"testing"
	"time"

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

func TestMatchesRule_CompoundAND(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Status:   models.StatusBlocked,
		Priority: 1,
	}

	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Compound AND",
		Enabled: true,
		Triggers: []RuleTrigger{
			{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		},
		TriggerLogic: LogicAnd,
		Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "urgent"}},
	}

	if !matchesRule(task, &rule) {
		t.Error("expected compound AND rule to match when all conditions met")
	}

	// Change one condition to not match
	task.Priority = 3
	if matchesRule(task, &rule) {
		t.Error("expected compound AND rule to not match when one condition fails")
	}
}

func TestMatchesRule_CompoundOR(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Status:   models.StatusTodo,
		Priority: 1,
	}

	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Compound OR",
		Enabled: true,
		Triggers: []RuleTrigger{
			{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
			{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		},
		TriggerLogic: LogicOr,
		Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "important"}},
	}

	// Only priority matches, but OR should still succeed
	if !matchesRule(task, &rule) {
		t.Error("expected compound OR rule to match when one condition met")
	}

	// Neither matches
	task.Priority = 3
	if matchesRule(task, &rule) {
		t.Error("expected compound OR rule to not match when no conditions met")
	}
}

func TestMatchesRule_LegacySingleTrigger(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Priority: 1,
	}

	// Legacy rule with single Trigger field (not Triggers slice)
	rule := WorkflowRule{
		ID:      "rule-1",
		Name:    "Legacy",
		Enabled: true,
		Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "test"}},
	}

	if !matchesRule(task, &rule) {
		t.Error("expected legacy single trigger rule to match")
	}
}

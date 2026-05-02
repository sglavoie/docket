package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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

func TestMatchesRule_TriggerGroup(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		Priority: 1,
		Status:   models.StatusInProgress,
		Tags:     []string{},
		Type:     models.TaskTypeTask,
	}

	rule := WorkflowRule{
		ID:      "rule-group",
		Name:    "Group Rule",
		Enabled: true,
		TriggerGroup: &TriggerGroup{
			Logic: LogicAnd,
			Triggers: []RuleTrigger{
				{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				{Field: FieldStatus, Operation: OpEquals, Value: "in_progress"},
			},
		},
		Actions: []RuleAction{{ActionType: ActionAddTag, Value: "urgent"}},
	}

	if !matchesRule(task, &rule) {
		t.Error("expected matchesRule to match rule with TriggerGroup")
	}

	// Non-matching case
	nonMatchTask := &models.Task{
		ID:       "task-2",
		Priority: 5,
		Status:   models.StatusTodo,
		Tags:     []string{},
		Type:     models.TaskTypeTask,
	}
	if matchesRule(nonMatchTask, &rule) {
		t.Error("expected matchesRule NOT to match when TriggerGroup conditions are not met")
	}
}

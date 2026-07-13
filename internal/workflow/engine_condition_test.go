package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestEvaluateRules_ConditionMet(t *testing.T) {
	// Action with a met condition should execute
	tasks := []*models.Task{
		{ID: "1", Title: "High pri task", Priority: 1, WorkflowState: models.StateReady, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Conditional Tag",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionAddTag,
					Value:      "urgent",
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("urgent") {
		t.Error("expected 'urgent' tag when condition is met")
	}
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
}

func TestEvaluateRules_ConditionNotMet(t *testing.T) {
	// Action with an unmet condition should be skipped
	tasks := []*models.Task{
		{ID: "1", Title: "Low pri task", Priority: 3, WorkflowState: models.StateReady, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Conditional Tag",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionAddTag,
					Value:      "urgent",
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	if tasks[0].HasTag("urgent") {
		t.Error("expected no 'urgent' tag when condition is not met")
	}
	if len(result.ModifiedTasks) != 0 {
		t.Errorf("expected 0 modified tasks, got %d", len(result.ModifiedTasks))
	}
}

func TestEvaluateRules_ComplementaryConditions(t *testing.T) {
	// Two actions with complementary conditions (branching pattern)
	// High priority -> Active section, low priority -> Someday section
	tasks := []*models.Task{
		{ID: "1", Title: "High pri", Priority: 1, WorkflowState: models.StateInbox},
		{ID: "2", Title: "Low pri", Priority: 3, WorkflowState: models.StateInbox},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Priority Router",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionSetSection,
					Value:      "", // Active
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpLessThan, Value: "3"},
				},
				{
					ActionType: ActionSetSection,
					Value:      "Someday",
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpGreaterThan, Value: "2"},
				},
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	if models.SectionForState(tasks[0].State()) != "" {
		t.Errorf("expected high pri task section to be '' (Active), got %q", models.SectionForState(tasks[0].State()))
	}
	if models.SectionForState(tasks[1].State()) != "Someday" {
		t.Errorf("expected low pri task section to be 'Someday', got %q", models.SectionForState(tasks[1].State()))
	}
	if len(result.ModifiedTasks) != 2 {
		t.Errorf("expected 2 modified tasks, got %d", len(result.ModifiedTasks))
	}
}

func TestEvaluateRules_NoConditionBackwardCompat(t *testing.T) {
	// Actions without conditions should work exactly as before
	tasks := []*models.Task{
		{ID: "1", Title: "Test task", WorkflowState: models.StateReady, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Unconditional",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "processed"},
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("processed") {
		t.Error("expected 'processed' tag for unconditional action")
	}
	if len(result.ModifiedTasks) != 1 {
		t.Errorf("expected 1 modified task, got %d", len(result.ModifiedTasks))
	}
}

func TestEvaluateRules_ConditionExcludedFromConflict(t *testing.T) {
	// An action with an unmet condition should not contribute to conflict detection.
	// Rule A sets section to Active (unconditional), Rule B sets section to Next
	// (conditional on priority=1). For a priority=3 task, Rule B's action is unmet
	// so there should be no conflict.
	tasks := []*models.Task{
		{ID: "1", Title: "Normal task", Priority: 3, WorkflowState: models.StateInbox},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-a",
			Name:    "Move Active",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{ActionType: ActionSetSection, Value: ""},
			},
		},
		{
			ID:      "rule-b",
			Name:    "Move Next If High Pri",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{
					ActionType: ActionSetSection,
					Value:      models.SectionNext,
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
			},
		},
	}

	result := EvaluateRules(tasks, rules)

	// No conflict because rule-b's action condition is not met for priority=3
	if len(result.Conflicts) != 0 {
		t.Errorf("expected 0 conflicts (unmet condition excluded), got %d", len(result.Conflicts))
	}
	// Rule A's action should apply
	if models.SectionForState(tasks[0].State()) != "" {
		t.Errorf("expected section to be '' (Active), got %q", models.SectionForState(tasks[0].State()))
	}
}

func TestEvaluateRules_MixedConditionalAndUnconditional(t *testing.T) {
	// A rule with both conditional and unconditional actions
	tasks := []*models.Task{
		{ID: "1", Title: "Task", Priority: 1, WorkflowState: models.StateReady, Tags: []string{}},
	}

	rules := []WorkflowRule{
		{
			ID:      "rule-1",
			Name:    "Mixed Actions",
			Enabled: true,
			Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
			Actions: []RuleAction{
				{ActionType: ActionAddTag, Value: "processed"}, // unconditional
				{
					ActionType: ActionAddTag,
					Value:      "urgent",
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
				{
					ActionType: ActionAddTag,
					Value:      "low-pri",
					Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpGreaterThan, Value: "2"},
				},
			},
		},
	}

	EvaluateRules(tasks, rules)

	if !tasks[0].HasTag("processed") {
		t.Error("expected 'processed' tag (unconditional)")
	}
	if !tasks[0].HasTag("urgent") {
		t.Error("expected 'urgent' tag (condition met: priority=1)")
	}
	if tasks[0].HasTag("low-pri") {
		t.Error("expected no 'low-pri' tag (condition not met: priority=1, not >2)")
	}
}

func TestRuleActionValidate_InvalidCondition(t *testing.T) {
	action := RuleAction{
		ActionType: ActionAddTag,
		Value:      "test",
		Condition: &RuleTrigger{
			Field:     "invalid_field",
			Operation: OpEquals,
			Value:     "test",
		},
	}

	err := action.Validate()
	if err == nil {
		t.Error("expected validation error for invalid condition field")
	}
}

func TestRuleActionValidate_ValidCondition(t *testing.T) {
	action := RuleAction{
		ActionType: ActionAddTag,
		Value:      "test",
		Condition: &RuleTrigger{
			Field:     FieldPriority,
			Operation: OpEquals,
			Value:     "1",
		},
	}

	err := action.Validate()
	if err != nil {
		t.Errorf("expected no error for valid condition, got: %v", err)
	}
}

func TestRuleActionString_WithCondition(t *testing.T) {
	action := RuleAction{
		ActionType: ActionSetSection,
		Value:      "",
		Condition: &RuleTrigger{
			Field:     FieldPriority,
			Operation: OpGreaterThan,
			Value:     "2",
		},
	}

	s := action.stringWithCondition()
	expected := "move to Active (if priority > 2)"
	if s != expected {
		t.Errorf("expected %q, got %q", expected, s)
	}

	// Without condition
	action.Condition = nil
	s = action.stringWithCondition()
	expected = "move to Active"
	if s != expected {
		t.Errorf("expected %q, got %q", expected, s)
	}
}

func TestRuleSummary_WithCondition(t *testing.T) {
	rule := WorkflowRule{
		Name:    "Test Rule",
		Trigger: RuleTrigger{Field: FieldStatus, Operation: OpEquals, Value: "todo"},
		Actions: []RuleAction{
			{
				ActionType: ActionSetSection,
				Value:      "",
				Condition:  &RuleTrigger{Field: FieldPriority, Operation: OpLessThan, Value: "3"},
			},
		},
	}

	summary := rule.Summary()
	expected := "When status = todo: move to Active (if priority < 3)"
	if summary != expected {
		t.Errorf("expected %q, got %q", expected, summary)
	}
}

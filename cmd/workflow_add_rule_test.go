package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestWorkflowAdd_NewRuleCreation(t *testing.T) {
	trigger := workflow.RuleTrigger{
		Field:     workflow.FieldState,
		Operation: workflow.OpEquals,
		Value:     "done",
	}
	actions := []workflow.RuleAction{
		{ActionType: workflow.ActionSetState, Value: "Completed"},
	}

	rule := workflow.NewRule("Test Rule", trigger, actions)

	if rule.ID == "" {
		t.Error("expected rule to have generated ID")
	}
	testutil.AssertEqual(t, rule.Name, "Test Rule")
	testutil.AssertEqual(t, rule.Enabled, true)
	testutil.AssertEqual(t, len(rule.Actions), 1)
}

func TestWorkflowAdd_NewCompoundRuleCreation(t *testing.T) {
	triggers := []workflow.RuleTrigger{
		{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
		{Field: workflow.FieldTags, Operation: workflow.OpContains, Value: "archive"},
	}
	actions := []workflow.RuleAction{
		{ActionType: workflow.ActionSetState, Value: "Completed"},
	}

	rule := workflow.NewCompoundRule("Compound Rule", triggers, workflow.LogicAnd, actions)

	if rule.ID == "" {
		t.Error("expected rule to have generated ID")
	}
	testutil.AssertEqual(t, rule.Name, "Compound Rule")
	testutil.AssertEqual(t, rule.Enabled, true)
	testutil.AssertEqual(t, len(rule.Triggers), 2)
	testutil.AssertEqual(t, rule.TriggerLogic, workflow.LogicAnd)
}

func TestWorkflowAdd_RuleValidation(t *testing.T) {
	tests := []struct {
		name      string
		ruleName  string
		triggers  []workflow.RuleTrigger
		actions   []workflow.RuleAction
		expectErr bool
	}{
		{
			name:     "valid rule",
			ruleName: "Valid Rule",
			triggers: []workflow.RuleTrigger{
				{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
			},
			actions: []workflow.RuleAction{
				{ActionType: workflow.ActionSetState, Value: "done"},
			},
			expectErr: false,
		},
		{
			name:      "missing name",
			ruleName:  "",
			triggers:  []workflow.RuleTrigger{{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"}},
			actions:   []workflow.RuleAction{{ActionType: workflow.ActionSetState}},
			expectErr: true,
		},
		{
			name:      "no triggers",
			ruleName:  "No Triggers",
			triggers:  []workflow.RuleTrigger{},
			actions:   []workflow.RuleAction{{ActionType: workflow.ActionSetState}},
			expectErr: true,
		},
		{
			name:     "no actions",
			ruleName: "No Actions",
			triggers: []workflow.RuleTrigger{
				{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
			},
			actions:   []workflow.RuleAction{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := &workflow.WorkflowRule{
				ID:       "test-id",
				Name:     tt.ruleName,
				Triggers: tt.triggers,
				Actions:  tt.actions,
			}

			err := rule.Validate()
			hasErr := err != nil

			if hasErr != tt.expectErr {
				t.Errorf("expected error=%v, got error=%v (%v)", tt.expectErr, hasErr, err)
			}
		})
	}
}

func TestWorkflowAdd_RuleSummary(t *testing.T) {
	trigger := workflow.RuleTrigger{
		Field:     workflow.FieldState,
		Operation: workflow.OpEquals,
		Value:     "done",
	}
	actions := []workflow.RuleAction{
		{ActionType: workflow.ActionSetState, Value: "done"},
	}

	rule := workflow.NewRule("Complete Done", trigger, actions)
	summary := rule.Summary()

	if summary == "" {
		t.Error("expected non-empty summary")
	}
}

func TestWorkflowAdd_TriggerString(t *testing.T) {
	tests := []struct {
		trigger  workflow.RuleTrigger
		expected string
	}{
		{
			workflow.RuleTrigger{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
			"state = done",
		},
		{
			workflow.RuleTrigger{Field: workflow.FieldTags, Operation: workflow.OpContains, Value: "urgent"},
			"tags contains urgent",
		},
		{
			workflow.RuleTrigger{Field: workflow.FieldPriority, Operation: workflow.OpLessThan, Value: "3"},
			"priority < 3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			got := tt.trigger.String()
			testutil.AssertEqual(t, got, tt.expected)
		})
	}
}

func TestWorkflowAdd_ActionString(t *testing.T) {
	tests := []struct {
		action   workflow.RuleAction
		contains string
	}{
		{workflow.RuleAction{ActionType: workflow.ActionSetState, Value: "done"}, "state"},
		{workflow.RuleAction{ActionType: workflow.ActionAddTag, Value: "urgent"}, "tag"},
		{workflow.RuleAction{ActionType: workflow.ActionSetState, Value: "done"}, "done"},
		{workflow.RuleAction{ActionType: workflow.ActionClearDue}, "clear"},
	}

	for _, tt := range tests {
		t.Run(string(tt.action.ActionType), func(t *testing.T) {
			got := tt.action.String()
			if got == "" {
				t.Error("expected non-empty action string")
			}
		})
	}
}

func TestWorkflowAdd_GetTriggers(t *testing.T) {
	singleTrigger := workflow.RuleTrigger{
		Field:     workflow.FieldState,
		Operation: workflow.OpEquals,
		Value:     "done",
	}

	ruleSingle := &workflow.WorkflowRule{
		Triggers: []workflow.RuleTrigger{singleTrigger},
	}

	triggers := ruleSingle.GetTriggers()
	testutil.AssertEqual(t, len(triggers), 1)

	ruleMultiple := &workflow.WorkflowRule{
		Triggers: []workflow.RuleTrigger{
			{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
			{Field: workflow.FieldTags, Operation: workflow.OpContains, Value: "archive"},
		},
	}

	triggers = ruleMultiple.GetTriggers()
	testutil.AssertEqual(t, len(triggers), 2)
}

func TestWorkflowAdd_GetTriggerLogic(t *testing.T) {
	ruleAnd := &workflow.WorkflowRule{TriggerLogic: workflow.LogicAnd}
	testutil.AssertEqual(t, ruleAnd.GetTriggerLogic(), workflow.LogicAnd)

	ruleOr := &workflow.WorkflowRule{TriggerLogic: workflow.LogicOr}
	testutil.AssertEqual(t, ruleOr.GetTriggerLogic(), workflow.LogicOr)

	ruleDefault := &workflow.WorkflowRule{}
	testutil.AssertEqual(t, ruleDefault.GetTriggerLogic(), workflow.LogicAnd)
}

func TestWorkflowAdd_IsCompound(t *testing.T) {
	ruleSingle := &workflow.WorkflowRule{
		Triggers: []workflow.RuleTrigger{
			{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
		},
	}

	if ruleSingle.IsCompound() {
		t.Error("expected single trigger rule to not be compound")
	}

	ruleCompound := &workflow.WorkflowRule{
		Triggers: []workflow.RuleTrigger{
			{Field: workflow.FieldState, Operation: workflow.OpEquals, Value: "done"},
			{Field: workflow.FieldTags, Operation: workflow.OpContains, Value: "archive"},
		},
	}

	if !ruleCompound.IsCompound() {
		t.Error("expected multiple trigger rule to be compound")
	}
}

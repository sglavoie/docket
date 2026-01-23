package workflow

import (
	"testing"
)

func TestRuleValidation(t *testing.T) {
	tests := []struct {
		name    string
		rule    WorkflowRule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test Rule",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: true,
		},
		{
			name: "invalid trigger field",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: "invalid", Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetSection, Value: ""}},
			},
			wantErr: true,
		},
		{
			name: "no actions",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{},
			},
			wantErr: true,
		},
		{
			name: "invalid action type",
			rule: WorkflowRule{
				ID:      "1",
				Name:    "Test",
				Enabled: true,
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: "invalid", Value: "x"}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRuleValidation_NewActions(t *testing.T) {
	tests := []struct {
		name    string
		rule    WorkflowRule
		wantErr bool
	}{
		{
			name: "valid set_due_relative",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetDueRelative, Value: "+3d"}},
			},
			wantErr: false,
		},
		{
			name: "invalid set_due_relative format",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetDueRelative, Value: "invalid"}},
			},
			wantErr: true,
		},
		{
			name: "valid clear_due (no value)",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionClearDue, Value: ""}},
			},
			wantErr: false,
		},
		{
			name: "valid set_active true",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetActive, Value: "true"}},
			},
			wantErr: false,
		},
		{
			name: "invalid set_active value",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				Actions: []RuleAction{{ActionType: ActionSetActive, Value: "yes"}},
			},
			wantErr: true,
		},
		{
			name: "valid compound rule",
			rule: WorkflowRule{
				Name: "Test",
				Triggers: []RuleTrigger{
					{Field: FieldStatus, Operation: OpEquals, Value: "blocked"},
					{Field: FieldPriority, Operation: OpEquals, Value: "1"},
				},
				TriggerLogic: LogicAnd,
				Actions:      []RuleAction{{ActionType: ActionAddTag, Value: "urgent"}},
			},
			wantErr: false,
		},
		{
			name: "valid time-based trigger",
			rule: WorkflowRule{
				Name:    "Test",
				Trigger: RuleTrigger{Field: FieldDueDays, Operation: OpLessThan, Value: "3"},
				Actions: []RuleAction{{ActionType: ActionSetPriority, Value: "1"}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseRelativeDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected int
		wantErr  bool
	}{
		{"+3d", 3, false},
		{"3d", 3, false},
		{"+1w", 7, false},
		{"+2w", 14, false},
		{"+1m", 30, false},
		{"+1y", 365, false},
		{"+10d", 10, false},
		{"", 0, true},
		{"d", 0, true},
		{"+0d", 0, true},
		{"+3x", 0, true},
		{"abc", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseRelativeDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRelativeDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.expected {
				t.Errorf("ParseRelativeDuration(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

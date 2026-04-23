package cmd

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/workflow"
)

// TestParseCommaSeparatedIndices tests index parsing with various inputs
func TestParseCommaSeparatedIndices(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    []int
		wantErr bool
	}{
		{"single index", "0", []int{0}, false},
		{"multiple indices", "0,2,4", []int{0, 2, 4}, false},
		{"with spaces", "0, 2, 4", []int{0, 2, 4}, false},
		{"duplicates removed", "0,2,0,4,2", []int{0, 2, 4}, false},
		{"empty string", "", nil, false},
		{"invalid format", "abc", nil, true},
		{"mixed invalid", "0,abc,2", nil, true},
		{"trailing chars rejected", "3abc", nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseCommaSeparatedIndices(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseCommaSeparatedIndices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !equalIntSlices(got, tt.want) {
				t.Errorf("parseCommaSeparatedIndices() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRemoveByIndices tests slice element removal
func TestRemoveByIndices(t *testing.T) {
	tests := []struct {
		name    string
		slice   []string
		indices []int
		want    []string
		wantErr bool
	}{
		{"remove single", []string{"a", "b", "c"}, []int{1}, []string{"a", "c"}, false},
		{"remove multiple", []string{"a", "b", "c", "d"}, []int{0, 2}, []string{"b", "d"}, false},
		{"remove none", []string{"a", "b"}, []int{}, []string{"a", "b"}, false},
		{"out of bounds", []string{"a", "b"}, []int{5}, nil, true},
		{"negative index", []string{"a", "b"}, []int{-1}, nil, true},
		{"empty slice", []string{}, []int{0}, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := removeByIndices(tt.slice, tt.indices)
			if (err != nil) != tt.wantErr {
				t.Errorf("removeByIndices() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !equalStringSlices(got, tt.want) {
				t.Errorf("removeByIndices() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCollectValidationErrors tests validation error collection
func TestCollectValidationErrors(t *testing.T) {
	tests := []struct {
		name      string
		rule      *workflow.WorkflowRule
		wantCount int
	}{
		{
			name: "valid rule",
			rule: &workflow.WorkflowRule{
				ID:   "test",
				Name: "Test Rule",
				Triggers: []workflow.RuleTrigger{
					{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
				},
				Actions: []workflow.RuleAction{
					{ActionType: workflow.ActionAddTag, Value: "archived"},
				},
			},
			wantCount: 0,
		},
		{
			name: "missing name",
			rule: &workflow.WorkflowRule{
				ID:   "test",
				Name: "",
				Triggers: []workflow.RuleTrigger{
					{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
				},
				Actions: []workflow.RuleAction{
					{ActionType: workflow.ActionAddTag, Value: "archived"},
				},
			},
			wantCount: 1,
		},
		{
			name: "no triggers",
			rule: &workflow.WorkflowRule{
				ID:       "test",
				Name:     "Test",
				Triggers: []workflow.RuleTrigger{},
				Actions: []workflow.RuleAction{
					{ActionType: workflow.ActionAddTag, Value: "archived"},
				},
			},
			wantCount: 1,
		},
		{
			name: "no actions",
			rule: &workflow.WorkflowRule{
				ID:   "test",
				Name: "Test",
				Triggers: []workflow.RuleTrigger{
					{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
				},
				Actions: []workflow.RuleAction{},
			},
			wantCount: 1,
		},
		{
			name: "valid rule with TriggerGroup instead of flat triggers",
			rule: &workflow.WorkflowRule{
				ID:   "test",
				Name: "TriggerGroup Rule",
				TriggerGroup: &workflow.TriggerGroup{
					Logic: workflow.LogicAnd,
					Triggers: []workflow.RuleTrigger{
						{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
						{Field: workflow.FieldTags, Operation: workflow.OpContains, Value: "release"},
					},
				},
				Actions: []workflow.RuleAction{
					{ActionType: workflow.ActionAddTag, Value: "archived"},
				},
			},
			wantCount: 0,
		},
		{
			name: "TriggerGroup with invalid trigger",
			rule: &workflow.WorkflowRule{
				ID:   "test",
				Name: "Bad TriggerGroup",
				TriggerGroup: &workflow.TriggerGroup{
					Logic: workflow.LogicAnd,
					Triggers: []workflow.RuleTrigger{
						{Field: "", Operation: workflow.OpEquals, Value: "done"},
					},
				},
				Actions: []workflow.RuleAction{
					{ActionType: workflow.ActionAddTag, Value: "archived"},
				},
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errors := collectValidationErrors(tt.rule)
			if len(errors) != tt.wantCount {
				t.Errorf("collectValidationErrors() found %d errors, want %d: %v", len(errors), tt.wantCount, errors)
			}
		})
	}
}

// TestRemoveTriggerFromLegacyRule verifies that --remove-trigger works on rules
// that use the legacy single Trigger field (rather than the Triggers slice).
// Before the fix, this would fail with "cannot remove from empty list" because
// removeByIndices was called on the nil Triggers slice without first migrating
// the legacy Trigger into it.
func TestRemoveTriggerFromLegacyRule(t *testing.T) {
	// Simulate a legacy rule with only the single Trigger field set
	rule := workflow.WorkflowRule{
		ID:      "test-legacy",
		Name:    "Legacy Rule",
		Enabled: true,
		Trigger: workflow.RuleTrigger{
			Field:     workflow.FieldStatus,
			Operation: workflow.OpEquals,
			Value:     "done",
		},
		// Triggers slice is empty (legacy format)
		Actions: []workflow.RuleAction{
			{ActionType: workflow.ActionAddTag, Value: "archived"},
		},
	}

	// Verify the legacy trigger is accessible via GetTriggers()
	triggers := rule.GetTriggers()
	if len(triggers) != 1 {
		t.Fatalf("expected 1 trigger via GetTriggers(), got %d", len(triggers))
	}

	// Now simulate what the edit command does after the fix:
	// Migrate legacy trigger before removal
	modifiedRule := rule
	if len(modifiedRule.Triggers) == 0 && modifiedRule.Trigger.Field != "" {
		modifiedRule.Triggers = []workflow.RuleTrigger{modifiedRule.Trigger}
		modifiedRule.Trigger = workflow.RuleTrigger{}
	}

	// Verify migration happened
	if len(modifiedRule.Triggers) != 1 {
		t.Fatalf("expected 1 trigger after migration, got %d", len(modifiedRule.Triggers))
	}
	if modifiedRule.Trigger.Field != "" {
		t.Error("expected legacy Trigger field to be cleared after migration")
	}

	// Now removeByIndices should work
	result, err := removeByIndices(modifiedRule.Triggers, []int{0})
	if err != nil {
		t.Fatalf("removeByIndices failed: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 triggers after removal, got %d", len(result))
	}
}

// TestRemoveTriggerFromLegacyRuleAddNew verifies that removing the legacy trigger
// and adding a new one in the same edit works correctly.
func TestRemoveTriggerFromLegacyRuleAddNew(t *testing.T) {
	rule := workflow.WorkflowRule{
		ID:      "test-replace",
		Name:    "Replace Trigger Rule",
		Enabled: true,
		Trigger: workflow.RuleTrigger{
			Field:     workflow.FieldStatus,
			Operation: workflow.OpEquals,
			Value:     "done",
		},
		Actions: []workflow.RuleAction{
			{ActionType: workflow.ActionAddTag, Value: "archived"},
		},
	}

	modifiedRule := rule

	// Migrate legacy trigger (happens for both --add-trigger and --remove-trigger paths)
	if len(modifiedRule.Triggers) == 0 && modifiedRule.Trigger.Field != "" {
		modifiedRule.Triggers = []workflow.RuleTrigger{modifiedRule.Trigger}
		modifiedRule.Trigger = workflow.RuleTrigger{}
	}

	// Remove old trigger
	var err error
	modifiedRule.Triggers, err = removeByIndices(modifiedRule.Triggers, []int{0})
	if err != nil {
		t.Fatalf("removeByIndices failed: %v", err)
	}

	// Add new trigger
	newTrigger := workflow.RuleTrigger{
		Field:     workflow.FieldPriority,
		Operation: workflow.OpEquals,
		Value:     "1",
	}
	modifiedRule.Triggers = append(modifiedRule.Triggers, newTrigger)

	// Validate the modified rule
	errors := collectValidationErrors(&modifiedRule)
	if len(errors) != 0 {
		t.Errorf("expected no validation errors, got: %v", errors)
	}

	// Check final state
	if len(modifiedRule.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(modifiedRule.Triggers))
	}
	if modifiedRule.Triggers[0].Field != workflow.FieldPriority {
		t.Errorf("expected priority trigger, got %s", modifiedRule.Triggers[0].Field)
	}
}

// TestCollectValidationErrors_LevelMismatch verifies that collectValidationErrors
// catches project-level rules with task-level triggers (and vice versa).
func TestCollectValidationErrors_LevelMismatch(t *testing.T) {
	// Project-level rule with a task-level trigger should produce an error.
	rule := &workflow.WorkflowRule{
		ID:    "test-level",
		Name:  "Bad Level Rule",
		Level: workflow.RuleLevelProject,
		Triggers: []workflow.RuleTrigger{
			{Field: workflow.FieldStatus, Operation: workflow.OpEquals, Value: "done"},
		},
		Actions: []workflow.RuleAction{
			{ActionType: workflow.ActionSetProjectStatus, Value: "active"},
		},
	}
	errors := collectValidationErrors(rule)
	if len(errors) == 0 {
		t.Error("expected validation error for project-level rule with task-level trigger")
	}

	found := false
	for _, e := range errors {
		if strings.Contains(e, "project-level rule") || strings.Contains(e, "task-level") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected level mismatch error, got: %v", errors)
	}
}

// Helper functions
func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

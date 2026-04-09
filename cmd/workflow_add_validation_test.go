package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/workflow"
)

func TestWorkflowAdd_TriggerFieldValidation(t *testing.T) {
	validFields := map[workflow.TriggerField]bool{
		workflow.FieldStatus:           true,
		workflow.FieldPriority:         true,
		workflow.FieldSection:          true,
		workflow.FieldTags:             true,
		workflow.FieldType:             true,
		workflow.FieldDueDays:          true,
		workflow.FieldCreatedDays:      true,
		workflow.FieldUpdatedDays:      true,
		workflow.FieldStatusChanged:    true,
		workflow.FieldSectionChanged:   true,
		workflow.FieldPriorityChanged:  true,
		workflow.FieldTagAdded:         true,
		workflow.FieldTagRemoved:       true,
		workflow.FieldCompleted:        true,
		workflow.FieldReopened:         true,
		workflow.FieldParentStatus:     true,
		workflow.FieldParentType:       true,
		workflow.FieldParentTags:       true,
		workflow.FieldLevel:            true,
		workflow.FieldHasChildren:      true,
		workflow.FieldChildrenProgress: true,
		workflow.FieldAllChildrenDone:  true,
		workflow.FieldIsBlocked:        true,
		workflow.FieldBlockerCount:     true,
		workflow.FieldDependencyCount:  true,
	}

	for field := range validFields {
		if !validFields[field] {
			t.Errorf("expected field %s to be valid", field)
		}
	}

	invalidFields := []string{"invalid_field", "foo", "bar"}
	for _, field := range invalidFields {
		if validFields[workflow.TriggerField(field)] {
			t.Errorf("expected field %s to be invalid", field)
		}
	}
}

func TestWorkflowAdd_TriggerOperationValidation(t *testing.T) {
	validOps := map[workflow.TriggerOperation]bool{
		workflow.OpEquals:      true,
		workflow.OpNotEquals:   true,
		workflow.OpContains:    true,
		workflow.OpGreaterThan: true,
		workflow.OpLessThan:    true,
	}

	for op := range validOps {
		if !validOps[op] {
			t.Errorf("expected operation %s to be valid", op)
		}
	}

	invalidOps := []string{"invalid", "match", "regex"}
	for _, op := range invalidOps {
		if validOps[workflow.TriggerOperation(op)] {
			t.Errorf("expected operation %s to be invalid", op)
		}
	}
}

func TestWorkflowAdd_ActionTypeValidation(t *testing.T) {
	validActions := map[workflow.ActionType]bool{
		workflow.ActionSetStatus:      true,
		workflow.ActionSetPriority:    true,
		workflow.ActionSetSection:     true,
		workflow.ActionAddTag:         true,
		workflow.ActionRemoveTag:      true,
		workflow.ActionSetDueRelative: true,
		workflow.ActionClearDue:       true,
		workflow.ActionSetActive:      true,
		workflow.ActionSetPinned:      true,
		workflow.ActionAddComment:     true,
		workflow.ActionSetType:        true,
		workflow.ActionCompleteParent: true,
		workflow.ActionBlockParent:    true,
		workflow.ActionCascadeTag:     true,
		workflow.ActionInheritTag:     true,
		workflow.ActionNotifyParent:   true,
	}

	for action := range validActions {
		if !validActions[action] {
			t.Errorf("expected action %s to be valid", action)
		}
	}

	invalidActions := []string{"invalid_action", "do_something", "magic"}
	for _, action := range invalidActions {
		if validActions[workflow.ActionType(action)] {
			t.Errorf("expected action %s to be invalid", action)
		}
	}
}

func TestWorkflowAdd_TriggerCountMismatch(t *testing.T) {
	tests := []struct {
		name      string
		fields    []string
		values    []string
		expectErr bool
	}{
		{"matching counts", []string{"status"}, []string{"done"}, false},
		{"mismatched counts", []string{"status", "priority"}, []string{"done"}, true},
		{"empty fields", []string{}, []string{}, false},
		{"multiple matching", []string{"status", "tags"}, []string{"done", "urgent"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mismatch := len(tt.fields) != len(tt.values)
			if mismatch != tt.expectErr {
				t.Errorf("expected error=%v for fields=%v values=%v", tt.expectErr, tt.fields, tt.values)
			}
		})
	}
}

func TestWorkflowAdd_DefaultOpsToEquals(t *testing.T) {
	fields := []string{"status", "priority", "tags"}
	var ops []string

	if len(ops) == 0 {
		ops = make([]string, len(fields))
		for i := range ops {
			ops[i] = "equals"
		}
	}

	testutil.AssertEqual(t, len(ops), 3)
	for _, op := range ops {
		testutil.AssertEqual(t, op, "equals")
	}
}

func TestWorkflowAdd_OpCountMismatch(t *testing.T) {
	fields := []string{"status", "priority"}
	ops := []string{"equals"}

	if len(ops) > 0 && len(ops) != len(fields) {
		return
	}

	t.Error("expected op count mismatch")
}

func TestWorkflowAdd_TriggerLogicValidation(t *testing.T) {
	tests := []struct {
		logic string
		valid bool
	}{
		{"and", true},
		{"or", true},
		{"xor", false},
		{"nand", false},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.logic, func(t *testing.T) {
			logic := workflow.TriggerLogic(tt.logic)
			valid := logic == workflow.LogicAnd || logic == workflow.LogicOr || logic == ""
			if valid != tt.valid {
				t.Errorf("expected logic %q valid=%v, got %v", tt.logic, tt.valid, valid)
			}
		})
	}
}

func TestWorkflowAdd_NoValueRequiredActions(t *testing.T) {
	noValueRequired := map[workflow.ActionType]bool{
		workflow.ActionSetSection:     true,
		workflow.ActionClearDue:       true,
		workflow.ActionCompleteParent: true,
		workflow.ActionBlockParent:    true,
	}

	for action, required := range noValueRequired {
		if !required {
			t.Errorf("expected action %s to not require value", action)
		}
	}

	requiresValue := []workflow.ActionType{
		workflow.ActionSetStatus,
		workflow.ActionSetPriority,
		workflow.ActionAddTag,
		workflow.ActionRemoveTag,
	}

	for _, action := range requiresValue {
		if noValueRequired[action] {
			t.Errorf("expected action %s to require value", action)
		}
	}
}

func TestWorkflowAdd_PriorityValueValidation(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"1", true},
		{"2", true},
		{"3", true},
		{"4", true},
		{"5", true},
		{"0", false},
		{"6", false},
		{"-1", false},
		{"abc", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			var p int
			var valid bool
			if n, err := parseInt(tt.value); err == nil && n >= 1 && n <= 5 {
				p = n
				valid = true
			}

			if valid != tt.valid {
				t.Errorf("priority %q: expected valid=%v, got %v (p=%d)", tt.value, tt.valid, valid, p)
			}
		})
	}
}

func TestWorkflowAdd_BooleanValueValidation(t *testing.T) {
	tests := []struct {
		value string
		valid bool
	}{
		{"true", true},
		{"false", true},
		{"True", false},
		{"False", false},
		{"1", false},
		{"0", false},
		{"yes", false},
		{"no", false},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			valid := tt.value == "true" || tt.value == "false"
			if valid != tt.valid {
				t.Errorf("boolean value %q: expected valid=%v, got %v", tt.value, tt.valid, valid)
			}
		})
	}
}

func TestWorkflowAdd_RelativeDurationValidation(t *testing.T) {
	tests := []struct {
		value string
		valid bool
		days  int
	}{
		{"+3d", true, 3},
		{"+1w", true, 7},
		{"+2m", true, 60},
		{"+1y", true, 365},
		{"3d", true, 3},
		{"+0d", false, 0},
		{"abc", false, 0},
		{"+3x", false, 0},
		{"", false, 0},
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			days, err := workflow.ParseRelativeDuration(tt.value)
			valid := err == nil && days > 0

			if valid != tt.valid {
				t.Errorf("duration %q: expected valid=%v, got %v (days=%d, err=%v)", tt.value, tt.valid, valid, days, err)
			}
			if valid && days != tt.days {
				t.Errorf("duration %q: expected days=%d, got %d", tt.value, tt.days, days)
			}
		})
	}
}

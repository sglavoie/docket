package cmd

import (
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

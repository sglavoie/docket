package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesScope_NilScope(t *testing.T) {
	rule := WorkflowRule{Scope: nil}
	task := &models.Task{ID: "1", Type: models.TaskTypeTask, Section: models.SectionNext}

	if !rule.MatchesScope(task, "MyProject", "proj-123") {
		t.Error("nil scope should match everything")
	}
}

func TestMatchesScope_ProjectMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Projects: []string{"MyProject"}},
	}
	task := &models.Task{ID: "1"}

	tests := []struct {
		name        string
		projectName string
		projectID   string
		expected    bool
	}{
		{"exact name match", "MyProject", "proj-123", true},
		{"case-insensitive match", "myproject", "proj-123", true},
		{"no match", "OtherProject", "proj-456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rule.MatchesScope(task, tt.projectName, tt.projectID); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_ProjectMatchByID(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Projects: []string{"proj-123"}},
	}
	task := &models.Task{ID: "1"}

	tests := []struct {
		name        string
		projectName string
		projectID   string
		expected    bool
	}{
		{"ID match when name differs", "SomeName", "proj-123", true},
		{"no match on either", "SomeName", "proj-456", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rule.MatchesScope(task, tt.projectName, tt.projectID); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_TypeMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Types: []string{"bug", "task"}},
	}

	tests := []struct {
		name     string
		taskType models.TaskType
		expected bool
	}{
		{"bug matches", models.TaskTypeBug, true},
		{"task matches", models.TaskTypeTask, true},
		{"epic does not match", models.TaskTypeEpic, false},
		{"story does not match", models.TaskTypeStory, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", Type: tt.taskType}
			if got := rule.MatchesScope(task, "Proj", "p1"); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_SectionListMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Sections: []string{models.SectionNext, models.SectionCompleted}},
	}

	tests := []struct {
		name     string
		section  string
		expected bool
	}{
		{"Next matches", models.SectionNext, true},
		{"Completed matches", models.SectionCompleted, true},
		{"case-insensitive match", "next", true},
		{"Active does not match", "", false},
		{"Inbox does not match", "Inbox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", Type: models.TaskTypeTask, Section: tt.section}
			if got := rule.MatchesScope(task, "Proj", "p1"); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_CombinedScope(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{
			Projects: []string{"MyProject"},
			Types:    []string{"bug"},
			Sections: []string{models.SectionNext},
		},
	}

	tests := []struct {
		name        string
		projectName string
		taskType    models.TaskType
		section     string
		expected    bool
	}{
		{"all match", "MyProject", models.TaskTypeBug, models.SectionNext, true},
		{"project mismatch", "Other", models.TaskTypeBug, models.SectionNext, false},
		{"type mismatch", "MyProject", models.TaskTypeTask, models.SectionNext, false},
		{"section mismatch", "MyProject", models.TaskTypeBug, "Inbox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{ID: "1", Type: tt.taskType, Section: tt.section}
			if got := rule.MatchesScope(task, tt.projectName, "p1"); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_SectionMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Sections: []string{models.SectionInbox, models.SectionNext}},
	}

	tests := []struct {
		name     string
		section  string
		expected bool
	}{
		{"section matches in scope list", models.SectionInbox, true},
		{"section case-insensitive", "inbox", true},
		{"neither matches", models.SectionWaiting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:      "1",
				Type:    models.TaskTypeTask,
				Section: tt.section,
			}
			if got := rule.MatchesScope(task, "Proj", "p1"); got != tt.expected {
				t.Errorf("MatchesScope() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestMatchesScope_EmptyFields(t *testing.T) {
	// Empty slices mean no restriction for that dimension
	rule := WorkflowRule{
		Scope: &RuleScope{
			Projects: []string{},
			Types:    []string{"bug"},
			Sections: []string{},
		},
	}

	task := &models.Task{ID: "1", Type: models.TaskTypeBug, Section: "Anything"}
	if !rule.MatchesScope(task, "AnyProject", "any-id") {
		t.Error("empty project/section slices should not restrict; only type should filter")
	}

	taskWrongType := &models.Task{ID: "2", Type: models.TaskTypeEpic, Section: "Anything"}
	if rule.MatchesScope(taskWrongType, "AnyProject", "any-id") {
		t.Error("non-empty type filter should reject non-matching types")
	}
}

func TestMatchesScope_NilTaskWithProjectScope(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Projects: []string{"proj-123"}},
	}

	if !rule.MatchesScope(nil, "AnyProject", "proj-123") {
		t.Fatal("project-only scope should match even without a task")
	}
}

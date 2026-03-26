package workflow

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestMatchesScope_NilScope(t *testing.T) {
	rule := WorkflowRule{Scope: nil}
	task := &models.Task{ID: "1", Type: models.TaskTypeTask, Section: "Backlog"}

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

func TestMatchesScope_SectionMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Sections: []string{"Backlog", "Archive"}},
	}

	tests := []struct {
		name     string
		section  string
		expected bool
	}{
		{"Backlog matches", "Backlog", true},
		{"Archive matches", "Archive", true},
		{"case-insensitive match", "backlog", true},
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
			Sections: []string{"Backlog"},
		},
	}

	tests := []struct {
		name        string
		projectName string
		taskType    models.TaskType
		section     string
		expected    bool
	}{
		{"all match", "MyProject", models.TaskTypeBug, "Backlog", true},
		{"project mismatch", "Other", models.TaskTypeBug, "Backlog", false},
		{"type mismatch", "MyProject", models.TaskTypeTask, "Backlog", false},
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

func TestMatchesScope_CustomSectionMatch(t *testing.T) {
	rule := WorkflowRule{
		Scope: &RuleScope{Sections: []string{"Sprint-1", "Backlog"}},
	}

	tests := []struct {
		name          string
		section       string
		customSection string
		expected      bool
	}{
		{"section matches in scope list", "Backlog", "", true}, // "Backlog" is in the scope list and matches task.Section
		{"custom section matches", "", "Sprint-1", true},
		{"custom section case-insensitive", "", "sprint-1", true},
		{"neither matches", "Inbox", "Sprint-2", false},
		{"GTD section in scope", "Backlog", "Sprint-2", true}, // Backlog is in scope
		{"custom section in scope with GTD section set", "Inbox", "Sprint-1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := &models.Task{
				ID:            "1",
				Type:          models.TaskTypeTask,
				Section:       tt.section,
				CustomSection: tt.customSection,
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

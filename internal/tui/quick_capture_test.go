package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestParseQuickCapture(t *testing.T) {
	// Create a test project with some tasks for parent resolution
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "epic-1", ShortID: 1, Type: models.TaskTypeEpic, Title: "User Authentication"},
			{ID: "story-2", ShortID: 2, Type: models.TaskTypeStory, Title: "Login Flow"},
			{ID: "task-3", ShortID: 3, Type: models.TaskTypeTask, Title: "Implement button"},
			{ID: "note-4", ShortID: 4, Type: models.TaskTypeNote, Title: "Meeting notes"},
		},
	}

	tests := []struct {
		name           string
		input          string
		wantTitle      string
		wantPriority   int
		wantTags       []string
		wantHasDueDate bool
		wantParentID   *string
		wantErr        bool
		errContains    string
	}{
		{
			name:         "simple title only",
			input:        "Fix login bug",
			wantTitle:    "Fix login bug",
			wantPriority: 3,
			wantTags:     nil,
		},
		{
			name:         "title with priority",
			input:        "Fix critical bug !1",
			wantTitle:    "Fix critical bug",
			wantPriority: 1,
			wantTags:     nil,
		},
		{
			name:         "title with low priority",
			input:        "Cleanup code !5",
			wantTitle:    "Cleanup code",
			wantPriority: 5,
			wantTags:     nil,
		},
		{
			name:         "priority at start",
			input:        "!2 Update documentation",
			wantTitle:    "Update documentation",
			wantPriority: 2,
			wantTags:     nil,
		},
		{
			name:         "single tag",
			input:        "Add feature #backend",
			wantTitle:    "Add feature",
			wantPriority: 3,
			wantTags:     []string{"backend"},
		},
		{
			name:         "multiple tags",
			input:        "Fix bug #backend #urgent #hotfix",
			wantTitle:    "Fix bug",
			wantPriority: 3,
			wantTags:     []string{"backend", "urgent", "hotfix"},
		},
		{
			name:         "tags at different positions",
			input:        "#frontend Add button #ui to sidebar",
			wantTitle:    "Add button to sidebar",
			wantPriority: 3,
			wantTags:     []string{"frontend", "ui"},
		},
		{
			name:         "priority and tags combined",
			input:        "Critical fix !1 #hotfix #production",
			wantTitle:    "Critical fix",
			wantPriority: 1,
			wantTags:     []string{"hotfix", "production"},
		},
		{
			name:           "with due date tomorrow",
			input:          "Review PR @tomorrow",
			wantTitle:      "Review PR",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date relative days",
			input:          "Submit report @+3d",
			wantTitle:      "Submit report",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date weekday",
			input:          "Team meeting @friday",
			wantTitle:      "Team meeting",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "with due date exact",
			input:          "Project deadline @2025-12-31",
			wantTitle:      "Project deadline",
			wantPriority:   3,
			wantHasDueDate: true,
		},
		{
			name:           "all metadata combined",
			input:          "Urgent task !1 #backend #critical @tomorrow",
			wantTitle:      "Urgent task",
			wantPriority:   1,
			wantTags:       []string{"backend", "critical"},
			wantHasDueDate: true,
		},
		{
			name:         "parent by short ID",
			input:        "Subtask under epic >1",
			wantTitle:    "Subtask under epic",
			wantPriority: 3,
			wantParentID: strPtr("epic-1"),
		},
		{
			name:         "parent by title prefix",
			input:        "Implementation detail >Login",
			wantTitle:    "Implementation detail",
			wantPriority: 3,
			wantParentID: strPtr("story-2"),
		},
		{
			name:           "full syntax with parent",
			input:          "Complete login form !2 #frontend @+1w >2",
			wantTitle:      "Complete login form",
			wantPriority:   2,
			wantTags:       []string{"frontend"},
			wantHasDueDate: true,
			wantParentID:   strPtr("story-2"),
		},
		{
			name:        "empty after extraction",
			input:       "!1 #tag @tomorrow",
			wantErr:     true,
			errContains: "title cannot be empty",
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:        "invalid due date",
			input:       "Task with bad date @notadate",
			wantErr:     true,
			errContains: "invalid date",
		},
		{
			name:        "parent not found",
			input:       "Task with missing parent >999",
			wantErr:     true,
			errContains: "parent not found",
		},
		{
			name:        "invalid parent type (note cannot have task children)",
			input:       "Task under note >4",
			wantErr:     true,
			errContains: "cannot be child",
		},
		{
			name:         "extra whitespace handling",
			input:        "  Fix   the   bug   !2   #backend   ",
			wantTitle:    "Fix the bug",
			wantPriority: 2,
			wantTags:     []string{"backend"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuickCapture(tt.input, project)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.title != tt.wantTitle {
				t.Errorf("title = %q, want %q", result.title, tt.wantTitle)
			}

			if result.priority != tt.wantPriority {
				t.Errorf("priority = %d, want %d", result.priority, tt.wantPriority)
			}

			if !slicesEqual(result.tags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", result.tags, tt.wantTags)
			}

			hasDueDate := result.dueDate != nil
			if hasDueDate != tt.wantHasDueDate {
				t.Errorf("hasDueDate = %v, want %v", hasDueDate, tt.wantHasDueDate)
			}

			if tt.wantParentID != nil {
				if result.parentID == nil {
					t.Errorf("parentID = nil, want %q", *tt.wantParentID)
				} else if *result.parentID != *tt.wantParentID {
					t.Errorf("parentID = %q, want %q", *result.parentID, *tt.wantParentID)
				}
			} else if result.parentID != nil {
				t.Errorf("parentID = %q, want nil", *result.parentID)
			}
		})
	}
}

func TestResolveParent(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "task-1", ShortID: 1, Title: "First Task"},
			{ID: "task-2", ShortID: 2, Title: "Second Task"},
			{ID: "task-42", ShortID: 42, Title: "Answer to Everything"},
		},
	}

	tests := []struct {
		name    string
		ref     string
		wantID  string
		wantNil bool
	}{
		{
			name:   "by short ID 1",
			ref:    "1",
			wantID: "task-1",
		},
		{
			name:   "by short ID 42",
			ref:    "42",
			wantID: "task-42",
		},
		{
			name:   "by title prefix",
			ref:    "First",
			wantID: "task-1",
		},
		{
			name:   "by title prefix case insensitive",
			ref:    "second",
			wantID: "task-2",
		},
		{
			name:   "by partial title",
			ref:    "Answer",
			wantID: "task-42",
		},
		{
			name:    "not found numeric",
			ref:     "999",
			wantNil: true,
		},
		{
			name:    "not found text",
			ref:     "NonexistentTask",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveParent(tt.ref, project)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got task %q", result.ID)
				}
				return
			}

			if result == nil {
				t.Errorf("expected task %q, got nil", tt.wantID)
				return
			}

			if result.ID != tt.wantID {
				t.Errorf("got task ID %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

func TestParseDateIntegration(t *testing.T) {
	// Test that ParseDate works correctly for quick capture scenarios
	tests := []struct {
		input   string
		wantErr bool
	}{
		{"today", false},
		{"tomorrow", false},
		{"+3d", false},
		{"+1w", false},
		{"+2m", false},
		{"monday", false},
		{"fri", false},
		{"2025-12-31", false},
		{"12/31/2025", false},
		{"31.12.2025", false},
		{"next week", false},
		{"notadate", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			_, err := ParseDate(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDate(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestParseDateTomorrow(t *testing.T) {
	result, err := ParseDate("tomorrow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	now := time.Now()
	expected := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, 1)

	if !result.Equal(expected) {
		t.Errorf("ParseDate(tomorrow) = %v, want %v", result, expected)
	}
}

// Helper functions

func strPtr(s string) *string {
	return &s
}

func slicesEqual(a, b []string) bool {
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

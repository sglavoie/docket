package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

type quickCaptureParseTestCase struct {
	name           string
	input          string
	wantTitle      string
	wantPriority   int
	wantTags       []string
	wantHasDueDate bool
	wantParentID   *string
	wantTaskType   models.TaskType
	wantSection    string
	wantHasRecur   bool
	wantState      models.WorkflowState
	wantStatus     models.WorkflowState
	wantErr        bool
	errContains    string
}

func strPtr(s string) *string {
	return &s
}

func quickCaptureTestProject() *models.Project {
	return &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "epic-1", ShortID: 1, Type: models.TaskTypeEpic, Title: "User Authentication"},
			{ID: "story-2", ShortID: 2, Type: models.TaskTypeStory, Title: "Login Flow"},
			{ID: "task-3", ShortID: 3, Type: models.TaskTypeTask, Title: "Implement button"},
			{ID: "task-4", ShortID: 4, Type: models.TaskTypeTask, Title: "Implement search"},
		},
	}
}

func runQuickCaptureParseTests(t *testing.T, tests []quickCaptureParseTestCase) {
	t.Helper()

	project := quickCaptureTestProject()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuickCapture(tt.input, project, nil, false)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
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

			if !slices.Equal(result.tags, tt.wantTags) {
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

			if tt.wantTaskType != "" && result.taskType != tt.wantTaskType {
				t.Errorf("taskType = %q, want %q", result.taskType, tt.wantTaskType)
			}

			hasRecur := result.recurrence != nil
			if hasRecur != tt.wantHasRecur {
				t.Errorf("hasRecurrence = %v, want %v", hasRecur, tt.wantHasRecur)
			}

			if tt.wantState != "" {
				if !result.stateSet || result.state != tt.wantState {
					t.Errorf("state = %q (set=%v), want %q", result.state, result.stateSet, tt.wantState)
				}
			}

		})
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func TestCrossProjectCapture(t *testing.T) {
	currentProject := &models.Project{
		ID:   "current-project",
		Slug: "current-project",
		Name: "Current Project",
	}
	targetProject := &models.Project{
		ID:   "other-project",
		Slug: "other-project",
		Name: "Other Project",
	}
	allProjects := []*models.Project{currentProject, targetProject}

	tests := []struct {
		name           string
		input          string
		wantTitle      string
		wantTargetID   *string
		wantTargetName string
		wantErr        bool
		errContains    string
	}{
		{
			name:           "no target project",
			input:          "Simple task",
			wantTitle:      "Simple task",
			wantTargetID:   nil,
			wantTargetName: "",
		},
		{
			name:           "target project by slug",
			input:          "Task for other %other-project",
			wantTitle:      "Task for other",
			wantTargetID:   strPtr("other-project"),
			wantTargetName: "Other Project",
		},
		{
			name:           "target project by fuzzy name",
			input:          "%other Task in other project",
			wantTitle:      "Task in other project",
			wantTargetID:   strPtr("other-project"),
			wantTargetName: "Other Project",
		},
		{
			name:        "nonexistent project",
			input:       "%nonexistent Some task",
			wantErr:     true,
			errContains: "project not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseQuickCapture(tt.input, currentProject, allProjects)

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

			if tt.wantTargetID == nil {
				if result.targetProjectID != nil {
					t.Errorf("targetProjectID = %q, want nil", *result.targetProjectID)
				}
			} else {
				if result.targetProjectID == nil {
					t.Errorf("targetProjectID = nil, want %q", *tt.wantTargetID)
				} else if *result.targetProjectID != *tt.wantTargetID {
					t.Errorf("targetProjectID = %q, want %q", *result.targetProjectID, *tt.wantTargetID)
				}
			}

			if result.targetProjectName != tt.wantTargetName {
				t.Errorf("targetProjectName = %q, want %q", result.targetProjectName, tt.wantTargetName)
			}
		})
	}
}

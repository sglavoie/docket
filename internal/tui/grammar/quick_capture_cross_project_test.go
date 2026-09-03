package grammar

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
		ID:    "other-project",
		Slug:  "other-project",
		Name:  "Other Project",
		Tasks: []*models.Task{{ID: "other-parent", ShortID: 1, Title: "Other Parent"}},
	}
	allProjects := []*models.Project{currentProject, targetProject}

	tests := []struct {
		name           string
		input          string
		wantTitle      string
		wantTargetID   *string
		wantTargetName string
		wantParentID   *string
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
			name:        "target project requires exact slug",
			input:       "%other Task in other project",
			wantErr:     true,
			errContains: "exact %project-slug",
		},
		{
			name:           "project resolves before earlier parent token",
			input:          "Routed child >1 %other-project",
			wantTitle:      "Routed child",
			wantTargetID:   strPtr("other-project"),
			wantTargetName: "Other Project",
			wantParentID:   strPtr("other-parent"),
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
			result, err := ParseQuickCapture(tt.input, currentProject, allProjects, false)

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

			if result.Title != tt.wantTitle {
				t.Errorf("title = %q, want %q", result.Title, tt.wantTitle)
			}

			if tt.wantTargetID == nil {
				if result.TargetProjectID != nil {
					t.Errorf("targetProjectID = %q, want nil", *result.TargetProjectID)
				}
			} else {
				if result.TargetProjectID == nil {
					t.Errorf("targetProjectID = nil, want %q", *tt.wantTargetID)
				} else if *result.TargetProjectID != *tt.wantTargetID {
					t.Errorf("targetProjectID = %q, want %q", *result.TargetProjectID, *tt.wantTargetID)
				}
			}

			if result.targetProjectName != tt.wantTargetName {
				t.Errorf("targetProjectName = %q, want %q", result.targetProjectName, tt.wantTargetName)
			}
			if tt.wantParentID != nil && (result.ParentID == nil || *result.ParentID != *tt.wantParentID) {
				t.Errorf("parentID = %v, want %q", result.ParentID, *tt.wantParentID)
			}
		})
	}
}

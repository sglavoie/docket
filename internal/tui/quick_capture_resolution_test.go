package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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
		{name: "by short ID 1", ref: "1", wantID: "task-1"},
		{name: "by short ID 42", ref: "42", wantID: "task-42"},
		{name: "by title prefix", ref: "First", wantID: "task-1"},
		{name: "by title prefix case insensitive", ref: "second", wantID: "task-2"},
		{name: "by partial title", ref: "Answer", wantID: "task-42"},
		{name: "not found numeric", ref: "999", wantNil: true},
		{name: "not found text", ref: "NonexistentTask", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := resolveParent(tt.ref, project)
			if err != nil {
				t.Fatalf("resolveParent: %v", err)
			}

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

func TestResolveProject(t *testing.T) {
	projects := []*models.Project{
		{ID: "proj-1", Slug: "project-alpha", Name: "Project Alpha"},
		{ID: "proj-2", Slug: "my-app", Name: "My Application"},
		{ID: "proj-3", Slug: "archived-proj", Name: "Old Project", Archived: true},
	}

	tests := []struct {
		name    string
		ref     string
		wantID  string
		wantNil bool
	}{
		{name: "exact slug match", ref: "project-alpha", wantID: "proj-1"},
		{name: "slug lookup requires canonical spelling", ref: "Project-Alpha", wantNil: true},
		{name: "partial name is not a slug", ref: "alpha", wantNil: true},
		{name: "fuzzy text is not a slug", ref: "myapp", wantNil: true},
		{name: "archived project should not match", ref: "archived-proj", wantNil: true},
		{name: "nonexistent project", ref: "doesnotexist", wantNil: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveProject(tt.ref, projects)

			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got project %q", result.ID)
				}
				return
			}

			if result == nil {
				t.Errorf("expected project %q, got nil", tt.wantID)
				return
			}

			if result.ID != tt.wantID {
				t.Errorf("got project ID %q, want %q", result.ID, tt.wantID)
			}
		})
	}
}

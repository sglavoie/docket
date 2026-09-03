package grammar

import (
	"strings"
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

func TestStructuralReferencesShareMatchingAndNameTheirRelationship(t *testing.T) {
	project := &models.Project{
		ID:   "test-project",
		Name: "Test Project",
		Tasks: []*models.Task{
			{ID: "task-1", ShortID: 1, Title: "Login flow"},
			{ID: "task-2", ShortID: 2, Title: "Login form"},
		},
	}

	for _, relationship := range []string{"parent", "child"} {
		byShortID, err := ResolveStructuralReference("2", project, relationship)
		if err != nil || byShortID == nil || byShortID.ID != "task-2" {
			t.Errorf("%s reference 2 = %v, %v; want task-2", relationship, byShortID, err)
		}
		byPrefix, err := ResolveStructuralReference("login fl", project, relationship)
		if err != nil || byPrefix == nil || byPrefix.ID != "task-1" {
			t.Errorf("%s reference 'login fl' = %v, %v; want task-1", relationship, byPrefix, err)
		}

		task, err := ResolveStructuralReference("Login", project, relationship)
		if task != nil || err == nil {
			t.Fatalf("%s reference Login = %v, %v; want an ambiguity error", relationship, task, err)
		}
		if !strings.Contains(err.Error(), relationship+" reference") {
			t.Errorf("%s ambiguity error %q does not name the relationship", relationship, err)
		}
		for _, label := range []string{"#1 Login flow", "#2 Login form"} {
			if !strings.Contains(err.Error(), label) {
				t.Errorf("%s ambiguity error %q is missing %q", relationship, err, label)
			}
		}
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

package models

import "testing"

func TestNewProject(t *testing.T) {
	project := NewProject("test-id", "My Project", "/tmp/my-project")

	if project.ID != "test-id" {
		t.Errorf("expected ID 'test-id', got '%s'", project.ID)
	}
	if project.Slug != "my-project" {
		t.Errorf("expected Slug 'my-project', got '%s'", project.Slug)
	}
	if project.Name != "My Project" {
		t.Errorf("expected Name 'My Project', got '%s'", project.Name)
	}
	if project.Path != "/tmp/my-project" {
		t.Errorf("expected Path '/tmp/my-project', got '%s'", project.Path)
	}
	if project.Context != "personal" {
		t.Errorf("expected Context 'personal', got '%s'", project.Context)
	}
	if project.Archived {
		t.Error("expected Archived to be false")
	}
	if project.NextShortID != 1 {
		t.Errorf("expected NextShortID 1, got %d", project.NextShortID)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Project", "my-project"},
		{"UPPERCASE", "uppercase"},
		{"with spaces", "with-spaces"},
		{"special!@#chars", "specialchars"},
		{"multiple   spaces", "multiple-spaces"},
		{"---dashes---", "dashes"},
		{"MixedCase123", "mixedcase123"},
		{"already-slugified", "already-slugified"},
		{"", ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := Slugify(tc.input)
			if got != tc.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tc.input, got, tc.expected)
			}
		})
	}
}

func TestProjectArchive(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	if project.Archived {
		t.Error("project should not be archived initially")
	}
	if project.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil initially")
	}

	project.Archive()

	if !project.Archived {
		t.Error("project should be archived after Archive()")
	}
	if project.ArchivedAt == nil {
		t.Error("ArchivedAt should be set after Archive()")
	}

	project.Unarchive()

	if project.Archived {
		t.Error("project should not be archived after Unarchive()")
	}
	if project.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil after Unarchive()")
	}
}

func TestNewProjectHasNextSection(t *testing.T) {
	project := NewProject("test", "Test", "/tmp/test")

	if len(project.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(project.Sections))
	}
	if project.Sections[0] != SectionNext {
		t.Errorf("expected first section to be Next, got %s", project.Sections[0])
	}
}

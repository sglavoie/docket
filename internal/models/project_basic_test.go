package models

import "testing"

func TestNewProject(t *testing.T) {
	project, err := NewProject("test-id", "My Project", "/tmp/my-project")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	if len(project.Sections) != 1 {
		t.Errorf("expected 1 section, got %d", len(project.Sections))
	}
	if project.Sections[0] != SectionNext {
		t.Errorf("expected first section to be Next, got %s", project.Sections[0])
	}
}

func TestMutateTask_NotFound(t *testing.T) {
	project, err := NewProject("test", "Test", "/tmp/test")
	if err != nil {
		t.Fatal(err)
	}

	task, err := NewTask("task-1", 1, TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = []*Task{task}

	// Mutate with nonexistent ID should return false
	found := project.MutateTask("nonexistent-id", func(t *Task) {
		t.Title = "Should not change"
	})
	if found {
		t.Error("MutateTask should return false for nonexistent task ID")
	}

	// Verify existing task was not modified
	if task.Title != "Task 1" {
		t.Errorf("expected title 'Task 1', got '%s'", task.Title)
	}

	// Mutate with existing ID should return true
	found = project.MutateTask(task.ID, func(t *Task) {
		t.Title = "Updated Title"
	})
	if !found {
		t.Error("MutateTask should return true for existing task ID")
	}
	if task.Title != "Updated Title" {
		t.Errorf("expected title 'Updated Title', got '%s'", task.Title)
	}
}

func TestNewProject_EmptyIDReturnsError(t *testing.T) {
	_, err := NewProject("", "My Project", "/path")
	if err == nil {
		t.Error("expected error for empty project ID, got nil")
	}
}

func TestNewProject_WhitespaceIDReturnsError(t *testing.T) {
	_, err := NewProject("   ", "My Project", "/path")
	if err == nil {
		t.Error("expected error for whitespace-only project ID, got nil")
	}
}

func TestNewProject_EmptyNameReturnsError(t *testing.T) {
	_, err := NewProject("my-id", "", "/path")
	if err == nil {
		t.Error("expected error for empty project name, got nil")
	}
}

func TestNewProject_WhitespaceNameReturnsError(t *testing.T) {
	_, err := NewProject("my-id", "   ", "/path")
	if err == nil {
		t.Error("expected error for whitespace-only project name, got nil")
	}
}

func TestNewProject_ValidInputSucceeds(t *testing.T) {
	p, err := NewProject("valid-id", "Valid Name", "/some/path")
	if err != nil {
		t.Fatalf("expected no error for valid input, got: %v", err)
	}
	if p.ID != "valid-id" {
		t.Errorf("ID = %q, want %q", p.ID, "valid-id")
	}
	if p.Name != "Valid Name" {
		t.Errorf("Name = %q, want %q", p.Name, "Valid Name")
	}
}

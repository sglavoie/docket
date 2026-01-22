package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple name", "My Project", "my-project"},
		{"already slug", "my-project", "my-project"},
		{"uppercase", "MY PROJECT", "my-project"},
		{"special chars", "My Project! @#$%", "my-project"},
		{"multiple spaces", "My   Project", "my-project"},
		{"trailing spaces", "  My Project  ", "my-project"},
		{"numbers", "Project 123", "project-123"},
		{"unicode chars", "Café Project", "caf-project"},
		{"dashes preserved", "my-cool-project", "my-cool-project"},
		{"mixed", "My Cool Project 2024!", "my-cool-project-2024"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := models.Slugify(tt.input)
			if got != tt.expected {
				t.Errorf("Slugify(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewProject(t *testing.T) {
	project := models.NewProject("test-id", "Test Project", "/path/to/project")

	testutil.AssertEqual(t, project.ID, "test-id")
	testutil.AssertEqual(t, project.Name, "Test Project")
	testutil.AssertEqual(t, project.Path, "/path/to/project")
	testutil.AssertEqual(t, project.NextShortID, 1)
	testutil.AssertEqual(t, project.Archived, false)

	if project.CreatedAt.IsZero() {
		t.Error("CreatedAt should be set")
	}
	if project.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should be set")
	}
}

func TestProjectContextDefault(t *testing.T) {
	project := models.NewProject("test-id", "Test Project", "/path")

	// Context defaults to "personal"
	testutil.AssertEqual(t, project.Context, "personal")
}

func TestProjectDescriptionDefault(t *testing.T) {
	project := models.NewProject("test-id", "Test Project", "/path")

	testutil.AssertEqual(t, project.Description, "")
}

func TestDuplicateSlugDetection(t *testing.T) {
	cfg, _ := testutil.NewTestConfigWithProject("Existing Project")

	// Slugify the existing project
	cfg.Projects[0].Slug = models.Slugify("Existing Project")

	// Check if a new project with the same slug would be detected
	newSlug := models.Slugify("Existing Project")
	for _, p := range cfg.Projects {
		if p.Slug == newSlug {
			return // Test passes - duplicate detected
		}
	}
	t.Error("duplicate slug should have been detected")
}

func TestDuplicateSlugWithDifferentCasing(t *testing.T) {
	cfg, _ := testutil.NewTestConfigWithProject("my project")
	cfg.Projects[0].Slug = models.Slugify("my project")

	// A project with different casing should produce the same slug
	slug1 := models.Slugify("my project")
	slug2 := models.Slugify("My Project")
	slug3 := models.Slugify("MY PROJECT")

	testutil.AssertEqual(t, slug1, slug2)
	testutil.AssertEqual(t, slug2, slug3)

	// Check duplicate detection
	for _, p := range cfg.Projects {
		if p.Slug == slug2 {
			return // Duplicate detected
		}
	}
	t.Error("duplicate slug should have been detected for different casing")
}

func TestProjectSlugUniqueness(t *testing.T) {
	cfg := testutil.NewTestConfig()

	// Add two projects with different names but same slug after processing
	project1 := testutil.NewTestProject("My Project!!!")
	project1.Slug = models.Slugify(project1.Name)

	cfg.Projects = append(cfg.Projects, project1)

	// Verify a new project with similar name would have same slug
	newSlug := models.Slugify("My Project")
	testutil.AssertEqual(t, project1.Slug, newSlug)
}

func TestProjectSections(t *testing.T) {
	project := models.NewProject("test-id", "Test", "/path")

	// Initially has Backlog section by default
	testutil.AssertEqual(t, len(project.Sections), 1)
	if !project.HasSection(models.SectionBacklog) {
		t.Error("project should have default Backlog section")
	}

	// Ensure a new section
	project.EnsureSection("Active")
	if !project.HasSection("Active") {
		t.Error("project should have 'Active' section")
	}

	// Ensure same section again - should not duplicate
	project.EnsureSection("Active")
	activeCount := 0
	for _, s := range project.Sections {
		if s == "Active" {
			activeCount++
		}
	}
	if activeCount != 1 {
		t.Errorf("expected 1 'Active' section, got %d", activeCount)
	}
}

func TestProjectAddTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Add a task
	task := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
	project.Tasks = append(project.Tasks, task)

	testutil.AssertTaskCount(t, project, 1)
	testutil.AssertEqual(t, project.Tasks[0].Title, "Test Task")
}

func TestProjectRemoveTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Add tasks
	task1 := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	task2 := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	project.Tasks = append(project.Tasks, task1, task2)
	project.BuildTaskTree()

	testutil.AssertTaskCount(t, project, 2)

	// Remove task1 by filtering the slice (standard pattern in codebase)
	for i, t := range project.Tasks {
		if t.ID == task1.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	testutil.AssertTaskCount(t, project, 1)
	testutil.AssertEqual(t, project.Tasks[0].ID, task2.ID)
}

func TestProjectGetTaskByID(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task := models.NewTask("unique-id", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
	project.Tasks = append(project.Tasks, task)

	found := project.GetTaskByID("unique-id")
	if found == nil {
		t.Fatal("task should be found by ID")
	}
	testutil.AssertEqual(t, found.Title, "Test Task")

	// Non-existent ID
	notFound := project.GetTaskByID("nonexistent")
	if notFound != nil {
		t.Error("non-existent ID should return nil")
	}
}

func TestProjectGetTaskByShortID(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task := models.NewTask("some-id", 42, models.TaskTypeTask, "Test Task")
	project.Tasks = append(project.Tasks, task)

	found := project.GetTaskByShortID(42)
	if found == nil {
		t.Fatal("task should be found by short ID")
	}
	testutil.AssertEqual(t, found.Title, "Test Task")

	// Non-existent short ID
	notFound := project.GetTaskByShortID(999)
	if notFound != nil {
		t.Error("non-existent short ID should return nil")
	}
}

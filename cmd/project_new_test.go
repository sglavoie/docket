package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func newProjectNewCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringVarP(&projectContext, "context", "c", "personal", "")
	cmd.Flags().StringVarP(&projectDescription, "description", "d", "", "")
	cmd.Flags().StringVarP(&projectStatusIcon, "status-icon", "i", "", "")
	addOutputFlags(cmd)
	return cmd
}

func resetProjectNewVars() {
	projectContext = "personal"
	projectDescription = ""
	projectStatusIcon = ""
}

// TestProjectNew_TrimsContextBeforeStoring verifies that a whitespace-padded
// --context value is trimmed before being assigned to the project. Without
// trimming, "  work  " is stored verbatim, which makes isKnownContext lookups
// fail elsewhere and pollutes the JSON config with stray spaces.
func TestProjectNew_TrimsContextBeforeStoring(t *testing.T) {
	deps := setupTestDeps(t)
	defer deps.Cleanup()
	defer resetProjectNewVars()

	viper.Set("config", deps.ConfigPath)
	defer viper.Set("config", "")

	manager := config.NewManager(deps.ConfigPath)
	if err := saveTestRuntime(manager, deps.Config); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cmd := newProjectNewCmd()
	if err := cmd.Flags().Set("context", "  work  "); err != nil {
		t.Fatalf("set context flag: %v", err)
	}
	if err := cmd.Flags().Set("quiet", "true"); err != nil {
		t.Fatalf("set quiet flag: %v", err)
	}

	if err := runProjectNew(cmd, []string{"Trimmable"}); err != nil {
		t.Fatalf("runProjectNew: %v", err)
	}

	reloaded, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	var got *models.Project
	for _, p := range reloaded.AllProjects() {
		if p.Slug == "trimmable" {
			got = p
			break
		}
	}
	if got == nil {
		t.Fatal("project not found in reloaded config")
	}
	if got.Context != "work" {
		t.Errorf("project.Context = %q, want %q (trimmed)", got.Context, "work")
	}
}

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
	project, err := models.NewProject("test-id", "Test Project", "/path/to/project")
	if err != nil {
		t.Fatal(err)
	}

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
	project, err := models.NewProject("test-id", "Test Project", "/path")
	if err != nil {
		t.Fatal(err)
	}

	// Context defaults to "personal"
	testutil.AssertEqual(t, project.Context, "personal")
}

func TestProjectDescriptionDefault(t *testing.T) {
	project, err := models.NewProject("test-id", "Test Project", "/path")
	if err != nil {
		t.Fatal(err)
	}

	testutil.AssertEqual(t, project.Description, "")
}

func TestDuplicateSlugDetection(t *testing.T) {
	cfg, _ := newTestConfigWithProject("Existing Project")

	// Slugify the existing project
	cfg.AllProjects()[0].Slug = models.Slugify("Existing Project")

	// Check if a new project with the same slug would be detected
	newSlug := models.Slugify("Existing Project")
	for _, p := range cfg.AllProjects() {
		if p.Slug == newSlug {
			return // Test passes - duplicate detected
		}
	}
	t.Error("duplicate slug should have been detected")
}

func TestDuplicateSlugWithDifferentCasing(t *testing.T) {
	cfg, _ := newTestConfigWithProject("my project")
	cfg.AllProjects()[0].Slug = models.Slugify("my project")

	// A project with different casing should produce the same slug
	slug1 := models.Slugify("my project")
	slug2 := models.Slugify("My Project")
	slug3 := models.Slugify("MY PROJECT")

	testutil.AssertEqual(t, slug1, slug2)
	testutil.AssertEqual(t, slug2, slug3)

	// Check duplicate detection
	for _, p := range cfg.AllProjects() {
		if p.Slug == slug2 {
			return // Duplicate detected
		}
	}
	t.Error("duplicate slug should have been detected for different casing")
}

func TestProjectSlugUniqueness(t *testing.T) {
	cfg := newTestConfig()

	// Add two projects with different names but same slug after processing
	project1 := testutil.NewTestProject("My Project!!!")
	project1.Slug = models.Slugify(project1.Name)

	addTestProject(cfg, project1)

	// Verify a new project with similar name would have same slug
	newSlug := models.Slugify("My Project")
	testutil.AssertEqual(t, project1.Slug, newSlug)
}

func TestProjectSections(t *testing.T) {
	project, err := models.NewProject("test-id", "Test", "/path")
	if err != nil {
		t.Fatal(err)
	}

	// Initially has Next section by default
	testutil.AssertEqual(t, len(project.Sections), 1)
	if !project.HasSection(models.SectionNext) {
		t.Error("project should have default Next section")
	}

	// Ensure a built-in section
	project.EnsureSection(models.SectionInbox)
	if !project.HasSection(models.SectionInbox) {
		t.Error("project should have Inbox section")
	}

	// Ensure same section again - should not duplicate
	project.EnsureSection(models.SectionInbox)
	inboxCount := 0
	for _, s := range project.Sections {
		if s == models.SectionInbox {
			inboxCount++
		}
	}
	if inboxCount != 1 {
		t.Errorf("expected 1 Inbox section, got %d", inboxCount)
	}
}

func TestProjectAddTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Add a task
	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)

	testutil.AssertTaskCount(t, project, 1)
	testutil.AssertEqual(t, project.Tasks[0].Title, "Test Task")
}

func TestProjectRemoveTask(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Add tasks
	task1, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task2, err := models.NewTask("task-2", project.GetNextShortID(), models.TaskTypeTask, "Task 2")
	if err != nil {
		t.Fatal(err)
	}
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

	task, err := models.NewTask("unique-id", project.GetNextShortID(), models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
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

// TestProjectNewTrimsContext verifies that whitespace around the --context flag
// value is stripped before the value is persisted on the project. Previously a
// value like " personal " would be stored verbatim and fail the
// `isKnownContext` check, producing a bogus warning and an untrimmed
// project.Context.
func TestProjectNewTrimsContext(t *testing.T) {
	deps := setupTestDeps(t)
	t.Cleanup(deps.Cleanup)

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	oldContext := projectContext
	oldDescription := projectDescription
	oldStatusIcon := projectStatusIcon
	t.Cleanup(func() {
		projectContext = oldContext
		projectDescription = oldDescription
		projectStatusIcon = oldStatusIcon
	})

	projectContext = "  personal  "
	projectDescription = ""
	projectStatusIcon = ""

	err := runProjectNew(nil, []string{"Trim Context Project"})
	testutil.AssertNoError(t, err)

	cfg, err := manager.LoadRuntimeState()
	testutil.AssertNoError(t, err)

	var created *models.Project
	for _, p := range cfg.AllProjects() {
		if p.Slug == "trim-context-project" {
			created = p
			break
		}
	}
	if created == nil {
		t.Fatal("expected project 'trim-context-project' to be created")
	}
	if created.Context != "personal" {
		t.Errorf("Context = %q, want %q", created.Context, "personal")
	}
}

func TestProjectGetTaskByShortID(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	task, err := models.NewTask("some-id", 42, models.TaskTypeTask, "Test Task")
	if err != nil {
		t.Fatal(err)
	}
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

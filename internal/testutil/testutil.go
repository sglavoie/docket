package testutil

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// NewTestProject creates a project with sample tasks for testing.
func NewTestProject(name string) *models.Project {
	project := models.NewProject(uuid.New().String(), name, "")
	project.Slug = models.Slugify(name)
	return project
}

// addTask is a helper to add a task to a project and increment shortID.
func addTask(project *models.Project, task *models.Task) {
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
}

// NewTestProjectWithTasks creates a project with a variety of sample tasks.
func NewTestProjectWithTasks(name string) *models.Project {
	project := NewTestProject(name)

	// Add an epic
	epic := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeEpic, "Setup Infrastructure")
	epic.Description = "Set up the development infrastructure"
	epic.Priority = 1
	addTask(project, epic)

	// Add a story under the epic
	story := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeStory, "User Authentication")
	story.Description = "Implement user login and registration"
	story.Priority = 2
	story.ParentID = &epic.ID
	addTask(project, story)

	// Add tasks under the story
	task1 := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Create login form")
	task1.ParentID = &story.ID
	task1.Status = models.StatusInProgress
	addTask(project, task1)

	task2 := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Add password validation")
	task2.ParentID = &story.ID
	task2.Status = models.StatusTodo
	addTask(project, task2)

	// Add a completed task
	task3 := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Design login page mockup")
	task3.ParentID = &story.ID
	task3.Status = models.StatusDone
	now := time.Now()
	task3.CompletedAt = &now
	addTask(project, task3)

	// Add a bug
	bug := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeBug, "Fix login redirect")
	bug.Priority = 1
	bug.Tags = []string{"urgent", "auth"}
	addTask(project, bug)

	// Add an idea
	idea := models.NewTask(uuid.New().String(), project.NextShortID, models.TaskTypeIdea, "Add social login")
	idea.Description = "Consider adding Google/GitHub OAuth"
	addTask(project, idea)

	// Rebuild tree to establish parent-child relationships
	project.BuildTaskTree()

	return project
}

// NewTestConfig creates a config with test defaults.
func NewTestConfig() *config.Config {
	return &config.Config{
		DataDir:        "",
		CurrentProject: "",
		DefaultContext: "personal",
		FzfEnabled:     false,
		GlowEnabled:    false,
		Editor:         "code",
	}
}

// NewTestConfigWithProject creates a config with a single test project.
func NewTestConfigWithProject(projectName string) (*config.Config, *models.Project) {
	cfg := NewTestConfig()
	project := NewTestProject(projectName)
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID
	return cfg, project
}

// TempProjectDir creates a temporary directory for test project files.
// Returns the directory path and a cleanup function.
func TempProjectDir(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "tt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	return dir, cleanup
}

// TempConfigFile creates a temporary config file for testing.
// Returns the file path and a cleanup function.
func TempConfigFile(t *testing.T) (string, func()) {
	t.Helper()

	dir, cleanup := TempProjectDir(t)
	configPath := filepath.Join(dir, "tt.json")

	return configPath, cleanup
}

// NewTestTask creates a simple task for testing.
func NewTestTask(title string, taskType models.TaskType) *models.Task {
	return models.NewTask(uuid.New().String(), 1, taskType, title)
}

// NewTestTaskWithStatus creates a task with a specific status.
func NewTestTaskWithStatus(title string, status models.TaskStatus) *models.Task {
	task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	task.Status = status
	if status == models.StatusDone {
		now := time.Now()
		task.CompletedAt = &now
	}
	return task
}

// NewTestTaskWithTags creates a task with specific tags.
func NewTestTaskWithTags(title string, tags ...string) *models.Task {
	task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	task.Tags = tags
	return task
}

// NewTestTaskWithPriority creates a task with a specific priority.
func NewTestTaskWithPriority(title string, priority int) *models.Task {
	task := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	task.Priority = priority
	return task
}

// AssertTaskCount checks that a project has the expected number of tasks.
func AssertTaskCount(t *testing.T, project *models.Project, expected int) {
	t.Helper()
	if got := len(project.Tasks); got != expected {
		t.Errorf("expected %d tasks, got %d", expected, got)
	}
}

// AssertTaskStatus checks that a task has the expected status.
func AssertTaskStatus(t *testing.T, task *models.Task, expected models.TaskStatus) {
	t.Helper()
	if task.Status != expected {
		t.Errorf("expected status %s, got %s", expected, task.Status)
	}
}

// AssertNoError fails the test if err is not nil.
func AssertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// AssertError fails the test if err is nil.
func AssertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
}

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

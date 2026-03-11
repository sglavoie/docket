package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/external"
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

// mustNewTask is a helper that panics on error (for non-test-context helpers).
func mustNewTask(id string, shortID int, taskType models.TaskType, title string) *models.Task {
	task, err := models.NewTask(id, shortID, taskType, title)
	if err != nil {
		panic(fmt.Sprintf("mustNewTask(%q): %v", title, err))
	}
	return task
}

// NewTestProjectWithTasks creates a project with a variety of sample tasks.
func NewTestProjectWithTasks(name string) *models.Project {
	project := NewTestProject(name)

	// Add an epic
	epic := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeEpic, "Setup Infrastructure")
	epic.Description = "Set up the development infrastructure"
	epic.Priority = 1
	addTask(project, epic)

	// Add a story under the epic
	story := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeStory, "User Authentication")
	story.Description = "Implement user login and registration"
	story.Priority = 2
	story.ParentID = &epic.ID
	addTask(project, story)

	// Add tasks under the story
	task1 := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Create login form")
	task1.ParentID = &story.ID
	task1.Status = models.StatusInProgress
	addTask(project, task1)

	task2 := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Add password validation")
	task2.ParentID = &story.ID
	task2.Status = models.StatusTodo
	addTask(project, task2)

	// Add a completed task
	task3 := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeTask, "Design login page mockup")
	task3.ParentID = &story.ID
	task3.Status = models.StatusDone
	now := time.Now()
	task3.CompletedAt = &now
	addTask(project, task3)

	// Add a bug
	bug := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeBug, "Fix login redirect")
	bug.Priority = 1
	bug.Tags = []string{"urgent", "auth"}
	addTask(project, bug)

	// Add an idea
	idea := mustNewTask(uuid.New().String(), project.NextShortID, models.TaskTypeIdea, "Add social login")
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
		_ = os.RemoveAll(dir)
	}

	return dir, cleanup
}

// NewTestTask creates a simple task for testing.
func NewTestTask(title string, taskType models.TaskType) *models.Task {
	task, err := models.NewTask(uuid.New().String(), 1, taskType, title)
	if err != nil {
		panic(fmt.Sprintf("NewTestTask(%q): %v", title, err))
	}
	return task
}

// NewTestTaskWithStatus creates a task with a specific status.
func NewTestTaskWithStatus(title string, status models.TaskStatus) *models.Task {
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		panic(fmt.Sprintf("NewTestTaskWithStatus(%q): %v", title, err))
	}
	task.Status = status
	if status == models.StatusDone {
		now := time.Now()
		task.CompletedAt = &now
	}
	return task
}

// NewTestTaskWithPriority creates a task with a specific priority.
func NewTestTaskWithPriority(title string, priority int) *models.Task {
	task, err := models.NewTask(uuid.New().String(), 1, models.TaskTypeTask, title)
	if err != nil {
		panic(fmt.Sprintf("NewTestTaskWithPriority(%q): %v", title, err))
	}
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

// AssertEqual fails the test if got != want.
func AssertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

// TestDeps holds mock dependencies for testing cmd package.
// Use SetupTestDeps to create this struct with all mocks initialized.
type TestDeps struct {
	Selector   *MockSelector
	Renderer   *MockRenderer
	Editor     *MockEditor
	Repository *MockRepository
	Config     *config.Config
	ConfigPath string
	Cleanup    func()
}

// SetupTestDeps creates a complete test environment with mock dependencies.
// It creates a temporary directory for config and project files.
// Call Cleanup() when done to remove temp files and restore external defaults.
// Note: Tests must set storage.DefaultRepository themselves to avoid import cycles.
func SetupTestDeps(t *testing.T) *TestDeps {
	t.Helper()

	// Create temp directory for test files
	tempDir, err := os.MkdirTemp("", "tt-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	configPath := filepath.Join(tempDir, "config.json")

	// Store original defaults to restore later
	origSelector := external.DefaultSelector
	origRenderer := external.DefaultRenderer
	origEditor := external.DefaultEditor

	// Create mocks
	selector := &MockSelector{Available_: true}
	renderer := &MockRenderer{Available_: true}
	editor := &MockEditor{Available_: true}
	repository := NewMockRepository()

	// Replace defaults with mocks
	external.DefaultSelector = selector
	external.DefaultRenderer = renderer
	external.DefaultEditor = editor

	// Create test config
	cfg := NewTestConfig()
	cfg.DataDir = tempDir

	cleanup := func() {
		// Restore original defaults
		external.DefaultSelector = origSelector
		external.DefaultRenderer = origRenderer
		external.DefaultEditor = origEditor
		// Remove temp directory
		_ = os.RemoveAll(tempDir)
	}

	return &TestDeps{
		Selector:   selector,
		Renderer:   renderer,
		Editor:     editor,
		Repository: repository,
		Config:     cfg,
		ConfigPath: configPath,
		Cleanup:    cleanup,
	}
}

// SetupTestDepsWithProject creates test deps and adds a project with tasks.
// Returns the TestDeps and the created project.
func SetupTestDepsWithProject(t *testing.T, projectName string) (*TestDeps, *models.Project) {
	t.Helper()

	deps := SetupTestDeps(t)

	project := NewTestProjectWithTasks(projectName)
	project.Path = deps.Config.GetProjectPath(project.Slug)
	deps.Config.Projects = []*models.Project{project}
	deps.Config.CurrentProject = project.ID
	deps.Repository.Projects[project.ID] = project

	return deps, project
}

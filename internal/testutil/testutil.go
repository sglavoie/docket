package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
)

// NewTestProject creates a project with sample tasks for testing.
func NewTestProject(name string) *models.Project {
	project, err := models.NewProject(uuid.New().String(), name, "")
	if err != nil {
		panic(err)
	}
	project.Slug = models.Slugify(name)
	return project
}

// addTask is a helper to add a task to a project and increment shortID.
func addTask(project *models.Project, task *models.Task) {
	project.Tasks = append(project.Tasks, task)
	project.NextShortID++
}

// mustNewTask is a helper that panics on error (for non-test-context helpers).
func mustNewTask(id string, shortID int, title string) *models.Task {
	task, err := models.NewGenericTask(id, shortID, title)
	if err != nil {
		panic(fmt.Sprintf("mustNewTask(%q): %v", title, err))
	}
	return task
}

// NewTestProjectWithTasks creates a project with a variety of sample tasks.
func NewTestProjectWithTasks(name string) *models.Project {
	project := NewTestProject(name)

	// Add an epic
	epic := mustNewTask(uuid.New().String(), project.NextShortID, "Setup Infrastructure")
	epic.Description = "Set up the development infrastructure"
	epic.Priority = 1
	addTask(project, epic)

	// Add a story under the epic
	story := mustNewTask(uuid.New().String(), project.NextShortID, "User Authentication")
	story.Description = "Implement user login and registration"
	story.Priority = 2
	story.ParentID = &epic.ID
	addTask(project, story)

	// Add tasks under the story
	task1 := mustNewTask(uuid.New().String(), project.NextShortID, "Create login form")
	task1.ParentID = &story.ID
	task1.WorkflowState = models.StateDoing
	addTask(project, task1)

	task2 := mustNewTask(uuid.New().String(), project.NextShortID, "Add password validation")
	task2.ParentID = &story.ID
	task2.WorkflowState = models.StateBacklog
	addTask(project, task2)

	// Add a completed task
	task3 := mustNewTask(uuid.New().String(), project.NextShortID, "Design login page mockup")
	task3.ParentID = &story.ID
	task3.WorkflowState = models.StateDone
	now := time.Now()
	task3.CompletedAt = &now
	addTask(project, task3)

	// Add a bug
	bug := mustNewTask(uuid.New().String(), project.NextShortID, "Fix login redirect")
	bug.Priority = 1
	bug.Tags = []string{"urgent", "auth"}
	addTask(project, bug)

	// Add an idea
	idea := mustNewTask(uuid.New().String(), project.NextShortID, "Add social login")
	idea.Description = "Consider adding Google/GitHub OAuth"
	addTask(project, idea)

	// Rebuild tree to establish parent-child relationships
	project.BuildTaskTree()

	return project
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
func NewTestTask(title string) *models.Task {
	task, err := models.NewGenericTask(uuid.New().String(), 1, title)
	if err != nil {
		panic(fmt.Sprintf("NewTestTask(%q): %v", title, err))
	}
	return task
}

// TestDeps holds mock external dependencies plus caller-owned test state. The
// state is generic so this support package does not depend on the application
// configuration facade; callers may supply domain fixtures, RuntimeState, or a
// compatibility value local to their _test.go package.
type TestDeps[T any] struct {
	Selector   *MockSelector
	Renderer   *MockRenderer
	Editor     *MockEditor
	Repository *MockRepository
	Config     T
	DataDir    string
	ConfigPath string
	Cleanup    func()
}

// SetupTestDeps creates a complete test environment with mock dependencies and
// caller-owned state rooted at the temporary data directory.
// It creates a temporary directory for config and project files.
// Cleanup is registered with t.Cleanup automatically; call Cleanup only when
// early cleanup is needed.
// Note: Tests must set storage.DefaultRepository themselves to avoid import cycles.
func SetupTestDeps[T any](t *testing.T, newState func(dataDir string) T) *TestDeps[T] {
	t.Helper()
	if newState == nil {
		t.Fatal("SetupTestDeps requires a state factory")
	}

	// Isolate machine-local state (~/.tt/runtime/<workspace-id>/machine.json,
	// keyed off $HOME) into a throwaway directory so
	// no test writes into the developer's real home directory once Manager
	// persistence begins touching that tier.
	t.Setenv("HOME", t.TempDir())

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

	state := newState(tempDir)

	var cleanupOnce sync.Once
	cleanup := func() {
		cleanupOnce.Do(func() {
			// Restore original defaults
			external.DefaultSelector = origSelector
			external.DefaultRenderer = origRenderer
			external.DefaultEditor = origEditor
			// Remove temp directory
			_ = os.RemoveAll(tempDir)
		})
	}
	t.Cleanup(cleanup)

	return &TestDeps[T]{
		Selector:   selector,
		Renderer:   renderer,
		Editor:     editor,
		Repository: repository,
		Config:     state,
		DataDir:    tempDir,
		ConfigPath: configPath,
		Cleanup:    cleanup,
	}
}

// SetupTestDepsWithProject creates test deps and supplies a rooted project to
// the caller-owned state factory before registering it with the mock repository.
// Returns the TestDeps and the created project.
func SetupTestDepsWithProject[T any](
	t *testing.T,
	projectName string,
	newState func(dataDir string, project *models.Project) T,
) (*TestDeps[T], *models.Project) {
	t.Helper()
	if newState == nil {
		t.Fatal("SetupTestDepsWithProject requires a state factory")
	}

	project := NewTestProjectWithTasks(projectName)
	deps := SetupTestDeps(t, func(dataDir string) T {
		project.Path = filepath.Join(dataDir, "projects", project.Slug)
		return newState(dataDir, project)
	})
	deps.Repository.Projects[project.ID] = project

	return deps, project
}

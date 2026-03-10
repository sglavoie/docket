package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestIntegration_CreateTaskAndComplete(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Integration Test")
	defer deps.Cleanup()

	// Replace cmd.Deps with our mocks
	origDeps := Deps
	Deps = Dependencies{
		Selector:   deps.Selector,
		Renderer:   deps.Renderer,
		Editor:     deps.Editor,
		Repository: deps.Repository,
	}
	defer func() { Deps = origDeps }()

	// Verify initial state
	initialCount := len(project.Tasks)
	if initialCount == 0 {
		t.Fatal("project should have initial tasks from NewTestProjectWithTasks")
	}

	// Create a new task
	newTask, err := models.NewTask("new-task-1", project.GetNextShortID(), models.TaskTypeTask, "New Integration Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, newTask)
	project.BuildTaskTree()

	// Verify task was added
	testutil.AssertEqual(t, len(project.Tasks), initialCount+1)

	// Find the task by short ID
	foundTask := project.GetTaskByShortID(newTask.ShortID)
	if foundTask == nil {
		t.Fatal("should find task by short ID")
	}
	testutil.AssertEqual(t, foundTask.Title, "New Integration Task")
	testutil.AssertEqual(t, foundTask.Status, models.StatusTodo)

	// Complete the task
	foundTask.Complete()
	testutil.AssertEqual(t, foundTask.Status, models.StatusDone)
	if foundTask.CompletedAt == nil {
		t.Error("CompletedAt should be set after completing task")
	}

	// Save to mock repository
	err = deps.Repository.Save(project)
	testutil.AssertNoError(t, err)

	// Verify project was saved
	savedProject, ok := deps.Repository.Projects[project.ID]
	if !ok {
		t.Fatal("project should be saved in repository")
	}
	testutil.AssertEqual(t, len(savedProject.Tasks), initialCount+1)
}

func TestIntegration_TaskHierarchy(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Hierarchy Test")
	defer deps.Cleanup()

	// Replace cmd.Deps with our mocks
	origDeps := Deps
	Deps = Dependencies{
		Selector:   deps.Selector,
		Renderer:   deps.Renderer,
		Editor:     deps.Editor,
		Repository: deps.Repository,
	}
	defer func() { Deps = origDeps }()

	// Create an epic
	epic, err := models.NewTask("epic-int-1", project.GetNextShortID(), models.TaskTypeEpic, "Integration Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	// Create a story under the epic
	story, err := models.NewTask("story-int-1", project.GetNextShortID(), models.TaskTypeStory, "Integration Story")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)

	// Create tasks under the story
	task1, err := models.NewTask("task-int-1", project.GetNextShortID(), models.TaskTypeTask, "Integration Task 1")
	if err != nil {
		t.Fatal(err)
	}
	task1.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task1)

	task2, err := models.NewTask("task-int-2", project.GetNextShortID(), models.TaskTypeTask, "Integration Task 2")
	if err != nil {
		t.Fatal(err)
	}
	task2.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task2)

	// Build the tree
	project.BuildTaskTree()

	// Verify hierarchy
	if epic.Parent != nil {
		t.Error("epic should not have a parent")
	}
	testutil.AssertEqual(t, len(epic.Children), 1)
	testutil.AssertEqual(t, epic.Children[0].ID, story.ID)

	testutil.AssertEqual(t, story.Parent.ID, epic.ID)
	testutil.AssertEqual(t, len(story.Children), 2)

	testutil.AssertEqual(t, task1.Parent.ID, story.ID)
	testutil.AssertEqual(t, task2.Parent.ID, story.ID)

	// Complete both child tasks
	task1.Complete()
	task2.Complete()

	// Verify children are complete
	testutil.AssertEqual(t, task1.Status, models.StatusDone)
	testutil.AssertEqual(t, task2.Status, models.StatusDone)

	// Complete the story
	story.Complete()
	testutil.AssertEqual(t, story.Status, models.StatusDone)
}

func TestIntegration_FindTaskByIdentifier(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Find Test")
	defer deps.Cleanup()

	// Add a task with known values
	task, err := models.NewTask("find-me-uuid", project.GetNextShortID(), models.TaskTypeTask, "Findable Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	tests := []struct {
		name       string
		identifier string
		wantFound  bool
	}{
		{"by short ID", "100", true}, // Use a high number that won't conflict
		{"by UUID", "find-me-uuid", true},
		{"by title prefix", "Findable", true},
		{"by title prefix lowercase", "findable", true},
		{"not found", "nonexistent-xyz", false},
	}

	// Update task short ID for the test
	task.ShortID = 100

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found := FindTaskByIdentifier(project, tt.identifier)
			if tt.wantFound && found == nil {
				t.Errorf("expected to find task with identifier %q", tt.identifier)
			}
			if !tt.wantFound && found != nil {
				t.Errorf("did not expect to find task with identifier %q", tt.identifier)
			}
			if tt.wantFound && found != nil && found.ID != task.ID {
				t.Errorf("found wrong task: got %s, want %s", found.ID, task.ID)
			}
		})
	}
}

func TestIntegration_MockSelectorBehavior(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Selector Test")
	defer deps.Cleanup()

	// Configure mock to return a specific task
	targetTask := project.Tasks[0]
	deps.Selector.SelectedTask = targetTask

	// Verify mock returns the configured task
	selected, err := deps.Selector.SelectTask(project.Tasks, "Select task", "")
	testutil.AssertNoError(t, err)
	testutil.AssertEqual(t, selected.ID, targetTask.ID)
}

func TestIntegration_MockRepositorySaveAndLoad(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Repository Test")
	defer deps.Cleanup()

	// Add a new task
	newTask, err := models.NewTask("repo-test-1", project.GetNextShortID(), models.TaskTypeTask, "Repository Test Task")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, newTask)

	// Save the project
	err = deps.Repository.Save(project)
	testutil.AssertNoError(t, err)

	// Create a new project reference and load
	loadedProject := &models.Project{ID: project.ID}
	err = deps.Repository.Load(loadedProject)
	testutil.AssertNoError(t, err)

	// Verify loaded project has the new task
	testutil.AssertEqual(t, len(loadedProject.Tasks), len(project.Tasks))
}

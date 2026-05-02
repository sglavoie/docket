package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectDeleteCleansCrossProjectRefs(t *testing.T) {
	// Set up mock repository so LoadProject/SaveProject work without filesystem
	mockRepo := testutil.NewMockRepository()
	origRepo := storage.DefaultRepository
	storage.DefaultRepository = mockRepo
	defer func() { storage.DefaultRepository = origRepo }()

	// Create two projects: A (to be deleted) and B (has refs to A)
	projectA := testutil.NewTestProject("Project A")
	projectB := testutil.NewTestProject("Project B")

	// Create tasks in project B with cross-project refs to project A
	// Cross-project refs use the slug (not UUID) as prefix: "slug:taskID"
	task1, err := models.NewTask("task-b1", projectB.GetNextShortID(), models.TaskTypeTask, "Task B1")
	if err != nil {
		t.Fatal(err)
	}
	task1.DependsOn = []string{
		projectA.Slug + ":task-a1", // cross-project ref to A (should be removed)
		"same-project-task-id",     // same-project ref (should be preserved)
		"other-project:task-x1",    // cross-project ref to different project (should be preserved)
	}
	task1.RelatedTo = []string{
		projectA.Slug + ":task-a2", // cross-project ref to A (should be removed)
		"local-task-id",            // same-project ref (should be preserved)
	}

	task2, err := models.NewTask("task-b2", projectB.GetNextShortID(), models.TaskTypeTask, "Task B2")
	if err != nil {
		t.Fatal(err)
	}
	task2.DependsOn = []string{
		projectA.Slug + ":task-a3", // cross-project ref to A (should be removed)
	}

	projectB.Tasks = []*models.Task{task1, task2}

	// Register a separate project object in mock to avoid aliasing
	// (mock Load overwrites project.Tasks which would clobber stored.Tasks
	// if they point to the same object)
	mockProjectB := testutil.NewTestProject("Project B")
	mockProjectB.ID = projectB.ID
	mockProjectB.Slug = projectB.Slug
	mockProjectB.Tasks = []*models.Task{task1, task2}
	mockRepo.Projects[projectB.ID] = mockProjectB

	// Run cleanup: only project B remains (A is being deleted)
	// Pass slug (not UUID) since cross-project refs use the slug format
	remainingProjects := []*models.Project{projectB}
	totalCleaned, err := cleanCrossProjectRefs(remainingProjects, projectA.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have cleaned 3 refs total (2 from task1 + 1 from task2)
	testutil.AssertEqual(t, totalCleaned, 3)

	// Verify task1 DependsOn: only same-project and other-project refs remain
	savedB := mockRepo.Projects[projectB.ID]
	var savedTask1, savedTask2 *models.Task
	for _, task := range savedB.Tasks {
		switch task.ID {
		case "task-b1":
			savedTask1 = task
		case "task-b2":
			savedTask2 = task
		}
	}

	if savedTask1 == nil {
		t.Fatal("task-b1 not found in saved project")
	}
	testutil.AssertEqual(t, len(savedTask1.DependsOn), 2)
	testutil.AssertEqual(t, savedTask1.DependsOn[0], "same-project-task-id")
	testutil.AssertEqual(t, savedTask1.DependsOn[1], "other-project:task-x1")

	// Verify task1 RelatedTo: only local ref remains
	testutil.AssertEqual(t, len(savedTask1.RelatedTo), 1)
	testutil.AssertEqual(t, savedTask1.RelatedTo[0], "local-task-id")

	// Verify task2 DependsOn: empty after removal
	if savedTask2 == nil {
		t.Fatal("task-b2 not found in saved project")
	}
	testutil.AssertEqual(t, len(savedTask2.DependsOn), 0)
}

func TestProjectDeleteCleansCrossProjectRefsNoRefsUnchanged(t *testing.T) {
	// Set up mock repository
	mockRepo := testutil.NewMockRepository()
	origRepo := storage.DefaultRepository
	storage.DefaultRepository = mockRepo
	defer func() { storage.DefaultRepository = origRepo }()

	projectA := testutil.NewTestProject("Project A")
	projectB := testutil.NewTestProject("Project B")

	// Task with no cross-project refs to project A
	task1, err := models.NewTask("task-b1", projectB.GetNextShortID(), models.TaskTypeTask, "Task B1")
	if err != nil {
		t.Fatal(err)
	}
	task1.DependsOn = []string{"local-dep"}
	task1.RelatedTo = []string{"local-rel"}

	projectB.Tasks = []*models.Task{task1}

	// Store a separate copy to avoid aliasing in mock Load
	mockProjectB := testutil.NewTestProject("Project B")
	mockProjectB.ID = projectB.ID
	mockProjectB.Slug = projectB.Slug
	mockProjectB.Tasks = []*models.Task{task1}
	mockRepo.Projects[projectB.ID] = mockProjectB

	totalCleaned, err := cleanCrossProjectRefs([]*models.Project{projectB}, projectA.Slug)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No refs to clean
	testutil.AssertEqual(t, totalCleaned, 0)
}

func TestFilterRefs(t *testing.T) {
	deletedSlug := "project-a"

	tests := []struct {
		name     string
		refs     []string
		expected []string
	}{
		{
			name:     "nil refs unchanged",
			refs:     nil,
			expected: nil,
		},
		{
			name:     "empty refs unchanged",
			refs:     []string{},
			expected: []string{},
		},
		{
			name:     "removes cross-project ref to deleted project",
			refs:     []string{deletedSlug + ":task-1", "local-task"},
			expected: []string{"local-task"},
		},
		{
			name:     "preserves cross-project ref to other project",
			refs:     []string{"other-project:task-1", deletedSlug + ":task-2"},
			expected: []string{"other-project:task-1"},
		},
		{
			name:     "preserves all when no match",
			refs:     []string{"local-1", "other:task-1"},
			expected: []string{"local-1", "other:task-1"},
		},
		{
			name:     "removes all when all match",
			refs:     []string{deletedSlug + ":t1", deletedSlug + ":t2"},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterRefs(tc.refs, deletedSlug)
			if tc.refs == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			testutil.AssertEqual(t, len(result), len(tc.expected))
			for i, ref := range result {
				testutil.AssertEqual(t, ref, tc.expected[i])
			}
		})
	}
}

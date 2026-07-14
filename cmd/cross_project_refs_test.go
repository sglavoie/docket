package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestProjectDeleteCleansCrossProjectRefsOnDisk exercises the real
// FileSystemRepository path. The rewrite loader retains both persisted qualifier
// formats, and the returned artifact write carries the identity-cleaned tasks.md
// content that the atomic delete transaction commits.
func TestProjectDeleteCleansCrossProjectRefsOnDisk(t *testing.T) {
	origRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	defer func() { storage.DefaultRepository = origRepo }()

	tempDir := t.TempDir()
	projectsDir := filepath.Join(tempDir, "projects")
	if err := os.MkdirAll(projectsDir, 0o755); err != nil {
		t.Fatalf("mkdir projects: %v", err)
	}

	// Both projects exist on disk in the pre-trash state used by the planner.
	projectA := testutil.NewTestProject("Project A")
	projectA.Path = filepath.Join(projectsDir, projectA.Slug)
	projectB := testutil.NewTestProject("Project B")
	projectB.Path = filepath.Join(projectsDir, projectB.Slug)

	task1, err := models.NewTask("task-b1", projectB.GetNextShortID(), models.TaskTypeTask, "Task B1")
	if err != nil {
		t.Fatal(err)
	}
	task1.DependsOn = []string{
		projectA.Slug + ":task-a1",
		projectA.ID + ":task-a2",
	}
	task1.RelatedTo = []string{
		projectA.ID + ":task-a3",
		projectA.Slug + ":task-a4",
	}
	projectB.Tasks = []*models.Task{task1}

	if createErr := storage.CreateProjectDir(projectA); createErr != nil {
		t.Fatalf("create project A: %v", createErr)
	}
	if createErr := storage.CreateProjectDir(projectB); createErr != nil {
		t.Fatalf("create project B: %v", createErr)
	}
	base, readErr := os.ReadFile(storage.TasksFilePath(projectB.Path))
	if readErr != nil {
		t.Fatalf("read project B base: %v", readErr)
	}
	refs, projectsWithRefs := countCrossProjectRefs([]*models.Project{projectB}, projectA.Slug, projectA.ID)
	testutil.AssertEqual(t, refs, 4)
	testutil.AssertEqual(t, projectsWithRefs, 1)

	writes, cleaned := planCrossProjectRefCleanup([]*models.Project{projectB}, projectA.Slug, projectA.ID)
	testutil.AssertEqual(t, cleaned, 4)
	if len(writes) != 1 {
		t.Fatalf("expected 1 tasks.md rewrite, got %d", len(writes))
	}

	content := string(writes[0].Data)
	if strings.Contains(content, projectA.Slug+":task-a1") {
		t.Errorf("rewritten tasks.md still contains stale DependsOn ref to deleted project A:\n%s", content)
	}
	for _, qualifier := range []string{projectA.Slug, projectA.ID} {
		if strings.Contains(content, qualifier+":") {
			t.Errorf("rewritten tasks.md still contains stale ref to deleted project A via %q:\n%s", qualifier, content)
		}
	}
	testutil.AssertEqual(t, writes[0].Path, storage.TasksFilePath(projectB.Path))
	if writes[0].ExpectedRevision == nil {
		t.Fatal("cleanup rewrite is missing its tasks.md revision precondition")
	}
	testutil.AssertEqual(t, *writes[0].ExpectedRevision, storage.RawArtifactRevision(base))
}

func TestProjectRenameCrossProjectRewriteCarriesRevision(t *testing.T) {
	origRepo := storage.DefaultRepository
	storage.DefaultRepository = storage.NewFileSystemRepository()
	defer func() { storage.DefaultRepository = origRepo }()

	dataDir := t.TempDir()
	projectsDir := filepath.Join(dataDir, "projects")
	projectA := testutil.NewTestProject("Project A")
	projectA.Path = filepath.Join(projectsDir, projectA.Slug)
	projectB := testutil.NewTestProject("Project B")
	projectB.Path = filepath.Join(projectsDir, projectB.Slug)
	task, err := models.NewTask("task-b", projectB.GetNextShortID(), models.TaskTypeTask, "Task B")
	if err != nil {
		t.Fatal(err)
	}
	task.DependsOn = []string{projectA.Slug + ":task-a"}
	projectB.Tasks = []*models.Task{task}
	if err := storage.CreateProjectDir(projectA); err != nil {
		t.Fatalf("create project A: %v", err)
	}
	if err := storage.CreateProjectDir(projectB); err != nil {
		t.Fatalf("create project B: %v", err)
	}
	base, err := os.ReadFile(storage.TasksFilePath(projectB.Path))
	if err != nil {
		t.Fatalf("read project B base: %v", err)
	}

	writes, updated := planCrossProjectRefRewrites(
		[]*models.Project{projectB}, projectA.Slug, "renamed-project-a", projectA.ID)
	if updated != 1 || len(writes) != 1 {
		t.Fatalf("rewrite result = %d refs and %d writes; want 1 and 1", updated, len(writes))
	}
	if writes[0].ExpectedRevision == nil {
		t.Fatal("rename rewrite is missing its tasks.md revision precondition")
	}
	testutil.AssertEqual(t, *writes[0].ExpectedRevision, storage.RawArtifactRevision(base))
}

func TestProjectDeleteCleansCrossProjectRefs(t *testing.T) {
	// Set up mock repository so LoadProject works without a filesystem.
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
	localDepTask, err := models.NewTask("same-project-task-id", projectB.GetNextShortID(), models.TaskTypeTask, "Local dependency")
	if err != nil {
		t.Fatal(err)
	}
	localRelatedTask, err := models.NewTask("local-task-id", projectB.GetNextShortID(), models.TaskTypeTask, "Local relation")
	if err != nil {
		t.Fatal(err)
	}
	task1.DependsOn = []string{
		projectA.Slug + ":task-a1", // cross-project ref to A (should be removed)
		projectA.ID + ":task-a4",   // stable-ID ref to A (should be removed)
		localDepTask.ID,            // same-project ref (should be preserved)
		"other-project:task-x1",    // cross-project ref to different project (should be preserved)
	}
	task1.RelatedTo = []string{
		projectA.ID + ":task-a2", // stable-ID ref to A (should be removed)
		localRelatedTask.ID,      // same-project ref (should be preserved)
	}

	task2, err := models.NewTask("task-b2", projectB.GetNextShortID(), models.TaskTypeTask, "Task B2")
	if err != nil {
		t.Fatal(err)
	}
	task2.DependsOn = []string{
		projectA.Slug + ":task-a3", // cross-project ref to A (should be removed)
	}

	projectB.Tasks = []*models.Task{task1, localDepTask, localRelatedTask, task2}

	// Register a separate project object in mock to avoid aliasing
	// (mock Load overwrites project.Tasks which would clobber stored.Tasks
	// if they point to the same object)
	mockProjectB := testutil.NewTestProject("Project B")
	mockProjectB.ID = projectB.ID
	mockProjectB.Slug = projectB.Slug
	mockProjectB.Tasks = []*models.Task{task1, localDepTask, localRelatedTask, task2}
	mockRepo.Projects[projectB.ID] = mockProjectB

	// Plan cleanup: only project B remains (A is being deleted). Pass the slug
	// (not UUID) since cross-project refs use the slug format.
	writes, totalCleaned := planCrossProjectRefCleanup([]*models.Project{projectB}, projectA.Slug, projectA.ID)

	// Should have cleaned 4 refs total (3 from task1 + 1 from task2), covering
	// both persisted qualifier representations.
	testutil.AssertEqual(t, totalCleaned, 4)
	if len(writes) != 1 {
		t.Fatalf("expected 1 tasks.md rewrite, got %d", len(writes))
	}

	// The rewritten tasks.md must drop every ref to project A but keep the
	// same-project and other-project refs.
	content := string(writes[0].Data)
	for _, qualifier := range []string{projectA.Slug, projectA.ID} {
		if strings.Contains(content, qualifier+":") {
			t.Errorf("rewritten tasks.md still references deleted project A via %q:\n%s", qualifier, content)
		}
	}
	if !strings.Contains(content, "other-project:task-x1") {
		t.Errorf("rewritten tasks.md dropped an unrelated cross-project ref:\n%s", content)
	}

	// The in-memory projectB tasks are mutated in place by the planner; verify the
	// surviving refs directly.
	var planTask1, planTask2 *models.Task
	for _, task := range projectB.Tasks {
		switch task.ID {
		case "task-b1":
			planTask1 = task
		case "task-b2":
			planTask2 = task
		}
	}
	if planTask1 == nil {
		t.Fatal("task-b1 not found")
	}
	testutil.AssertEqual(t, len(planTask1.DependsOn), 2)
	testutil.AssertEqual(t, planTask1.DependsOn[0], "same-project-task-id")
	testutil.AssertEqual(t, planTask1.DependsOn[1], "other-project:task-x1")
	testutil.AssertEqual(t, len(planTask1.RelatedTo), 1)
	testutil.AssertEqual(t, planTask1.RelatedTo[0], "local-task-id")

	if planTask2 == nil {
		t.Fatal("task-b2 not found")
	}
	testutil.AssertEqual(t, len(planTask2.DependsOn), 0)
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
	localDepTask, err := models.NewTask("local-dep", projectB.GetNextShortID(), models.TaskTypeTask, "Local dependency")
	if err != nil {
		t.Fatal(err)
	}
	localRelatedTask, err := models.NewTask("local-rel", projectB.GetNextShortID(), models.TaskTypeTask, "Local relation")
	if err != nil {
		t.Fatal(err)
	}
	task1.DependsOn = []string{localDepTask.ID}
	task1.RelatedTo = []string{localRelatedTask.ID}

	projectB.Tasks = []*models.Task{task1, localDepTask, localRelatedTask}

	// Store a separate copy to avoid aliasing in mock Load
	mockProjectB := testutil.NewTestProject("Project B")
	mockProjectB.ID = projectB.ID
	mockProjectB.Slug = projectB.Slug
	mockProjectB.Tasks = []*models.Task{task1, localDepTask, localRelatedTask}
	mockRepo.Projects[projectB.ID] = mockProjectB

	writes, totalCleaned := planCrossProjectRefCleanup([]*models.Project{projectB}, projectA.Slug, projectA.ID)

	// No refs to clean.
	testutil.AssertEqual(t, totalCleaned, 0)
	if len(writes) != 0 {
		t.Fatalf("expected no rewrites when nothing changed, got %d", len(writes))
	}
}

func TestFilterRefs(t *testing.T) {
	deletedSlug := "project-a"
	deletedID := "project-id-a"

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
			name:     "removes stable ID ref to deleted project",
			refs:     []string{deletedID + ":task-1", "local-task"},
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
			refs:     []string{deletedSlug + ":t1", deletedID + ":t2"},
			expected: []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := filterRefs(tc.refs, deletedSlug, deletedID)
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

func TestCountCrossProjectRefsMatchesIDAndSlugQualifiers(t *testing.T) {
	mockRepo := testutil.NewMockRepository()
	origRepo := storage.DefaultRepository
	storage.DefaultRepository = mockRepo
	defer func() { storage.DefaultRepository = origRepo }()

	deleted := testutil.NewTestProject("Deleted")
	carrier := testutil.NewTestProject("Carrier")
	task, err := models.NewTask("carrier-task", carrier.GetNextShortID(), models.TaskTypeTask, "Carrier task")
	if err != nil {
		t.Fatal(err)
	}
	task.DependsOn = []string{deleted.ID + ":dep", "other:dep"}
	task.RelatedTo = []string{deleted.Slug + ":rel"}
	carrier.Tasks = []*models.Task{task}

	stored := testutil.NewTestProject("Carrier")
	stored.ID = carrier.ID
	stored.Slug = carrier.Slug
	stored.Tasks = []*models.Task{task}
	mockRepo.Projects[carrier.ID] = stored

	refs, projects := countCrossProjectRefs([]*models.Project{carrier}, deleted.Slug, deleted.ID)
	testutil.AssertEqual(t, refs, 2)
	testutil.AssertEqual(t, projects, 1)
}

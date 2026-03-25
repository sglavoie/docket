package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestConfigRemoveProject(t *testing.T) {
	cfg, project := testutil.NewTestConfigWithProject("Test Project")

	// Verify project exists
	testutil.AssertEqual(t, len(cfg.Projects), 1)
	testutil.AssertEqual(t, cfg.CurrentProject, project.ID)

	// Remove the project
	removed := cfg.RemoveProject(project.ID)
	if !removed {
		t.Error("project should have been removed")
	}

	testutil.AssertEqual(t, len(cfg.Projects), 0)
	testutil.AssertEqual(t, cfg.CurrentProject, "") // Should be cleared
}

func TestConfigRemoveProjectCleansUpReferences(t *testing.T) {
	cfg, project := testutil.NewTestConfigWithProject("Test Project")

	// Set up various references to the project
	cfg.ProjectScoringProfile = map[string]string{project.ID: "profile-1", "other-id": "profile-2"}
	cfg.ProjectViewStates = map[string]config.ProjectViewState{
		project.ID: {ShowCompleted: true},
		"other-id": {ShowCompleted: false},
	}
	cfg.WorkflowEvalStates = map[string]config.WorkflowEvalState{
		project.ID: {},
		"other-id": {},
	}
	cfg.FocusQueue = []config.FocusQueueItem{
		{TaskID: "t1", ProjectID: project.ID},
		{TaskID: "t2", ProjectID: "other-id"},
	}
	cfg.RecentTaskVisits = []config.RecentTaskVisit{
		{TaskID: "t1", ProjectID: project.ID},
		{TaskID: "t2", ProjectID: "other-id"},
	}
	cfg.CycleProjects = []string{project.ID, "other-id"}
	cfg.ContextCycleProjects = map[string][]string{
		"work": {project.ID, "other-id"},
	}
	cfg.CustomContexts = map[string][]string{
		"custom": {project.ID, "other-id"},
	}
	cfg.ActionInboxProjectID = project.ID

	// Remove the project
	removed := cfg.RemoveProject(project.ID)
	if !removed {
		t.Fatal("project should have been removed")
	}

	// Verify all references are cleaned up
	if _, ok := cfg.ProjectScoringProfile[project.ID]; ok {
		t.Error("scoring profile reference should be removed")
	}
	if len(cfg.ProjectScoringProfile) != 1 {
		t.Error("other scoring profile should be preserved")
	}

	if _, ok := cfg.ProjectViewStates[project.ID]; ok {
		t.Error("view state should be removed")
	}
	if len(cfg.ProjectViewStates) != 1 {
		t.Error("other view state should be preserved")
	}

	if _, ok := cfg.WorkflowEvalStates[project.ID]; ok {
		t.Error("workflow eval state should be removed")
	}

	if len(cfg.FocusQueue) != 1 || cfg.FocusQueue[0].ProjectID != "other-id" {
		t.Error("focus queue should only contain other project's entry")
	}

	if len(cfg.RecentTaskVisits) != 1 || cfg.RecentTaskVisits[0].ProjectID != "other-id" {
		t.Error("recent visits should only contain other project's entry")
	}

	if len(cfg.CycleProjects) != 1 || cfg.CycleProjects[0] != "other-id" {
		t.Error("cycle projects should only contain other project")
	}

	if len(cfg.ContextCycleProjects["work"]) != 1 || cfg.ContextCycleProjects["work"][0] != "other-id" {
		t.Error("context cycle projects should only contain other project")
	}

	if len(cfg.CustomContexts["custom"]) != 1 || cfg.CustomContexts["custom"][0] != "other-id" {
		t.Error("custom contexts should only contain other project")
	}

	if cfg.ActionInboxProjectID != "" {
		t.Error("action inbox project ID should be cleared")
	}
}

func TestConfigRemoveNonexistentProject(t *testing.T) {
	cfg := testutil.NewTestConfig()

	removed := cfg.RemoveProject("nonexistent-id")
	if removed {
		t.Error("removing nonexistent project should return false")
	}
}

func TestConfigRemoveProjectPreservesOthers(t *testing.T) {
	cfg := testutil.NewTestConfig()

	// Add multiple projects
	project1 := testutil.NewTestProject("Project 1")
	project2 := testutil.NewTestProject("Project 2")
	project3 := testutil.NewTestProject("Project 3")

	cfg.Projects = []*models.Project{project1, project2, project3}
	cfg.CurrentProject = project2.ID

	// Remove project2
	removed := cfg.RemoveProject(project2.ID)
	if !removed {
		t.Error("project should have been removed")
	}

	testutil.AssertEqual(t, len(cfg.Projects), 2)
	testutil.AssertEqual(t, cfg.CurrentProject, "") // Was the current project

	// Verify remaining projects are correct
	for _, p := range cfg.Projects {
		if p.ID == project2.ID {
			t.Error("removed project should not be in list")
		}
	}
}

func TestConfigRemoveProjectNotCurrent(t *testing.T) {
	cfg := testutil.NewTestConfig()

	project1 := testutil.NewTestProject("Project 1")
	project2 := testutil.NewTestProject("Project 2")

	cfg.Projects = []*models.Project{project1, project2}
	cfg.CurrentProject = project1.ID

	// Remove project2 (not the current one)
	removed := cfg.RemoveProject(project2.ID)
	if !removed {
		t.Error("project should have been removed")
	}

	testutil.AssertEqual(t, len(cfg.Projects), 1)
	testutil.AssertEqual(t, cfg.CurrentProject, project1.ID) // Should be preserved
}

func TestConfigGetProject(t *testing.T) {
	cfg := testutil.NewTestConfig()

	project := testutil.NewTestProject("My Project")
	project.Slug = models.Slugify(project.Name)
	cfg.Projects = []*models.Project{project}

	// Find by ID
	found := cfg.GetProject(project.ID)
	if found == nil {
		t.Fatal("should find project by ID")
	}
	testutil.AssertEqual(t, found.Name, "My Project")

	// Find by slug
	found = cfg.GetProject(project.Slug)
	if found == nil {
		t.Fatal("should find project by slug")
	}
	testutil.AssertEqual(t, found.Name, "My Project")

	// Name lookup is not supported — only ID and slug
	notFoundByName := cfg.GetProject(project.Name)
	if notFoundByName != nil {
		t.Error("should not find project by name (only ID and slug are supported)")
	}

	// Not found
	notFound := cfg.GetProject("nonexistent")
	if notFound != nil {
		t.Error("should not find nonexistent project")
	}
}

func TestConfigGetCurrentProject(t *testing.T) {
	cfg := testutil.NewTestConfig()

	// No current project
	current := cfg.GetCurrentProject()
	if current != nil {
		t.Error("should return nil when no current project")
	}

	// Set current project
	project := testutil.NewTestProject("Test")
	cfg.Projects = []*models.Project{project}
	cfg.CurrentProject = project.ID

	current = cfg.GetCurrentProject()
	if current == nil {
		t.Fatal("should return current project")
	}
	testutil.AssertEqual(t, current.ID, project.ID)
}

func TestConfigGetCurrentProjectInvalid(t *testing.T) {
	cfg := testutil.NewTestConfig()

	// Set to invalid ID
	cfg.CurrentProject = "invalid-id"

	current := cfg.GetCurrentProject()
	if current != nil {
		t.Error("should return nil when current project ID is invalid")
	}
}

func TestProjectArchived(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Initially not archived
	testutil.AssertEqual(t, project.Archived, false)
	if project.ArchivedAt != nil {
		t.Error("ArchivedAt should be nil initially")
	}
}

func TestConfigAddProject(t *testing.T) {
	cfg := testutil.NewTestConfig()

	project := testutil.NewTestProject("New Project")

	cfg.AddProject(project)

	testutil.AssertEqual(t, len(cfg.Projects), 1)
	testutil.AssertEqual(t, cfg.Projects[0].Name, "New Project")
}

func TestConfigAddMultipleProjects(t *testing.T) {
	cfg := testutil.NewTestConfig()

	project1 := testutil.NewTestProject("Project 1")
	project2 := testutil.NewTestProject("Project 2")

	cfg.AddProject(project1)
	cfg.AddProject(project2)

	testutil.AssertEqual(t, len(cfg.Projects), 2)
}

func TestProjectRemoveTaskClearsReferences(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create parent and child
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, task)
	project.BuildTaskTree()

	// Verify child is linked
	if len(epic.Children) != 1 {
		t.Errorf("epic should have 1 child, got %d", len(epic.Children))
	}

	// Remove child task by filtering slice (standard pattern)
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	testutil.AssertTaskCount(t, project, 1)

	// Rebuild tree and verify epic has no children
	project.BuildTaskTree()
	if len(epic.Children) != 0 {
		t.Errorf("epic should have 0 children after removal, got %d", len(epic.Children))
	}
}

func TestProjectRemoveTaskPreservesOthers(t *testing.T) {
	project := testutil.NewTestProject("Test Project")

	// Create hierarchy: epic -> story -> task
	epic, err := models.NewTask("epic-1", project.GetNextShortID(), models.TaskTypeEpic, "Epic")
	if err != nil {
		t.Fatal(err)
	}
	project.Tasks = append(project.Tasks, epic)

	story, err := models.NewTask("story-1", project.GetNextShortID(), models.TaskTypeStory, "Story")
	if err != nil {
		t.Fatal(err)
	}
	story.ParentID = &epic.ID
	project.Tasks = append(project.Tasks, story)

	task, err := models.NewTask("task-1", project.GetNextShortID(), models.TaskTypeTask, "Task")
	if err != nil {
		t.Fatal(err)
	}
	task.ParentID = &story.ID
	project.Tasks = append(project.Tasks, task)

	project.BuildTaskTree()

	testutil.AssertTaskCount(t, project, 3)

	// Remove just the task (children removal is handled separately by TUI)
	for i, t := range project.Tasks {
		if t.ID == task.ID {
			project.Tasks = append(project.Tasks[:i], project.Tasks[i+1:]...)
			break
		}
	}

	// Epic and story should remain
	testutil.AssertTaskCount(t, project, 2)

	// Verify epic and story still exist
	if project.GetTaskByID(epic.ID) == nil {
		t.Error("epic should still exist")
	}
	if project.GetTaskByID(story.ID) == nil {
		t.Error("story should still exist")
	}
}

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

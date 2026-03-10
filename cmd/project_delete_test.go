package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
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

	// Find by name
	found = cfg.GetProject(project.Name)
	if found == nil {
		t.Fatal("should find project by name")
	}
	testutil.AssertEqual(t, found.Name, "My Project")

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

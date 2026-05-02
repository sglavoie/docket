package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
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

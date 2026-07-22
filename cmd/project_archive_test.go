package cmd

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectArchive(t *testing.T) {
	t.Run("archives project successfully", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		// Configure viper to use test config path
		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		// Create and add project to config
		project := testutil.NewTestProject("Test Project")
		setTestProjects(deps.Config, []*models.Project{project})
		deps.Config.Machine().CurrentProject = project.ID

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		// Archive the project
		err = runProjectArchive(nil, []string{project.ID})
		testutil.AssertNoError(t, err)

		// Reload config to verify
		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)

		reloadedProject := reloaded.GetProject(project.ID)
		if reloadedProject == nil {
			t.Fatal("project should still exist after archiving")
		}
		if !reloadedProject.Archived {
			t.Error("project should be archived")
		}
		if reloaded.Machine().CurrentProject == project.ID {
			t.Error("current project should be cleared when archiving")
		}
	})

	t.Run("clears current project stored as slug (legacy config)", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		project := testutil.NewTestProject("Legacy Project")
		setTestProjects(deps.Config, []*models.Project{project})
		// Simulate legacy config where CurrentProject stored the slug
		// instead of the ID.
		deps.Config.Machine().CurrentProject = project.Slug

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectArchive(nil, []string{project.ID})
		testutil.AssertNoError(t, err)

		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)

		if reloaded.Machine().CurrentProject != "" {
			t.Errorf("current project should be cleared when archiving legacy-slug-referenced project, got %q", reloaded.Machine().CurrentProject)
		}
	})

	t.Run("rejects archiving already archived project", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		project := testutil.NewTestProject("Archived Project")
		project.Archived = true
		setTestProjects(deps.Config, []*models.Project{project})

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectArchive(nil, []string{project.ID})
		if err == nil {
			t.Error("runProjectArchive() should error for already archived project")
		}
	})

	t.Run("rejects nonexistent project", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectArchive(nil, []string{"nonexistent"})
		if err == nil {
			t.Error("runProjectArchive() should error for nonexistent project")
		}
	})
}

func TestProjectArchivePreservesDuplicateIDReferencesInOtherProject(t *testing.T) {
	deps := setupTestDeps(t)
	defer deps.Cleanup()
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	project := func(id string) *models.Project {
		p, err := models.NewProject(id, id, "")
		if err != nil {
			t.Fatalf("NewProject(%s): %v", id, err)
		}
		p.Path = filepath.Join(deps.DataDir, "projects", p.Slug)
		task, err := models.NewTask("shared-id", 1, models.TaskTypeTask, id+" shared")
		if err != nil {
			t.Fatalf("NewTask(%s): %v", id, err)
		}
		p.Tasks = []*models.Task{task}
		p.NextShortID = 2
		if err := storage.SaveProject(p); err != nil {
			t.Fatalf("SaveProject(%s): %v", id, err)
		}
		return p
	}
	alpha := project("alpha")
	beta := project("beta")
	setTestProjects(deps.Config, []*models.Project{alpha, beta})
	deps.Config.Machine().CurrentProject = alpha.ID
	deps.Config.Planning().FocusQueue = []config.FocusQueueItem{
		{ProjectID: alpha.ID, TaskID: "shared-id"},
		{ProjectID: beta.ID, TaskID: "shared-id"},
	}
	deps.Config.Planning().DailyPlan = []config.DailyPlanItem{
		{ProjectID: alpha.ID, TaskID: "shared-id"},
		{ProjectID: beta.ID, TaskID: "shared-id"},
	}
	deps.Config.Planning().FocusDismissedIDs = []string{"shared-id"}
	deps.Config.Machine().RecentTaskVisits = []config.RecentTaskVisit{
		{ProjectID: alpha.ID, TaskID: "shared-id"},
		{ProjectID: beta.ID, TaskID: "shared-id"},
	}
	manager := config.NewManager(deps.ConfigPath)
	if err := saveTestRuntime(manager, deps.Config); err != nil {
		t.Fatalf("saveTestRuntime: %v", err)
	}

	if err := runProjectArchive(nil, []string{beta.ID}); err != nil {
		t.Fatalf("runProjectArchive: %v", err)
	}
	reloaded, err := manager.LoadRuntimeState()
	if err != nil {
		t.Fatalf("LoadRuntimeState: %v", err)
	}
	if got := reloaded.Planning().FocusQueue; len(got) != 1 || got[0].ProjectID != alpha.ID {
		t.Fatalf("project archive cleaned wrong focus refs: %+v", got)
	}
	if got := reloaded.Planning().DailyPlan; len(got) != 1 || got[0].ProjectID != alpha.ID {
		t.Fatalf("project archive cleaned wrong daily refs: %+v", got)
	}
	if got := reloaded.Planning().FocusDismissedIDs; len(got) != 1 || got[0] != "shared-id" {
		t.Fatalf("project archive changed legacy dismissed IDs: %v", got)
	}
	if got := reloaded.Machine().RecentTaskVisits; len(got) != 1 || got[0].ProjectID != alpha.ID {
		t.Fatalf("project archive cleaned wrong recent visits: %+v", got)
	}
}

func TestProjectUnarchive(t *testing.T) {
	t.Run("unarchives project successfully", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		// Create archived project
		project := testutil.NewTestProject("Archived Project")
		project.Archived = true
		setTestProjects(deps.Config, []*models.Project{project})

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectUnarchive(nil, []string{project.ID})
		testutil.AssertNoError(t, err)

		// Reload config to verify
		reloaded, err := manager.LoadRuntimeState()
		testutil.AssertNoError(t, err)

		reloadedProject := reloaded.GetProject(project.ID)
		if reloadedProject == nil {
			t.Fatal("project should exist after unarchiving")
		}
		if reloadedProject.Archived {
			t.Error("project should not be archived after unarchive")
		}
	})

	t.Run("rejects unarchiving non-archived project", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()

		project := testutil.NewTestProject("Active Project")
		project.Archived = false
		setTestProjects(deps.Config, []*models.Project{project})

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectUnarchive(nil, []string{project.ID})
		if err == nil {
			t.Error("runProjectUnarchive() should error for non-archived project")
		}
	})
}

// Note: Testing project purge is complex as it involves trash directory
// operations and user input confirmation. The command uses storage.PurgeFromTrash
// which would require filesystem mocking to test properly.

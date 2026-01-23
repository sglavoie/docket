package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectArchive(t *testing.T) {
	t.Run("archives project successfully", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		// Create and add project to config
		project := testutil.NewTestProject("Test Project")
		deps.Config.Projects = []*models.Project{project}
		deps.Config.CurrentProject = project.ID

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		// Archive the project
		err = runProjectArchive(nil, []string{project.ID})
		testutil.AssertNoError(t, err)

		// Reload config to verify
		reloaded, err := manager.Load()
		testutil.AssertNoError(t, err)

		reloadedProject := reloaded.GetProject(project.ID)
		if reloadedProject == nil {
			t.Fatal("project should still exist after archiving")
		}
		if !reloadedProject.Archived {
			t.Error("project should be archived")
		}
		if reloaded.CurrentProject == project.ID {
			t.Error("current project should be cleared when archiving")
		}
	})

	t.Run("rejects archiving already archived project", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		project := testutil.NewTestProject("Archived Project")
		project.Archived = true
		deps.Config.Projects = []*models.Project{project}

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectArchive(nil, []string{project.ID})
		if err == nil {
			t.Error("runProjectArchive() should error for already archived project")
		}
	})

	t.Run("rejects nonexistent project", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectArchive(nil, []string{"nonexistent"})
		if err == nil {
			t.Error("runProjectArchive() should error for nonexistent project")
		}
	})
}

func TestProjectUnarchive(t *testing.T) {
	t.Run("unarchives project successfully", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		// Create archived project
		project := testutil.NewTestProject("Archived Project")
		project.Archived = true
		deps.Config.Projects = []*models.Project{project}

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		err = runProjectUnarchive(nil, []string{project.ID})
		testutil.AssertNoError(t, err)

		// Reload config to verify
		reloaded, err := manager.Load()
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
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()

		project := testutil.NewTestProject("Active Project")
		project.Archived = false
		deps.Config.Projects = []*models.Project{project}

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
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

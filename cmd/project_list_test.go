package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestProjectList(t *testing.T) {
	t.Run("runs without error with no projects", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("runs without error with projects", func(t *testing.T) {
		setupProjectTest(t)
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		err := runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("archived flag does not mutate config project order", func(t *testing.T) {
		deps := testutil.SetupTestDeps(t)
		defer deps.Cleanup()
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add projects in a specific order: Z before A
		projectZ, _ := models.NewProject("id-z", "Zebra Project", deps.Config.GetProjectPath("zebra-project"))
		projectA, _ := models.NewProject("id-a", "Aardvark Project", deps.Config.GetProjectPath("aardvark-project"))
		deps.Config.AddProject(projectZ)
		deps.Config.AddProject(projectA)
		deps.Config.CurrentProject = projectZ.ID

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		// Capture original order
		originalFirst := deps.Config.Projects[0].ID
		originalSecond := deps.Config.Projects[1].ID

		// Run with --archived (triggers sort)
		showArchived = true
		err = runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify config order is unchanged
		if deps.Config.Projects[0].ID != originalFirst {
			t.Errorf("config.Projects[0] was mutated: got %s, want %s", deps.Config.Projects[0].ID, originalFirst)
		}
		if deps.Config.Projects[1].ID != originalSecond {
			t.Errorf("config.Projects[1] was mutated: got %s, want %s", deps.Config.Projects[1].ID, originalSecond)
		}
	})
}

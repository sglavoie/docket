package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
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
}

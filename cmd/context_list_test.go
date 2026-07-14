package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextList(t *testing.T) {
	setup := func(t *testing.T) (*testDeps, func()) {
		t.Helper()
		deps, _ := setupTestDepsWithProject(t, "Test Project")

		viper.Set("config", deps.ConfigPath)

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository

		cleanup := func() {
			viper.Set("config", "")
			storage.DefaultRepository = oldStorageRepo
			_ = contextListCmd.Flags().Set("json", "false")
			deps.Cleanup()
		}

		return deps, cleanup
	}

	t.Run("runs without error with no custom contexts", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextList(contextListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs without error with custom contexts", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.Machine().SetCustomContext("Work", []string{"id1", "id2"})
		deps.Config.Machine().SetCustomContext("Personal", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		err := runContextList(contextListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs without error with JSON output", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.Machine().SetCustomContext("Work", []string{"id1"})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		_ = contextListCmd.Flags().Set("json", "true")
		err := runContextList(contextListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

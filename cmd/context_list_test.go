package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextList(t *testing.T) {
	setup := func(t *testing.T) (*testutil.TestDeps, func()) {
		t.Helper()
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")

		viper.Set("config", deps.ConfigPath)

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository

		cleanup := func() {
			viper.Set("config", "")
			storage.DefaultRepository = oldStorageRepo
			contextListJSON = false
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

		deps.Config.SetCustomContext("Work", []string{"id1", "id2"})
		deps.Config.SetCustomContext("Personal", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextList(contextListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs without error with JSON output", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.SetCustomContext("Work", []string{"id1"})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		contextListJSON = true
		err := runContextList(contextListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

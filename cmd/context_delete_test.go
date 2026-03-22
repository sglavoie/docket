package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextDelete(t *testing.T) {
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
			contextDeleteForce = false
			deps.Cleanup()
		}

		return deps, cleanup
	}

	t.Run("rejects missing args", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := contextDeleteCmd.Args(contextDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("deletes existing context with force flag", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.SetCustomContext("Work", []string{"id1"})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		contextDeleteForce = true
		err := runContextDelete(contextDeleteCmd, []string{"Work"})
		testutil.AssertNoError(t, err)

		// Reload config and verify deletion
		reloaded, loadErr := config.NewManager(deps.ConfigPath).Load()
		testutil.AssertNoError(t, loadErr)
		if reloaded.GetCustomContext("Work") != nil {
			t.Error("expected context 'Work' to be deleted")
		}
	})

	t.Run("rejects nonexistent context", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		contextDeleteForce = true
		err := runContextDelete(contextDeleteCmd, []string{"Nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent context")
		}
	})
}

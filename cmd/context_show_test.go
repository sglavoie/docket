package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextShow(t *testing.T) {
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
			deps.Cleanup()
		}

		return deps, cleanup
	}

	t.Run("rejects missing args", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextShow(contextShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects extra args", func(t *testing.T) {
		err := contextShowCmd.Args(contextShowCmd, []string{"Work", "ignored"})
		if err == nil {
			t.Error("expected error for extra args")
		}
	})

	t.Run("rejects nonexistent context", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextShow(contextShowCmd, []string{"Nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent context")
		}
	})

	t.Run("shows context successfully", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		project := deps.Config.Projects[0]
		deps.Config.SetCustomContext("Work", []string{project.ID})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextShow(contextShowCmd, []string{"Work"})
		testutil.AssertNoError(t, err)
	})

	t.Run("shows empty context successfully", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.SetCustomContext("Empty", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextShow(contextShowCmd, []string{"Empty"})
		testutil.AssertNoError(t, err)
	})
}

package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextEdit(t *testing.T) {
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

		err := runContextEdit(contextEditCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects extra args", func(t *testing.T) {
		if err := contextEditCmd.ValidateArgs([]string{"Work", "extra"}); err == nil {
			t.Error("expected error for extra args")
		}
	})

	t.Run("rejects nonexistent context", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextEdit(contextEditCmd, []string{"Nonexistent"})
		if err == nil {
			t.Error("expected error for nonexistent context")
		}
	})
}

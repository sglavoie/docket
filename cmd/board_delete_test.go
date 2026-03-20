package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunBoardDelete(t *testing.T) {
	setup := func(t *testing.T) *testutil.TestDeps {
		t.Helper()
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")

		viper.Set("config", deps.ConfigPath)
		t.Cleanup(func() { viper.Set("config", "") })

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

		return deps
	}

	t.Run("rejects missing args", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardDelete(boardDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardDelete(boardDeleteCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})
}

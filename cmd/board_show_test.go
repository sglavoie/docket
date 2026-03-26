package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunBoardShow(t *testing.T) {
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

	t.Run("rejects missing args with correct error message", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardShow(boardShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
		// Should mention "action ID", not "board column"
		if err != nil && strings.Contains(err.Error(), "board column") {
			t.Errorf("error message should not mention 'board column': %v", err)
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardShow(boardShowCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})
}

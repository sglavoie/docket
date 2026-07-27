package cmd

import (
	"errors"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tterrors"
)

// TestRunTaskList_RejectsInvalidFilterValues verifies that typo'd filter values
// surface as errors instead of silently returning an empty list. Without these
// checks `tt task list --type fix` matched no tasks (none have type "fix") and
// printed "no tasks match", masking the typo.
func TestRunTaskList_RejectsInvalidFilterValues(t *testing.T) {
	setup := func(t *testing.T) func() {
		t.Helper()
		deps, _ := setupTestDepsWithProject(t, "Test Project")

		viper.Set("config", deps.ConfigPath)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository

		return func() {
			viper.Set("config", "")
			storage.DefaultRepository = oldStorageRepo
			deps.Cleanup()
		}
	}

	t.Run("rejects invalid --type", func(t *testing.T) {
		defer resetFilterVars()
		cleanup := setup(t)
		defer cleanup()

		listTaskType = "fix"
		err := runTaskList(taskListCmd, nil)
		if err == nil {
			t.Fatal("expected error for invalid task type, got nil")
		}
		if !errors.Is(err, tterrors.ErrInvalidTaskType) {
			t.Errorf("expected ErrInvalidTaskType, got %v", err)
		}
	})

	t.Run("rejects out-of-range --priority", func(t *testing.T) {
		defer resetFilterVars()
		cleanup := setup(t)
		defer cleanup()

		listPriority = 9
		err := runTaskList(taskListCmd, nil)
		if err == nil {
			t.Fatal("expected error for out-of-range priority, got nil")
		}
	})
}

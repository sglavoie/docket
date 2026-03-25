package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunContextRemove(t *testing.T) {
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

		err := runContextRemove(contextRemoveCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects single arg", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextRemove(contextRemoveCmd, []string{"Work"})
		if err == nil {
			t.Error("expected error for single arg")
		}
	})

	t.Run("rejects nonexistent context", func(t *testing.T) {
		_, cleanup := setup(t)
		defer cleanup()

		err := runContextRemove(contextRemoveCmd, []string{"Nonexistent", "test-project"})
		if err == nil {
			t.Error("expected error for nonexistent context")
		}
	})

	t.Run("rejects nonexistent project slug", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		deps.Config.SetCustomContext("Work", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextRemove(contextRemoveCmd, []string{"Work", "nonexistent-slug"})
		if err == nil {
			t.Error("expected error for nonexistent project slug")
		}
	})

	t.Run("removes project from context successfully", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		project := deps.Config.Projects[0]
		deps.Config.SetCustomContext("Work", []string{project.ID})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextRemove(contextRemoveCmd, []string{"Work", project.Slug})
		testutil.AssertNoError(t, err)

		// Reload config and verify removal
		reloaded, loadErr := config.NewManager(deps.ConfigPath).Load()
		testutil.AssertNoError(t, loadErr)
		ids := reloaded.GetCustomContext("Work")
		if len(ids) != 0 {
			t.Errorf("expected 0 projects in context after removal, got %d", len(ids))
		}
	})

	t.Run("rejects removing project not in context", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		project := deps.Config.Projects[0]
		deps.Config.SetCustomContext("Work", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextRemove(contextRemoveCmd, []string{"Work", project.Slug})
		if err == nil {
			t.Error("expected error for removing project not in context")
		}
	})

	t.Run("context still exists after removing last project", func(t *testing.T) {
		deps, cleanup := setup(t)
		defer cleanup()

		project := deps.Config.Projects[0]
		deps.Config.SetCustomContext("Work", []string{project.ID})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, manager.Save(deps.Config))

		err := runContextRemove(contextRemoveCmd, []string{"Work", project.Slug})
		testutil.AssertNoError(t, err)

		// Reload config and verify the context still exists (non-nil) with 0 projects.
		// Before the fix, removing the last project set nil which made the context
		// appear "not found" to other commands (add, show, edit, remove).
		reloaded, loadErr := config.NewManager(deps.ConfigPath).Load()
		testutil.AssertNoError(t, loadErr)
		ids := reloaded.GetCustomContext("Work")
		if ids == nil {
			t.Fatal("expected context to still exist (non-nil slice), got nil")
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 projects in context, got %d", len(ids))
		}

		// Verify the context is still usable by other commands (e.g., context add)
		err = runContextAdd(contextAddCmd, []string{"Work", project.Slug})
		testutil.AssertNoError(t, err)

		reloaded2, loadErr2 := config.NewManager(deps.ConfigPath).Load()
		testutil.AssertNoError(t, loadErr2)
		ids2 := reloaded2.GetCustomContext("Work")
		if len(ids2) != 1 {
			t.Errorf("expected 1 project after re-adding, got %d", len(ids2))
		}
	})
}

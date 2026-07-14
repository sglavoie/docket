package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupContextTest(t *testing.T) (*testDeps, func()) {
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
		deps.Cleanup()
	}

	return deps, cleanup
}

func TestRunContextAdd(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()
		_ = deps

		err := runContextAdd(contextAddCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects single arg", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()
		_ = deps

		err := runContextAdd(contextAddCmd, []string{"Work"})
		if err == nil {
			t.Error("expected error for single arg")
		}
	})

	t.Run("rejects nonexistent context name", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()
		_ = deps

		err := runContextAdd(contextAddCmd, []string{"Nonexistent", "test-project"})
		if err == nil {
			t.Error("expected error for nonexistent context")
		}
	})

	t.Run("rejects nonexistent project slug", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()

		deps.Config.Machine().SetCustomContext("Work", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		err := runContextAdd(contextAddCmd, []string{"Work", "nonexistent-slug"})
		if err == nil {
			t.Error("expected error for nonexistent project slug")
		}
	})

	t.Run("adds project to existing context successfully", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()

		deps.Config.Machine().SetCustomContext("Work", []string{})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		project := deps.Config.AllProjects()[0]
		err := runContextAdd(contextAddCmd, []string{"Work", project.Slug})
		testutil.AssertNoError(t, err)

		// Reload config and verify
		reloaded, loadErr := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)
		ids := reloaded.Machine().GetCustomContext("Work")
		if len(ids) != 1 {
			t.Errorf("expected 1 project in context, got %d", len(ids))
		}
		if ids[0] != project.ID {
			t.Errorf("expected project ID %s, got %s", project.ID, ids[0])
		}
	})

	t.Run("rejects duplicate project in context", func(t *testing.T) {
		deps, cleanup := setupContextTest(t)
		defer cleanup()

		project := deps.Config.AllProjects()[0]
		deps.Config.Machine().SetCustomContext("Work", []string{project.ID})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		err := runContextAdd(contextAddCmd, []string{"Work", project.Slug})
		if err == nil {
			t.Error("expected error for duplicate project in context")
		}
	})
}

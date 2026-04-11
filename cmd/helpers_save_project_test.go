package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestSaveProject(t *testing.T) {
	t.Run("basic save without options", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err := SaveProject(project, nil)
		testutil.AssertNoError(t, err)

		if deps.Repository.Projects[project.ID] == nil {
			t.Error("SaveProject() did not save project to repository")
		}
	})

	t.Run("save with rules but no config returns error", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        nil,
		}

		err := SaveProject(project, opts)
		if err == nil {
			t.Error("SaveProject() should error when Config is nil but EvaluateRules is true")
		}
	})

	t.Run("save with rules but no config manager returns error", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        deps.Config,
			ConfigManager: nil,
		}

		err := SaveProject(project, opts)
		if err == nil {
			t.Error("SaveProject() should error when ConfigManager is nil but EvaluateRules is true")
		}
	})

	t.Run("save with rules evaluates successfully", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		manager := config.NewManager(deps.ConfigPath)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		opts := &SaveOptions{
			EvaluateRules: true,
			Config:        deps.Config,
			ConfigManager: manager,
		}

		err := SaveProject(project, opts)
		testutil.AssertNoError(t, err)

		if deps.Repository.Projects[project.ID] == nil {
			t.Error("SaveProject() did not save project to repository")
		}
	})
}

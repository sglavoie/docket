package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupWorkflowTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })
	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)
	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })
	return deps, project
}

func TestWorkflowDelete(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowDeleteForce = false }()

		err := runWorkflowDelete(workflowDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects nonexistent rule ID", func(t *testing.T) {
		setupWorkflowTest(t)
		defer func() { workflowDeleteForce = false }()

		workflowDeleteForce = true
		err := runWorkflowDelete(workflowDeleteCmd, []string{"nonexistent-rule"})
		if err == nil {
			t.Error("expected error for nonexistent rule ID")
		}
	})
}

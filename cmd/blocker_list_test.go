package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetBlockerListVars() {
	showAllBlockers = false
	showResolvedBlockers = false
	showActiveBlockers = false
}

func setupBlockerListTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetBlockerListVars()

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

func TestRunBlockerList(t *testing.T) {
	t.Run("runs without error when project has no blockers file", func(t *testing.T) {
		deps, _ := setupBlockerListTest(t)
		defer deps.Cleanup()

		err := runBlockerList(blockerListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

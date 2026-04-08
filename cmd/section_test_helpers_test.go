package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupSectionCommandTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()

	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
	t.Cleanup(deps.Cleanup)

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() {
		viper.Set("config", "")
	})

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() {
		storage.DefaultRepository = oldStorageRepo
	})

	return deps, project
}

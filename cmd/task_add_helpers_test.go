package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// resetTaskAddVars resets all package-level flag variables used by runTaskAdd.
func resetTaskAddVars() {
	taskTypeFlag = ""
	taskParentFlag = ""
	taskPriority = 3
	taskTags = nil
	taskSectionFlag = ""
	taskStageFlag = ""
	taskRecurFlag = ""
	taskTemplateFlag = ""
	isEpic = false
	isStory = false
	isBug = false
	isIdea = false
	addTypeAttrs = TypeAttrValues{}
	taskCreateSectionFlag = false
}

// setupTaskAddTest creates common test dependencies and returns them.
// Caller must defer deps.Cleanup() and restore storage/viper.
func setupTaskAddTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetTaskAddVars()

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
package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetTemplateAddVars() {
	templateAddName = ""
	templateAddType = "task"
	templateAddPriority = 0
	templateAddTags = ""
	templateAddSection = ""
	templateAddDescription = ""
	templateAddCategory = ""
	templateAddChildren = nil
	templateAddOptChildren = nil
	templateAddDisabled = false
}

func setupTemplateAddTest(t *testing.T) *testDeps {
	t.Helper()
	resetTemplateAddVars()

	deps, _ := setupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	t.Cleanup(resetTemplateAddVars)

	return deps
}

func TestTemplateAddSectionValidation(t *testing.T) {
	t.Run("rejects non-GTD section", func(t *testing.T) {
		deps := setupTemplateAddTest(t)
		defer deps.Cleanup()

		templateAddName = "Invalid Section"
		templateAddSection = "Backlog"

		err := runTemplateAdd(templateAddCmd, nil)
		if err == nil {
			t.Fatal("expected error for non-GTD template section")
		}
	})

	t.Run("accepts GTD section", func(t *testing.T) {
		deps := setupTemplateAddTest(t)
		defer deps.Cleanup()

		templateAddName = "Inbox Template"
		templateAddSection = models.SectionInbox

		err := runTemplateAdd(templateAddCmd, nil)
		testutil.AssertNoError(t, err)

		reloaded, loadErr := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)

		tmpl := reloaded.Automation().GetTaskTemplateByName("Inbox Template")
		if tmpl == nil {
			t.Fatal("template not found after add")
		}
		testutil.AssertEqual(t, tmpl.State, models.StateInbox)
	})
}

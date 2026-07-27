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
	templateAddState = ""
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

func TestTemplateAddDefaultsToInbox(t *testing.T) {
	deps := setupTemplateAddTest(t)
	defer deps.Cleanup()

	templateAddName = "Inbox Template"
	if err := runTemplateAdd(templateAddCmd, nil); err != nil {
		t.Fatalf("runTemplateAdd: %v", err)
	}

	reloaded, loadErr := config.NewManager(deps.ConfigPath).LoadRuntimeState()
	testutil.AssertNoError(t, loadErr)
	tmpl := reloaded.Automation().GetTaskTemplateByName("Inbox Template")
	if tmpl == nil || tmpl.State != models.StateInbox {
		t.Fatalf("template = %+v, want inbox state", tmpl)
	}
}

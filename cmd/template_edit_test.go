package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
)

// resetTemplateEditVars resets all package-level flag variables used by runTemplateEdit.
func resetTemplateEditVars() {
	templateEditName = ""
	templateEditType = ""
	templateEditPriority = 0
	templateEditTags = ""
	templateEditState = ""
	templateEditDescription = ""
	templateEditCategory = ""
	templateEditAddChildren = nil
	templateEditAddOptChild = nil
	templateEditRemoveChild = nil
	templateEditClearChild = false
	templateEditEnabled = false
	templateEditAddChecklist = nil
	templateEditRemoveChecklist = nil
	templateEditClearChecklist = false
}

// setupTemplateEditTest creates common test dependencies and returns them.
func setupTemplateEditTest(t *testing.T) *testDeps {
	t.Helper()
	resetTemplateEditVars()

	deps, _ := setupTestDepsWithProject(t, "Test Project")

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	t.Cleanup(resetTemplateEditVars)

	return deps
}

func TestTemplateEditRemoveItemUsesOneBased(t *testing.T) {
	t.Run("remove-item 1 removes first child", func(t *testing.T) {
		deps := setupTemplateEditTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "edit-tmpl",
			Name:    "Edit Template",
			Enabled: true,
			Type:    models.TaskTypeEpic,
			Children: []template.TemplateChild{
				{Title: "First"},
				{Title: "Second"},
				{Title: "Third"},
			},
		}
		deps.Config.Automation().TaskTemplates = append(deps.Config.Automation().TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		// Set the flag to remove item 1 (1-based = first item)
		templateEditRemoveChild = []int{1}

		cmd := templateEditCmd
		// Mark the remove-item flag as changed so it's picked up
		err := cmd.Flags().Set("remove-item", "1")
		testutil.AssertNoError(t, err)

		err = runTemplateEdit(cmd, []string{"edit-tmpl"})
		testutil.AssertNoError(t, err)

		// Reload config
		reloaded, loadErr := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)

		updated := reloaded.Automation().GetTaskTemplate("edit-tmpl")
		if updated == nil {
			t.Fatal("template not found after edit")
		}
		if len(updated.Children) != 2 {
			t.Fatalf("expected 2 children, got %d", len(updated.Children))
		}
		// First item ("First") should be removed; remaining are "Second" and "Third"
		testutil.AssertEqual(t, updated.Children[0].Title, "Second")
		testutil.AssertEqual(t, updated.Children[1].Title, "Third")
	})

	t.Run("remove-item 0 is out of range", func(t *testing.T) {
		deps := setupTemplateEditTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:      "edit-tmpl-oor",
			Name:    "OOR Template",
			Enabled: true,
			Type:    models.TaskTypeTask,
			Children: []template.TemplateChild{
				{Title: "Only"},
			},
		}
		deps.Config.Automation().TaskTemplates = append(deps.Config.Automation().TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		templateEditRemoveChild = []int{0}
		cmd := templateEditCmd
		_ = cmd.Flags().Set("remove-item", "0")

		err := runTemplateEdit(cmd, []string{"edit-tmpl-oor"})
		if err == nil {
			t.Error("expected error for out-of-range index 0")
		}
	})

	t.Run("remove-checklist 1 removes first checklist item", func(t *testing.T) {
		deps := setupTemplateEditTest(t)
		defer deps.Cleanup()

		tmpl := template.TaskTemplate{
			ID:        "edit-cl-tmpl",
			Name:      "Checklist Template",
			Enabled:   true,
			Type:      models.TaskTypeTask,
			Checklist: []string{"Step A", "Step B", "Step C"},
		}
		deps.Config.Automation().TaskTemplates = append(deps.Config.Automation().TaskTemplates, tmpl)
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		templateEditRemoveChecklist = []int{1}
		cmd := templateEditCmd
		_ = cmd.Flags().Set("remove-checklist", "1")

		err := runTemplateEdit(cmd, []string{"edit-cl-tmpl"})
		testutil.AssertNoError(t, err)

		reloaded, loadErr := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, loadErr)

		updated := reloaded.Automation().GetTaskTemplate("edit-cl-tmpl")
		if updated == nil {
			t.Fatal("template not found after edit")
		}
		if len(updated.Checklist) != 2 {
			t.Fatalf("expected 2 checklist items, got %d", len(updated.Checklist))
		}
		testutil.AssertEqual(t, updated.Checklist[0], "Step B")
		testutil.AssertEqual(t, updated.Checklist[1], "Step C")
	})
}

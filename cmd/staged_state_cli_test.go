package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/template"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestCanonicalStateFlagsAreExclusiveAcrossLifecycleCommands(t *testing.T) {
	archiveList := newTaskArchiveListCmd()
	for name, command := range map[string]*cobra.Command{
		"quick add":     addCmd,
		"task add":      taskAddCmd,
		"task edit":     taskEditCmd,
		"task list":     taskListCmd,
		"archive list":  archiveList,
		"template add":  templateAddCmd,
		"template edit": templateEditCmd,
		"template use":  templateUseCmd,
	} {
		t.Run(name, func(t *testing.T) {
			if command.Flags().Lookup("state") == nil {
				t.Fatalf("%s is missing --state", name)
			}
			completion, ok := command.GetFlagCompletionFunc("state")
			if !ok {
				t.Fatalf("%s --state is missing canonical completion", name)
			}
			values, directive := completion(command, nil, "")
			if directive != cobra.ShellCompDirectiveNoFileComp || !slices.Equal(values, stateCompletions) {
				t.Fatalf("%s --state completions = %v / %v", name, values, directive)
			}
		})
	}

	for name, command := range map[string]*cobra.Command{
		"task add":      taskAddCmd,
		"template add":  templateAddCmd,
		"template edit": templateEditCmd,
	} {
		if command.Flags().Lookup("section") != nil {
			t.Fatalf("%s still exposes retired --section", name)
		}
	}
	if taskListCmd.Flags().Lookup("status") != nil || archiveList.Flags().Lookup("status") != nil {
		t.Fatal("task listing still exposes retired --status")
	}
}

func TestTaskAddCanonicalStateOverridesDefault(t *testing.T) {
	deps, project := setupTaskAddTest(t)
	defer deps.Cleanup()

	taskStateFlag = "waiting"
	if err := taskAddCmd.Flags().Set("state", taskStateFlag); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		taskStateFlag = ""
		taskAddCmd.Flags().Lookup("state").Changed = false
	})

	if err := runTaskAdd(taskAddCmd, []string{"Canonical state task"}); err != nil {
		t.Fatalf("runTaskAdd: %v", err)
	}
	got := deps.Repository.Projects[project.ID].Tasks[len(project.Tasks)]
	if got.State() != models.StateWaiting {
		t.Fatalf("state = %q, want waiting", got.State())
	}
}

func TestQuickAddCanonicalState(t *testing.T) {
	resetQuickAddVars()
	deps, project := setupTestDepsWithProject(t, "Quick State Project")
	defer deps.Cleanup()

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })
	manager := config.NewManager(deps.ConfigPath)
	testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))
	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() {
		storage.DefaultRepository = oldStorageRepo
		resetQuickAddVars()
		addCmd.Flags().Lookup("state").Changed = false
	})

	addStateFlag = "next"
	if err := addCmd.Flags().Set("state", addStateFlag); err != nil {
		t.Fatal(err)
	}
	if err := runQuickAdd(addCmd, []string{"Quick next task"}); err != nil {
		t.Fatalf("runQuickAdd: %v", err)
	}
	got := deps.Repository.Projects[project.ID].Tasks[len(project.Tasks)]
	if got.State() != models.StateNext {
		t.Fatalf("state = %q, want next", got.State())
	}
}

func TestTemplateCanonicalStateCreationAndEditing(t *testing.T) {
	t.Run("add", func(t *testing.T) {
		deps := setupTemplateAddTest(t)
		defer deps.Cleanup()

		templateAddName = "Waiting Template"
		templateAddState = "waiting"
		if err := templateAddCmd.Flags().Set("state", templateAddState); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { templateAddCmd.Flags().Lookup("state").Changed = false })

		if err := runTemplateAdd(templateAddCmd, nil); err != nil {
			t.Fatalf("runTemplateAdd: %v", err)
		}
		reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, err)
		got := reloaded.Automation().GetTaskTemplateByName("Waiting Template")
		if got == nil || got.State != models.StateWaiting {
			t.Fatalf("template = %+v, want waiting state", got)
		}
	})

	t.Run("edit", func(t *testing.T) {
		deps := setupTemplateEditTest(t)
		defer deps.Cleanup()

		deps.Config.Automation().TaskTemplates = append(deps.Config.Automation().TaskTemplates, template.TaskTemplate{
			ID: "canonical-state-edit", Name: "Canonical Edit", Enabled: true, Type: models.TaskTypeTask, State: models.StateInbox,
		})
		manager := config.NewManager(deps.ConfigPath)
		testutil.AssertNoError(t, saveTestRuntime(manager, deps.Config))

		templateEditState = "review"
		if err := templateEditCmd.Flags().Set("state", templateEditState); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { templateEditCmd.Flags().Lookup("state").Changed = false })

		if err := runTemplateEdit(templateEditCmd, []string{"canonical-state-edit"}); err != nil {
			t.Fatalf("runTemplateEdit: %v", err)
		}
		reloaded, err := config.NewManager(deps.ConfigPath).LoadRuntimeState()
		testutil.AssertNoError(t, err)
		got := reloaded.Automation().GetTaskTemplate("canonical-state-edit")
		if got == nil || got.State != models.StateReview {
			t.Fatalf("template = %+v, want review state", got)
		}
	})
}

func TestTaskEditCanonicalStateRoutesThroughWorkflowEdit(t *testing.T) {
	root := t.TempDir()
	project := newFileBackedProject(t, root, "project-edit-state", "Edit State Project")
	task := addTestTask(t, project, "edit-state-task", "Edit state", models.StateReady)
	setupFileBackedWorkflowCLI(t, root, project, []*models.Project{project}, nil, nil)

	editor := filepath.Join(root, "keep-task.sh")
	script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '# %s' '' '<!-- Edit the title above and description below. -->' '' > \"$1\"\n", task.Title)
	if err := os.WriteFile(editor, []byte(script), 0o755); err != nil {
		t.Fatalf("write editor: %v", err)
	}
	oldEditor := editEditor
	oldState := taskEditState
	editEditor = editor
	taskEditState = "waiting"
	t.Cleanup(func() {
		editEditor = oldEditor
		taskEditState = oldState
	})

	if err := runTaskEdit(newStateTestCmd(), []string{fmt.Sprintf("%d", task.ShortID)}); err != nil {
		t.Fatalf("runTaskEdit: %v", err)
	}
	reloadWorkflowProject(t, project)
	if got := project.GetTaskByID(task.ID); got == nil || got.State() != models.StateWaiting {
		t.Fatalf("edited task = %+v, want waiting state", got)
	}
}

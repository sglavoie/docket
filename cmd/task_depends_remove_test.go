package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskDependsRemoveDependency(t *testing.T) {
	t.Run("removes dependency successfully", func(t *testing.T) {
		deps, project := setupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		task1 := project.Tasks[0]
		task2 := project.Tasks[1]

		_ = task1.AddDependency(task2.ID)
		deps.Repository.Projects[project.ID] = project

		dependsOnFlag = ""
		dependsRemoveFlag = fmt.Sprintf("%d", task2.ShortID)
		dependsClearFlag = false
		defer func() { dependsRemoveFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task1.ShortID)})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		var savedTask *models.Task
		for _, task := range savedProject.Tasks {
			if task.ID == task1.ID {
				savedTask = task
				break
			}
		}

		if savedTask == nil {
			t.Fatal("task not found in saved project")
		}
		if savedTask.HasDependency(task2.ID) {
			t.Error("dependency was not removed")
		}
	})

	t.Run("removing last dep leaves task derived-unblocked without transitioning it", func(t *testing.T) {
		// Dependency blocking is a DERIVED predicate, not a workflow state.
		// Removing the last dependency must make the derived blocked predicate
		// false (HasIncompleteDependencies == false) while PRESERVING the task's
		// workflow state — the CLI no longer force-reopens the task.
		deps, project := setupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		task1 := project.Tasks[0]
		task2 := project.Tasks[1]
		_ = task1.AddDependency(task2.ID)
		task1.WorkflowState = models.StateWaiting
		deps.Repository.Projects[project.ID] = project

		dependsOnFlag = ""
		dependsRemoveFlag = fmt.Sprintf("%d", task2.ShortID)
		dependsClearFlag = false
		defer func() { dependsRemoveFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task1.ShortID)})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		var savedTask *models.Task
		for _, task := range savedProject.Tasks {
			if task.ID == task1.ID {
				savedTask = task
				break
			}
		}
		if savedTask == nil {
			t.Fatal("task not found")
		}
		if savedTask.HasDependency(task2.ID) {
			t.Error("dependency was not removed")
		}
		if savedProject.HasIncompleteDependencies(savedTask) {
			t.Errorf("expected task to be derived-unblocked after removing last dep")
		}
		if savedTask.State() != models.StateWaiting {
			t.Errorf("expected workflow state preserved (still blocked/waiting), got %s", savedTask.State())
		}
	})

	t.Run("errors when removing non-existent dependency", func(t *testing.T) {
		deps, project := setupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		task1 := project.Tasks[0]
		task2 := project.Tasks[1]

		dependsOnFlag = ""
		dependsRemoveFlag = fmt.Sprintf("%d", task2.ShortID)
		dependsClearFlag = false
		defer func() { dependsRemoveFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task1.ShortID)})
		if err == nil {
			t.Error("expected error when removing non-existent dependency")
		}
	})
}

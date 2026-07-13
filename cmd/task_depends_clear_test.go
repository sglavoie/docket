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

func TestTaskDependsClear(t *testing.T) {
	t.Run("clears all dependencies", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		task1 := project.Tasks[0]
		task2 := project.Tasks[1]
		task3 := project.Tasks[2]

		_ = task1.AddDependency(task2.ID)
		_ = task1.AddDependency(task3.ID)
		deps.Repository.Projects[project.ID] = project

		dependsOnFlag = ""
		dependsRemoveFlag = ""
		dependsClearFlag = true
		defer func() { dependsClearFlag = false }()

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
		if len(savedTask.DependsOn) != 0 {
			t.Errorf("expected 0 dependencies after clear, got %d", len(savedTask.DependsOn))
		}
	})

	t.Run("clearing all deps leaves task derived-unblocked without transitioning it", func(t *testing.T) {
		// Dependency blocking is a DERIVED predicate, not a workflow state.
		// Clearing the last dependency must make the derived blocked predicate
		// false (HasIncompleteDependencies == false) while PRESERVING the task's
		// workflow state — the CLI no longer force-reopens the task.
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
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
		dependsRemoveFlag = ""
		dependsClearFlag = true
		defer func() { dependsClearFlag = false }()

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
		if len(savedTask.DependsOn) != 0 {
			t.Errorf("expected 0 dependencies after clear, got %d", len(savedTask.DependsOn))
		}
		if savedProject.HasIncompleteDependencies(savedTask) {
			t.Errorf("expected task to be derived-unblocked after clearing all deps")
		}
		if savedTask.State() != models.StateWaiting {
			t.Errorf("expected workflow state preserved (still blocked/waiting), got %s", savedTask.State())
		}
	})

	t.Run("handles task with no dependencies to clear", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		task := project.Tasks[0]

		dependsOnFlag = ""
		dependsRemoveFlag = ""
		dependsClearFlag = true
		defer func() { dependsClearFlag = false }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task.ShortID)})
		testutil.AssertNoError(t, err)
	})
}

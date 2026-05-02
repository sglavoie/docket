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

func TestTaskDependsAddDependency(t *testing.T) {
	t.Run("adds dependency successfully", func(t *testing.T) {
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

		dependsOnFlag = fmt.Sprintf("%d", task2.ShortID)
		dependsRemoveFlag = ""
		dependsClearFlag = false
		defer func() { dependsOnFlag = "" }()

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
		if !savedTask.HasDependency(task2.ID) {
			t.Error("dependency was not added")
		}
	})

	t.Run("rejects self-dependency", func(t *testing.T) {
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

		dependsOnFlag = fmt.Sprintf("%d", task.ShortID)
		dependsRemoveFlag = ""
		dependsClearFlag = false
		defer func() { dependsOnFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task.ShortID)})
		if err == nil {
			t.Error("expected error for self-dependency")
		}
	})

	t.Run("rejects dependency on non-existent task", func(t *testing.T) {
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

		dependsOnFlag = "999"
		dependsRemoveFlag = ""
		dependsClearFlag = false
		defer func() { dependsOnFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task.ShortID)})
		if err == nil {
			t.Error("expected error for non-existent blocker task")
		}
	})

	t.Run("rejects circular dependency (A depends B, B depends A)", func(t *testing.T) {
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

		taskA := project.Tasks[0]
		taskB := project.Tasks[1]

		_ = taskB.AddDependency(taskA.ID)
		deps.Repository.Projects[project.ID] = project

		dependsOnFlag = fmt.Sprintf("%d", taskB.ShortID)
		dependsRemoveFlag = ""
		dependsClearFlag = false
		defer func() { dependsOnFlag = "" }()

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", taskA.ShortID)})
		if err == nil {
			t.Error("expected error for circular dependency")
		}
	})
}

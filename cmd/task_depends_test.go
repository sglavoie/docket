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

		taskB.AddDependency(taskA.ID)
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

func TestTaskDependsRemoveDependency(t *testing.T) {
	t.Run("removes dependency successfully", func(t *testing.T) {
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

		task1.AddDependency(task2.ID)
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

	t.Run("errors when removing non-existent dependency", func(t *testing.T) {
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

		task1.AddDependency(task2.ID)
		task1.AddDependency(task3.ID)
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

func TestTaskDependsShow(t *testing.T) {
	t.Run("shows dependencies without flags", func(t *testing.T) {
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
		dependsClearFlag = false

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task.ShortID)})
		testutil.AssertNoError(t, err)
	})

	t.Run("errors for non-existent task", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		dependsOnFlag = ""
		dependsRemoveFlag = ""
		dependsClearFlag = false

		err = runTaskDepends(nil, []string{"999"})
		if err == nil {
			t.Error("expected error for non-existent task")
		}
	})
}

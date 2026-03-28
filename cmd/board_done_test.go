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

func TestBoardDoneGeneratesRecurringInstance(t *testing.T) {
	deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
	defer deps.Cleanup()

	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })

	manager := config.NewManager(deps.ConfigPath)
	err := manager.Save(deps.Config)
	testutil.AssertNoError(t, err)

	oldStorageRepo := storage.DefaultRepository
	storage.DefaultRepository = deps.Repository
	t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

	// Create a recurring task
	actionTask, _ := models.NewTask("recur-task", project.GetNextShortID(), models.TaskTypeTask, "Daily Standup")
	actionTask.Recurrence = &models.RecurrencePattern{Interval: 1, Unit: "day"}
	project.Tasks = append(project.Tasks, actionTask)
	deps.Repository.Projects[project.ID] = project

	err = runBoardDone(boardDoneCmd, []string{fmt.Sprintf("%d", actionTask.ShortID)})
	testutil.AssertNoError(t, err)

	savedProject := deps.Repository.Projects[project.ID]

	// The completed task should have its recurrence cleared
	var completedTask *models.Task
	var newInstance *models.Task
	for _, task := range savedProject.Tasks {
		if task.ID == actionTask.ID {
			completedTask = task
		} else if task.Title == "Daily Standup" && task.Status == models.StatusTodo {
			newInstance = task
		}
	}

	if completedTask == nil {
		t.Fatal("completed task not found")
	}
	testutil.AssertEqual(t, completedTask.Status, models.StatusDone)
	if completedTask.Recurrence != nil {
		t.Error("completed task should have recurrence cleared (pattern moves to new instance)")
	}
	if newInstance == nil {
		t.Fatal("recurring instance was not generated — board done must call GenerateRecurringInstances")
	}
	if newInstance.Recurrence == nil {
		t.Error("new instance should carry the recurrence pattern")
	}
}

func TestRunBoardDone(t *testing.T) {
	setup := func(t *testing.T) (*testutil.TestDeps, *models.Project) {
		t.Helper()
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

	t.Run("rejects missing args", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardDone(boardDoneCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardDone(boardDoneCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})

	t.Run("marks action as done", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "Test Action")
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		err := runBoardDone(boardDoneCmd, []string{fmt.Sprintf("%d", actionTask.ShortID)})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		for _, task := range savedProject.Tasks {
			if task.ID == actionTask.ID {
				testutil.AssertEqual(t, task.Status, models.StatusDone)
				return
			}
		}
		t.Error("action task not found in saved project")
	})
}

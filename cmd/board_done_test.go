package cmd

import (
	"fmt"
	"strings"
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

func TestBoardDoneBlocksOnIncompleteDependencies(t *testing.T) {
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

	// Create a blocker task (todo) and a dependent task
	blockerTask, _ := models.NewTask("blocker-1", project.GetNextShortID(), models.TaskTypeTask, "Blocker Task")
	dependentTask, _ := models.NewTask("dependent-1", project.GetNextShortID(), models.TaskTypeTask, "Dependent Task")
	dependentTask.DependsOn = []string{blockerTask.ID}
	project.Tasks = append(project.Tasks, blockerTask, dependentTask)
	project.BuildTaskTree()
	deps.Repository.Projects[project.ID] = project

	err = runBoardDone(boardDoneCmd, []string{fmt.Sprintf("%d", dependentTask.ShortID)})
	if err == nil {
		t.Fatal("expected error when completing task with incomplete dependencies")
	}
	if !strings.Contains(err.Error(), "incomplete dependencies") {
		t.Errorf("expected error about incomplete dependencies, got: %s", err.Error())
	}

	// Verify the task was NOT completed
	savedProject := deps.Repository.Projects[project.ID]
	for _, task := range savedProject.Tasks {
		if task.ID == dependentTask.ID {
			if task.Status == models.StatusDone {
				t.Error("task should not be completed when it has incomplete dependencies")
			}
			return
		}
	}
}

func TestBoardDoneBlocksOnIncompleteChildren(t *testing.T) {
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

	// Create a parent epic with an incomplete child
	parentTask, _ := models.NewTask("parent-1", project.GetNextShortID(), models.TaskTypeEpic, "Parent Epic")
	childTask, _ := models.NewTask("child-1", project.GetNextShortID(), models.TaskTypeTask, "Child Task")
	childTask.ParentID = &parentTask.ID
	project.Tasks = append(project.Tasks, parentTask, childTask)
	project.BuildTaskTree()
	deps.Repository.Projects[project.ID] = project

	err = runBoardDone(boardDoneCmd, []string{fmt.Sprintf("%d", parentTask.ShortID)})
	if err == nil {
		t.Fatal("expected error when completing task with incomplete children")
	}
	if !strings.Contains(err.Error(), "incomplete children") {
		t.Errorf("expected error about incomplete children, got: %s", err.Error())
	}
}

func TestBoardDoneAutoUnblocksDependent(t *testing.T) {
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

	// Create a blocker task and a blocked dependent
	blockerTask, _ := models.NewTask("blocker-1", project.GetNextShortID(), models.TaskTypeTask, "Blocker")
	dependentTask, _ := models.NewTask("dependent-1", project.GetNextShortID(), models.TaskTypeTask, "Dependent")
	dependentTask.DependsOn = []string{blockerTask.ID}
	dependentTask.Block()
	project.Tasks = append(project.Tasks, blockerTask, dependentTask)
	project.BuildTaskTree()
	deps.Repository.Projects[project.ID] = project

	// Complete the blocker via board done
	err = runBoardDone(boardDoneCmd, []string{fmt.Sprintf("%d", blockerTask.ShortID)})
	testutil.AssertNoError(t, err)

	savedProject := deps.Repository.Projects[project.ID]
	for _, task := range savedProject.Tasks {
		if task.ID == dependentTask.ID {
			if task.Status == models.StatusBlocked {
				t.Error("dependent task should have been auto-unblocked after blocker was completed")
			}
			return
		}
	}
	t.Error("dependent task not found")
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

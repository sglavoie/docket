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

func TestBoardDismissAutoUnblocksDependent(t *testing.T) {
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

	// Dismiss the blocker via board dismiss
	err = runBoardDismiss(boardDismissCmd, []string{fmt.Sprintf("%d", blockerTask.ShortID)})
	testutil.AssertNoError(t, err)

	savedProject := deps.Repository.Projects[project.ID]
	for _, task := range savedProject.Tasks {
		if task.ID == dependentTask.ID {
			if task.Status == models.StatusBlocked {
				t.Error("dependent task should have been auto-unblocked after blocker was dismissed")
			}
			return
		}
	}
	t.Error("dependent task not found")
}

func TestRunBoardDismiss(t *testing.T) {
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

		err := runBoardDismiss(boardDismissCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardDismiss(boardDismissCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})

	t.Run("dismisses action as canceled", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "Test Action")
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		err := runBoardDismiss(boardDismissCmd, []string{fmt.Sprintf("%d", actionTask.ShortID)})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		for _, task := range savedProject.Tasks {
			if task.ID == actionTask.ID {
				testutil.AssertEqual(t, task.Status, models.StatusCanceled)
				return
			}
		}
		t.Error("action task not found in saved project")
	})
}

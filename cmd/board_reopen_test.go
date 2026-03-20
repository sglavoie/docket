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

func TestRunBoardReopen(t *testing.T) {
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

		err := runBoardReopen(boardReopenCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardReopen(boardReopenCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})

	t.Run("reopens done action", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeAction, "Test Action")
		actionTask.ApplyTransition(models.StatusDone)
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		err := runBoardReopen(boardReopenCmd, []string{fmt.Sprintf("%d", actionTask.ShortID)})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		for _, task := range savedProject.Tasks {
			if task.Type == models.TaskTypeAction {
				testutil.AssertEqual(t, task.Status, models.StatusTodo)
				return
			}
		}
		t.Error("action task not found in saved project")
	})
}

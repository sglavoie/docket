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

func TestRunBoardDelete(t *testing.T) {
	setup := func(t *testing.T) *testutil.TestDeps {
		t.Helper()
		boardDeleteForce = false
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")

		viper.Set("config", deps.ConfigPath)
		t.Cleanup(func() { viper.Set("config", "") })

		manager := config.NewManager(deps.ConfigPath)
		err := manager.Save(deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		t.Cleanup(func() { storage.DefaultRepository = oldStorageRepo })

		return deps
	}

	t.Run("rejects missing args", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardDelete(boardDeleteCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps := setup(t)
		defer deps.Cleanup()

		err := runBoardDelete(boardDeleteCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})
}

func TestBoardDeleteCleansUpReferences(t *testing.T) {
	boardDeleteForce = true
	defer func() { boardDeleteForce = false }()

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

	// Create tasks: task1 depends on task2, task3 is related to task2
	task1 := testutil.NewTestTask("Task 1", models.TaskTypeTask)
	task1.ShortID = project.GetNextShortID()
	task2 := testutil.NewTestTask("Task 2", models.TaskTypeTask)
	task2.ShortID = project.GetNextShortID()
	task3 := testutil.NewTestTask("Task 3", models.TaskTypeTask)
	task3.ShortID = project.GetNextShortID()

	_ = task1.AddDependency(task2.ID)
	_ = task3.AddRelatedTo(task2.ID)

	project.Tasks = []*models.Task{task1, task2, task3}
	project.BuildTaskTree()
	deps.Repository.Projects[project.ID] = project

	// Delete task2 by short ID
	err = runBoardDelete(boardDeleteCmd, []string{fmt.Sprintf("%d", task2.ShortID)})
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[project.ID]
	if len(saved.Tasks) != 2 {
		t.Fatalf("expected 2 tasks remaining, got %d", len(saved.Tasks))
	}

	// Verify dangling references were cleaned up
	for _, task := range saved.Tasks {
		if task.ID == task1.ID && len(task.DependsOn) != 0 {
			t.Errorf("task1 DependsOn should be empty after deleting task2, got %v", task.DependsOn)
		}
		if task.ID == task3.ID && len(task.RelatedTo) != 0 {
			t.Errorf("task3 RelatedTo should be empty after deleting task2, got %v", task.RelatedTo)
		}
	}
}

func TestBoardDeleteRemovesDescendants(t *testing.T) {
	boardDeleteForce = true
	defer func() { boardDeleteForce = false }()

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

	// Create parent epic with a child task
	parent := testutil.NewTestTask("Parent Epic", models.TaskTypeEpic)
	parent.ShortID = project.GetNextShortID()
	child := testutil.NewTestTask("Child Task", models.TaskTypeTask)
	child.ShortID = project.GetNextShortID()
	child.ParentID = &parent.ID

	// A sibling task that should survive
	sibling := testutil.NewTestTask("Sibling", models.TaskTypeTask)
	sibling.ShortID = project.GetNextShortID()

	project.Tasks = []*models.Task{parent, child, sibling}
	project.BuildTaskTree()
	deps.Repository.Projects[project.ID] = project

	// Delete the parent by short ID
	err = runBoardDelete(boardDeleteCmd, []string{fmt.Sprintf("%d", parent.ShortID)})
	testutil.AssertNoError(t, err)

	saved := deps.Repository.Projects[project.ID]
	if len(saved.Tasks) != 1 {
		t.Fatalf("expected 1 task remaining (sibling), got %d", len(saved.Tasks))
	}
	if saved.Tasks[0].ID != sibling.ID {
		t.Errorf("expected remaining task to be sibling, got %s", saved.Tasks[0].Title)
	}
}

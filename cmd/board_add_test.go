package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetBoardAddVars() {
	boardAddURL = ""
	boardAddSource = ""
	boardAddPriority = 0
	boardAddTags = nil
	boardAddProject = ""
}

func setupBoardAddTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetBoardAddVars()

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

func TestRunBoardAdd(t *testing.T) {
	t.Run("rejects empty title", func(t *testing.T) {
		deps, _ := setupBoardAddTest(t)
		defer deps.Cleanup()

		err := runBoardAdd(boardAddCmd, []string{})
		if err == nil {
			t.Error("expected error for empty title")
		}
	})

	t.Run("rejects whitespace-only title", func(t *testing.T) {
		deps, _ := setupBoardAddTest(t)
		defer deps.Cleanup()

		err := runBoardAdd(boardAddCmd, []string{"  "})
		if err == nil {
			t.Error("expected error for whitespace-only title")
		}
	})

	t.Run("creates action successfully", func(t *testing.T) {
		deps, project := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = project.Slug
		initialCount := len(project.Tasks)

		err := runBoardAdd(boardAddCmd, []string{"Test Action"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		if len(savedProject.Tasks) != initialCount+1 {
			t.Fatalf("expected %d tasks, got %d", initialCount+1, len(savedProject.Tasks))
		}
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Title, "Test Action")
		testutil.AssertEqual(t, task.Type, models.TaskTypeAction)
		testutil.AssertEqual(t, task.Status, models.StatusTodo)
	})

	t.Run("respects priority flag", func(t *testing.T) {
		deps, project := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = project.Slug
		_ = boardAddCmd.Flags().Set("priority", "2")
		defer func() { boardAddCmd.Flags().Lookup("priority").Changed = false }()

		err := runBoardAdd(boardAddCmd, []string{"Priority Action"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		testutil.AssertEqual(t, task.Priority, 2)
	})

	t.Run("respects tags", func(t *testing.T) {
		deps, project := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = project.Slug
		boardAddTags = []string{"planning", "review"}

		err := runBoardAdd(boardAddCmd, []string{"Tagged Action"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		task := savedProject.Tasks[len(savedProject.Tasks)-1]
		if !task.HasTag("planning") {
			t.Error("task should have 'planning' tag")
		}
		if !task.HasTag("review") {
			t.Error("task should have 'review' tag")
		}
	})

	t.Run("rejects invalid priority too low", func(t *testing.T) {
		deps, project := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = project.Slug
		_ = boardAddCmd.Flags().Set("priority", "0")
		defer func() { boardAddCmd.Flags().Lookup("priority").Changed = false }()

		err := runBoardAdd(boardAddCmd, []string{"Bad Priority"})
		if err == nil {
			t.Error("expected error for priority 0")
		}
	})

	t.Run("rejects invalid priority too high", func(t *testing.T) {
		deps, project := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = project.Slug
		_ = boardAddCmd.Flags().Set("priority", "6")
		defer func() { boardAddCmd.Flags().Lookup("priority").Changed = false }()

		err := runBoardAdd(boardAddCmd, []string{"Bad Priority"})
		if err == nil {
			t.Error("expected error for priority 6")
		}
	})

	t.Run("rejects nonexistent project slug", func(t *testing.T) {
		deps, _ := setupBoardAddTest(t)
		defer deps.Cleanup()

		boardAddProject = "nonexistent-project"

		err := runBoardAdd(boardAddCmd, []string{"Some Action"})
		if err == nil {
			t.Error("expected error for nonexistent project")
		}
	})
}

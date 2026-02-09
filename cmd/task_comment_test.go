package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestTaskCommentAdd(t *testing.T) {
	t.Run("adds comment to task", func(t *testing.T) {
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

		// Add a test task
		task := testutil.NewTestTask("Task with comment", models.TaskTypeTask)
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		err = runTaskCommentAdd(nil, []string{"1", "This", "is", "a", "comment"})
		testutil.AssertNoError(t, err)

		// Verify comment was added
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if savedTask == nil {
			t.Fatal("task not found after save")
		}

		if len(savedTask.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(savedTask.Comments))
		}

		if savedTask.Comments[0].Text != "This is a comment" {
			t.Errorf("comment text = %q, want %q", savedTask.Comments[0].Text, "This is a comment")
		}
	})

	t.Run("rejects empty comment", func(t *testing.T) {
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

		task := testutil.NewTestTask("Task", models.TaskTypeTask)
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		err = runTaskCommentAdd(nil, []string{"1", ""})
		if err == nil {
			t.Error("runTaskCommentAdd() should error for empty comment")
		}
	})

	t.Run("rejects nonexistent task", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err := runTaskCommentAdd(nil, []string{"999", "comment"})
		if err == nil {
			t.Error("runTaskCommentAdd() should error for nonexistent task")
		}
	})
}

func TestTaskCommentRemove(t *testing.T) {
	t.Run("removes comment by index", func(t *testing.T) {
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

		// Add task with comments
		// Use project's first task (epic from NewTestProjectWithTasks)
		task := project.Tasks[0]
		task.AddComment("First comment")
		task.AddComment("Second comment")
		err = deps.Repository.Save(project)
		testutil.AssertNoError(t, err)

		err = runTaskCommentRemove(nil, []string{"1", "0"})
		testutil.AssertNoError(t, err)

		// Verify first comment was removed
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if len(savedTask.Comments) != 1 {
			t.Fatalf("expected 1 comment, got %d", len(savedTask.Comments))
		}
		if savedTask.Comments[0].Text != "Second comment" {
			t.Errorf("remaining comment = %q, want %q", savedTask.Comments[0].Text, "Second comment")
		}
	})

	t.Run("clears all comments with flag", func(t *testing.T) {
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

		// Add task with multiple comments
		task := testutil.NewTestTask("Task with comments", models.TaskTypeTask)
		task.AddComment("Comment 1")
		task.AddComment("Comment 2")
		task.AddComment("Comment 3")
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		// Set the clear flag
		commentClearFlag = true
		defer func() { commentClearFlag = false }()

		err = runTaskCommentRemove(nil, []string{"1"})
		testutil.AssertNoError(t, err)

		// Verify all comments were removed
		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if len(savedTask.Comments) != 0 {
			t.Errorf("expected 0 comments, got %d", len(savedTask.Comments))
		}
	})

	t.Run("rejects invalid index", func(t *testing.T) {
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

		task := testutil.NewTestTask("Task", models.TaskTypeTask)
		task.AddComment("Comment")
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		err = runTaskCommentRemove(nil, []string{"1", "999"})
		if err == nil {
			t.Error("runTaskCommentRemove() should error for invalid index")
		}
	})
}

func TestTaskCommentEdit(t *testing.T) {
	t.Run("validates comment index", func(t *testing.T) {
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

		task := testutil.NewTestTask("Task", models.TaskTypeTask)
		task.AddComment("Comment")
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		// Test out of range index
		err = runTaskCommentEdit(nil, []string{"1", "999"})
		if err == nil {
			t.Error("runTaskCommentEdit() should error for out of range index")
		}

		// Test negative index
		err = runTaskCommentEdit(nil, []string{"1", "-1"})
		if err == nil {
			t.Error("runTaskCommentEdit() should error for negative index")
		}
	})

	t.Run("rejects nonexistent task", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err := runTaskCommentEdit(nil, []string{"999", "0"})
		if err == nil {
			t.Error("runTaskCommentEdit() should error for nonexistent task")
		}
	})

	t.Run("rejects invalid index format", func(t *testing.T) {
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

		task := testutil.NewTestTask("Task", models.TaskTypeTask)
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		err = runTaskCommentEdit(nil, []string{"1", "abc"})
		if err == nil {
			t.Error("runTaskCommentEdit() should error for non-numeric index")
		}
	})
}

func TestTaskCommentResolve(t *testing.T) {
	t.Run("toggles resolved state", func(t *testing.T) {
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

		// Use existing task from test project and add comments
		task := project.Tasks[0]
		task.AddComment("First comment")
		task.AddComment("Second comment")
		err = deps.Repository.Save(project)
		testutil.AssertNoError(t, err)

		// Resolve first comment
		err = runTaskCommentResolve(nil, []string{"1", "0"})
		testutil.AssertNoError(t, err)

		savedProject := deps.Repository.Projects[project.ID]
		savedTask := savedProject.GetTaskByShortID(task.ShortID)
		if !savedTask.Comments[0].Resolved {
			t.Error("comment 0 should be resolved")
		}
		if savedTask.Comments[1].Resolved {
			t.Error("comment 1 should not be resolved")
		}

		// Toggle back to unresolved
		err = runTaskCommentResolve(nil, []string{"1", "0"})
		testutil.AssertNoError(t, err)

		savedProject = deps.Repository.Projects[project.ID]
		savedTask = savedProject.GetTaskByShortID(task.ShortID)
		if savedTask.Comments[0].Resolved {
			t.Error("comment 0 should be unresolved after second toggle")
		}
	})

	t.Run("rejects invalid index", func(t *testing.T) {
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
		task.AddComment("Comment")
		err = deps.Repository.Save(project)
		testutil.AssertNoError(t, err)

		err = runTaskCommentResolve(nil, []string{"1", "999"})
		if err == nil {
			t.Error("runTaskCommentResolve() should error for invalid index")
		}
	})

	t.Run("rejects nonexistent task", func(t *testing.T) {
		deps, _ := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err := runTaskCommentResolve(nil, []string{"999", "0"})
		if err == nil {
			t.Error("runTaskCommentResolve() should error for nonexistent task")
		}
	})
}

func TestTaskCommentList_Filtering(t *testing.T) {
	t.Run("list uses real indices for resolved comments", func(t *testing.T) {
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
		task.AddComment("Unresolved comment")
		task.AddComment("Resolved comment")
		task.Comments[1].Resolved = true
		task.AddComment("Another unresolved")
		err = deps.Repository.Save(project)
		testutil.AssertNoError(t, err)

		// Without --all flag, resolved comments should be filtered
		commentListAllFlag = false
		err = runTaskCommentList(nil, []string{"1"})
		testutil.AssertNoError(t, err)

		// With --all flag, all comments should be shown
		commentListAllFlag = true
		err = runTaskCommentList(nil, []string{"1"})
		testutil.AssertNoError(t, err)
		commentListAllFlag = false // Reset
	})
}

// Note: Full integration testing of comment commands that require editor interaction
// is not included as it would require mocking external.Editor.EditContent which
// involves temporary file creation and user interaction simulation.

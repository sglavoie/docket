package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
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

		_ = task1.AddDependency(task2.ID)
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

func TestTaskDependsAddOutputAfterSave(t *testing.T) {
	t.Run("captures blocker info for output after save", func(t *testing.T) {
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

		// Capture stdout to verify output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task1.ShortID)})
		testutil.AssertNoError(t, err)

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = old
		output := buf.String()

		// Verify output contains both task IDs (confirming blocker info was captured)
		if !strings.Contains(output, fmt.Sprintf("#%d", task1.ShortID)) {
			t.Errorf("output should contain task short ID #%d, got: %s", task1.ShortID, output)
		}
		if !strings.Contains(output, fmt.Sprintf("#%d", task2.ShortID)) {
			t.Errorf("output should contain blocker short ID #%d, got: %s", task2.ShortID, output)
		}
		if !strings.Contains(output, "Added dependency") {
			t.Errorf("output should contain 'Added dependency', got: %s", output)
		}
	})
}

func TestTaskDependsRemoveOutputAfterSave(t *testing.T) {
	t.Run("captures blocker info for output after save", func(t *testing.T) {
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
		deps.Repository.Projects[project.ID] = project

		dependsOnFlag = ""
		dependsRemoveFlag = fmt.Sprintf("%d", task2.ShortID)
		dependsClearFlag = false
		defer func() { dependsRemoveFlag = "" }()

		// Capture stdout to verify output
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err = runTaskDepends(nil, []string{fmt.Sprintf("%d", task1.ShortID)})
		testutil.AssertNoError(t, err)

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = old
		output := buf.String()

		// Verify output contains both task IDs (confirming blocker info was captured)
		if !strings.Contains(output, fmt.Sprintf("#%d", task1.ShortID)) {
			t.Errorf("output should contain task short ID #%d, got: %s", task1.ShortID, output)
		}
		if !strings.Contains(output, fmt.Sprintf("#%d", task2.ShortID)) {
			t.Errorf("output should contain blocker short ID #%d, got: %s", task2.ShortID, output)
		}
		if !strings.Contains(output, "Removed dependency") {
			t.Errorf("output should contain 'Removed dependency', got: %s", output)
		}
	})
}

func TestShowTaskDependenciesCrossProject(t *testing.T) {
	t.Run("displays cross-project dependencies", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		task := project.Tasks[0]

		// Add a cross-project dependency directly (format "slug:taskID")
		task.DependsOn = append(task.DependsOn, "other-project:some-task-id")

		// Capture stdout
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		showTaskDependencies(project, task)

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = old
		output := buf.String()

		if !strings.Contains(output, "other-project:some-task-id") {
			t.Errorf("output should contain cross-project dependency ref, got: %s", output)
		}
		if !strings.Contains(output, "cross-project") {
			t.Errorf("output should indicate cross-project dependency, got: %s", output)
		}
		if strings.Contains(output, "Depends on: (none)") {
			t.Errorf("output should not say Depends on: (none) when cross-project deps exist, got: %s", output)
		}
	})

	t.Run("displays both local and cross-project dependencies", func(t *testing.T) {
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		task1 := project.Tasks[0]
		task2 := project.Tasks[1]

		// Add local dependency
		_ = task1.AddDependency(task2.ID)
		// Add cross-project dependency
		task1.DependsOn = append(task1.DependsOn, "ext-proj:ext-task-id")
		project.BuildTaskTree()

		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		showTaskDependencies(project, task1)

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		os.Stdout = old
		output := buf.String()

		if !strings.Contains(output, fmt.Sprintf("#%d", task2.ShortID)) {
			t.Errorf("output should contain local blocker short ID, got: %s", output)
		}
		if !strings.Contains(output, "ext-proj:ext-task-id") {
			t.Errorf("output should contain cross-project dependency ref, got: %s", output)
		}
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

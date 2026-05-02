package cmd

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

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

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

func TestTaskDependsAddOutputAfterSave(t *testing.T) {
	t.Run("captures blocker info for output after save", func(t *testing.T) {
		deps, project := setupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
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
		deps, project := setupTestDepsWithProject(t, "Test Project")
		defer deps.Cleanup()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
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

package cmd

import (
	"io"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestRunBoardShow(t *testing.T) {
	setup := func(t *testing.T) (*testutil.TestDeps, *models.Project) {
		t.Helper()
		deps, project := testutil.SetupTestDepsWithProject(t, "Test Project")

		boardShowJSON = false

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

	t.Run("rejects missing args with correct error message", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardShow(boardShowCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
		// Should mention "action ID", not "board column"
		if err != nil && strings.Contains(err.Error(), "board column") {
			t.Errorf("error message should not mention 'board column': %v", err)
		}
	})

	t.Run("returns error for nonexistent action ID", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardShow(boardShowCmd, []string{"999"})
		if err == nil {
			t.Error("expected error for nonexistent action ID")
		}
	})

	t.Run("json includes due_date null when no due date set", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		task, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "Show Action")
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		boardShowJSON = true

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runBoardShow(boardShowCmd, []string{"1"})

		if closeErr := w.Close(); closeErr != nil {
			t.Errorf("failed to close pipe: %v", closeErr)
		}

		os.Stdout = oldStdout
		outBytes, _ := io.ReadAll(r)
		out := string(outBytes)

		testutil.AssertNoError(t, err)

		if !strings.Contains(out, `"due_date"`) {
			t.Error("JSON output should always include due_date field, even when null")
		}
		if !strings.Contains(out, `"due_date": null`) {
			t.Error("due_date should be null when no due date is set")
		}
	})

	t.Run("text output handles short stored ID", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		task, _ := models.NewTask("abc", project.GetNextShortID(), models.TaskTypeTask, "Short ID Action")
		project.Tasks = append(project.Tasks, task)
		deps.Repository.Projects[project.ID] = project

		boardShowJSON = false

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runBoardShow(boardShowCmd, []string{strconv.Itoa(task.ShortID)})

		if closeErr := w.Close(); closeErr != nil {
			t.Errorf("failed to close pipe: %v", closeErr)
		}

		os.Stdout = oldStdout
		outBytes, _ := io.ReadAll(r)
		out := string(outBytes)

		testutil.AssertNoError(t, err)

		if !strings.Contains(out, "ID:       #"+strconv.Itoa(task.ShortID)+" (abc)") {
			t.Fatalf("expected short stored ID in output, got: %s", out)
		}
	})
}

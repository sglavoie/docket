package cmd

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetBoardListVars() {
	boardListAll = false
	boardListSource = ""
	boardListDone = false
	_ = boardListCmd.Flags().Set("json", "false")
	_ = boardListCmd.Flags().Set("quiet", "false")
}

func TestRunBoardList(t *testing.T) {
	setup := func(t *testing.T) (*testutil.TestDeps, *models.Project) {
		t.Helper()
		resetBoardListVars()

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

	t.Run("runs successfully with no actions", func(t *testing.T) {
		deps, _ := setup(t)
		defer deps.Cleanup()

		err := runBoardList(boardListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs successfully with action tasks", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "Test Action")
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		err := runBoardList(boardListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs with all flag", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "Done Action")
		actionTask.ApplyTransition(models.StatusDone)
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		boardListAll = true

		err := runBoardList(boardListCmd, []string{})
		testutil.AssertNoError(t, err)
	})

	t.Run("runs with json flag", func(t *testing.T) {
		deps, project := setup(t)
		defer deps.Cleanup()

		actionTask, _ := models.NewTask("action-1", project.GetNextShortID(), models.TaskTypeTask, "JSON Action")
		project.Tasks = append(project.Tasks, actionTask)
		deps.Repository.Projects[project.ID] = project

		_ = boardListCmd.Flags().Set("json", "true")

		err := runBoardList(boardListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

func TestBoardListJSON_IncludesTypeField(t *testing.T) {
	type actionJSONEntry struct {
		ID        string     `json:"id"`
		ShortID   int        `json:"short_id"`
		Type      string     `json:"type"`
		Title     string     `json:"title"`
		Status    string     `json:"status"`
		Priority  int        `json:"priority"`
		Tags      []string   `json:"tags"`
		CreatedAt time.Time  `json:"created_at"`
		UpdatedAt time.Time  `json:"updated_at"`
		DueDate   *time.Time `json:"due_date"`
	}

	now := time.Now()
	entry := actionJSONEntry{
		ID:        "test-id",
		ShortID:   1,
		Type:      "bug",
		Title:     "Fix login",
		Status:    "todo",
		Priority:  1,
		Tags:      []string{},
		CreatedAt: now,
		UpdatedAt: now,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if v, ok := parsed["type"]; !ok || v != "bug" {
		t.Errorf("expected type=bug in board list JSON, got %v", v)
	}
}

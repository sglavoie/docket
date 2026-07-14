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

func TestProjectList(t *testing.T) {
	t.Run("runs without error with no projects", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		err = runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("runs without error with projects", func(t *testing.T) {
		setupProjectTest(t)
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		err := runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("archived flag does not mutate config project order", func(t *testing.T) {
		deps := setupTestDeps(t)
		defer deps.Cleanup()
		defer func() {
			showArchived = false
			listProjectJSON = false
		}()

		viper.Set("config", deps.ConfigPath)
		defer viper.Set("config", "")

		oldStorageRepo := storage.DefaultRepository
		storage.DefaultRepository = deps.Repository
		defer func() { storage.DefaultRepository = oldStorageRepo }()

		// Add projects in a specific order: Z before A
		projectZ, _ := models.NewProject("id-z", "Zebra Project", deps.Config.ProjectPath("zebra-project"))
		projectA, _ := models.NewProject("id-a", "Aardvark Project", deps.Config.ProjectPath("aardvark-project"))
		addTestProject(deps.Config, projectZ)
		addTestProject(deps.Config, projectA)
		deps.Config.Machine().CurrentProject = projectZ.ID

		manager := config.NewManager(deps.ConfigPath)
		err := saveTestRuntime(manager, deps.Config)
		testutil.AssertNoError(t, err)

		// Capture original order
		originalFirst := deps.Config.AllProjects()[0].ID
		originalSecond := deps.Config.AllProjects()[1].ID

		// Run with --archived (triggers sort)
		showArchived = true
		err = runProjectList(projectListCmd, []string{})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Verify config order is unchanged
		if deps.Config.AllProjects()[0].ID != originalFirst {
			t.Errorf("config.Projects[0] was mutated: got %s, want %s", deps.Config.AllProjects()[0].ID, originalFirst)
		}
		if deps.Config.AllProjects()[1].ID != originalSecond {
			t.Errorf("config.Projects[1] was mutated: got %s, want %s", deps.Config.AllProjects()[1].ID, originalSecond)
		}
	})
}

func TestProjectListJSON_IncludesTimestampsAndArchivedAt(t *testing.T) {
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	type projectJSON struct {
		ID         string     `json:"id"`
		CreatedAt  time.Time  `json:"created_at"`
		UpdatedAt  time.Time  `json:"updated_at"`
		ArchivedAt *time.Time `json:"archived_at,omitempty"`
	}

	entry := projectJSON{
		ID:         "test-id",
		CreatedAt:  now,
		UpdatedAt:  now,
		ArchivedAt: &archivedAt,
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["created_at"]; !ok {
		t.Error("created_at field missing from project list JSON")
	}
	if _, ok := parsed["updated_at"]; !ok {
		t.Error("updated_at field missing from project list JSON")
	}
	if _, ok := parsed["archived_at"]; !ok {
		t.Error("archived_at field missing from project list JSON")
	}
}

func TestProjectListJSON_OmitsArchivedAtWhenNil(t *testing.T) {
	type projectJSON struct {
		ID         string     `json:"id"`
		ArchivedAt *time.Time `json:"archived_at,omitempty"`
	}

	entry := projectJSON{ID: "test-id", ArchivedAt: nil}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if _, ok := parsed["archived_at"]; ok {
		t.Error("archived_at should be omitted when nil")
	}
}

package cmd

import (
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func resetNoteListVars() {
	noteListTag = ""
	_ = noteListCmd.Flags().Set("json", "false")
	_ = noteListCmd.Flags().Set("quiet", "false")
	noteGlobal = false
}

func setupNoteListTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
	resetNoteListVars()

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

func TestRunNoteList(t *testing.T) {
	t.Run("runs without error for project with no notes", func(t *testing.T) {
		deps, _ := setupNoteListTest(t)
		defer deps.Cleanup()

		err := runNoteList(noteListCmd, []string{})
		testutil.AssertNoError(t, err)
	})
}

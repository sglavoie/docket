package cmd

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupProjectTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
	t.Helper()
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

func TestProjectEdit(t *testing.T) {
	t.Run("rejects when no flags provided", func(t *testing.T) {
		setupProjectTest(t)
		defer func() {
			editName = ""
			editContext = ""
			editDescription = ""
			editStatusIcon = ""
		}()

		// Create a fresh command so no flags are marked as "changed"
		cmd := &cobra.Command{}
		cmd.Flags().StringVarP(&editName, "name", "n", "", "")
		cmd.Flags().StringVarP(&editContext, "context", "c", "", "")
		cmd.Flags().StringVarP(&editDescription, "description", "d", "", "")
		cmd.Flags().StringVarP(&editStatusIcon, "status-icon", "i", "", "")

		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Error("expected error for no flags provided")
		}
	})

	t.Run("rejects empty project name", func(t *testing.T) {
		setupProjectTest(t)
		defer func() {
			editName = ""
			editContext = ""
			editDescription = ""
			editStatusIcon = ""
		}()

		// Create a command and explicitly set --name to empty string
		cmd := &cobra.Command{}
		cmd.Flags().StringVarP(&editName, "name", "n", "", "")
		cmd.Flags().StringVarP(&editContext, "context", "c", "", "")
		cmd.Flags().StringVarP(&editDescription, "description", "d", "", "")
		cmd.Flags().StringVarP(&editStatusIcon, "status-icon", "i", "", "")
		// Mark the name flag as changed
		_ = cmd.Flags().Set("name", "")

		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Error("expected error for empty project name")
		}
	})
}

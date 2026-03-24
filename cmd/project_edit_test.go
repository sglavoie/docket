package cmd

import (
	"strings"
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

func newEditCmd() *cobra.Command {
	cmd := &cobra.Command{}
	cmd.Flags().StringVarP(&editName, "name", "n", "", "")
	cmd.Flags().StringVarP(&editContext, "context", "c", "", "")
	cmd.Flags().StringVarP(&editDescription, "description", "d", "", "")
	cmd.Flags().StringVarP(&editStatusIcon, "status-icon", "i", "", "")
	cmd.Flags().BoolVar(&projectEditJSON, "json", false, "")
	cmd.Flags().BoolVar(&projectEditQuiet, "quiet", false, "")
	return cmd
}

func resetProjectEditVars() {
	editName = ""
	editContext = ""
	editDescription = ""
	editStatusIcon = ""
	projectEditJSON = false
	projectEditQuiet = false
}

func TestProjectEdit(t *testing.T) {
	t.Run("rejects when no flags provided", func(t *testing.T) {
		setupProjectTest(t)
		defer resetProjectEditVars()

		cmd := newEditCmd()
		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Error("expected error for no flags provided")
		}
	})

	t.Run("rejects empty project name", func(t *testing.T) {
		setupProjectTest(t)
		defer resetProjectEditVars()

		cmd := newEditCmd()
		_ = cmd.Flags().Set("name", "")

		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Error("expected error for empty project name")
		}
	})

	t.Run("rejects name exceeding max length", func(t *testing.T) {
		setupProjectTest(t)
		defer resetProjectEditVars()

		longName := strings.Repeat("x", models.MaxProjectNameLength+1)
		cmd := newEditCmd()
		_ = cmd.Flags().Set("name", longName)

		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Fatal("expected error for name exceeding max length")
		}
		if !strings.Contains(err.Error(), "too long") {
			t.Errorf("expected 'too long' in error, got: %s", err.Error())
		}
	})

	t.Run("rejects reserved slug on rename", func(t *testing.T) {
		setupProjectTest(t)
		defer resetProjectEditVars()

		// "projects" is a reserved slug
		cmd := newEditCmd()
		_ = cmd.Flags().Set("name", "Projects")

		err := runProjectEdit(cmd, []string{})
		if err == nil {
			t.Fatal("expected error for reserved slug")
		}
		if !strings.Contains(err.Error(), "reserved") {
			t.Errorf("expected 'reserved' in error, got: %s", err.Error())
		}
	})

	t.Run("rename updates tasks.md heading via SaveProject", func(t *testing.T) {
		deps, _ := setupProjectTest(t)
		defer resetProjectEditVars()

		cmd := newEditCmd()
		_ = cmd.Flags().Set("name", "Renamed Project")
		_ = cmd.Flags().Set("quiet", "true")

		err := runProjectEdit(cmd, []string{})
		testutil.AssertNoError(t, err)

		// Verify SaveProject was called with the new name (mock stores the saved project).
		// This ensures the tasks.md heading gets the updated project name.
		saved := deps.Repository.LastSaved
		if saved == nil {
			t.Fatal("expected SaveProject to be called after rename, but it was not")
		}
		testutil.AssertEqual(t, saved.Name, "Renamed Project")
	})
}

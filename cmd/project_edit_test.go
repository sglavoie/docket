package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupProjectTest(t *testing.T) (*testDeps, *models.Project) {
	t.Helper()
	deps, project := setupTestDepsWithProject(t, "Test Project")
	viper.Set("config", deps.ConfigPath)
	t.Cleanup(func() { viper.Set("config", "") })
	manager := config.NewManager(deps.ConfigPath)
	err := saveTestRuntime(manager, deps.Config)
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

	t.Run("rename rewrites tasks.md heading atomically", func(t *testing.T) {
		deps, _ := setupProjectTest(t)
		defer resetProjectEditVars()

		cmd := newEditCmd()
		_ = cmd.Flags().Set("name", "Renamed Project")
		_ = cmd.Flags().Set("quiet", "true")

		err := runProjectEdit(cmd, []string{})
		testutil.AssertNoError(t, err)

		// The rename commits the tasks.md heading rewrite through the app-layer
		// transaction, which writes to the new slug-derived project directory on
		// disk. Verify the renamed heading landed there.
		newPath := deps.Config.ProjectPath("renamed-project")
		content, readErr := os.ReadFile(storage.TasksFilePath(newPath))
		testutil.AssertNoError(t, readErr)
		if !strings.Contains(string(content), "Renamed Project") {
			t.Errorf("expected renamed heading in tasks.md, got:\n%s", content)
		}
	})
}

func TestRewriteRefs(t *testing.T) {
	t.Run("rewrites matching cross-project refs", func(t *testing.T) {
		refs := []string{"old-project:task-123", "other:task-456", "plain-task-id"}
		result, count := rewriteRefs(refs, "old-project", "new-project")
		if count != 1 {
			t.Errorf("expected 1 ref rewritten, got %d", count)
		}
		if result[0] != "new-project:task-123" {
			t.Errorf("expected 'new-project:task-123', got %q", result[0])
		}
		if result[1] != "other:task-456" {
			t.Errorf("expected 'other:task-456' unchanged, got %q", result[1])
		}
		if result[2] != "plain-task-id" {
			t.Errorf("expected 'plain-task-id' unchanged, got %q", result[2])
		}
	})

	t.Run("returns zero count for no matches", func(t *testing.T) {
		refs := []string{"other:task-456", "plain-task-id"}
		result, count := rewriteRefs(refs, "old-project", "new-project")
		if count != 0 {
			t.Errorf("expected 0 refs rewritten, got %d", count)
		}
		if len(result) != 2 {
			t.Errorf("expected 2 refs, got %d", len(result))
		}
	})

	t.Run("handles empty refs", func(t *testing.T) {
		result, count := rewriteRefs(nil, "old", "new")
		if count != 0 {
			t.Errorf("expected 0 refs rewritten, got %d", count)
		}
		if result != nil {
			t.Errorf("expected nil result for nil input, got %v", result)
		}
	})

	t.Run("rewrites multiple matching refs", func(t *testing.T) {
		refs := []string{"proj:task-1", "proj:task-2", "proj:task-3"}
		result, count := rewriteRefs(refs, "proj", "renamed-proj")
		if count != 3 {
			t.Errorf("expected 3 refs rewritten, got %d", count)
		}
		for i, ref := range result {
			expected := "renamed-proj:" + refs[i][len("proj:"):]
			if ref != expected {
				t.Errorf("result[%d]: expected %q, got %q", i, expected, ref)
			}
		}
	})
}

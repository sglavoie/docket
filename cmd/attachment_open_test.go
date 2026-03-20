package cmd

import (
	"fmt"
	"testing"

	"github.com/spf13/viper"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

func setupAttachmentTest(t *testing.T) (*testutil.TestDeps, *models.Project) {
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

func TestRunAttachmentOpen(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentOpen(attachmentOpenCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects single arg", func(t *testing.T) {
		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentOpen(attachmentOpenCmd, []string{"1"})
		if err == nil {
			t.Error("expected error for single arg")
		}
	})

	t.Run("task has no attachments", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		taskID := fmt.Sprintf("%d", project.Tasks[0].ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "0"})
		if err == nil {
			t.Error("expected error for task with no attachments")
		}
	})

	t.Run("invalid attachment index", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "test.txt",
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "abc"})
		if err == nil {
			t.Error("expected error for invalid attachment index")
		}
	})

	t.Run("attachment index out of range", func(t *testing.T) {
		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "test.txt",
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentOpen(attachmentOpenCmd, []string{taskID, "5"})
		if err == nil {
			t.Error("expected error for attachment index out of range")
		}
	})
}

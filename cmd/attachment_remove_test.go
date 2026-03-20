package cmd

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func resetAttachmentRemoveVars() {
	keepFile = false
}

func TestRunAttachmentRemove(t *testing.T) {
	t.Run("rejects missing args", func(t *testing.T) {
		resetAttachmentRemoveVars()

		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentRemove(attachmentRemoveCmd, []string{})
		if err == nil {
			t.Error("expected error for missing args")
		}
	})

	t.Run("rejects single arg", func(t *testing.T) {
		resetAttachmentRemoveVars()

		deps, _ := setupAttachmentTest(t)
		defer deps.Cleanup()

		err := runAttachmentRemove(attachmentRemoveCmd, []string{"1"})
		if err == nil {
			t.Error("expected error for single arg")
		}
	})

	t.Run("task has no attachments", func(t *testing.T) {
		resetAttachmentRemoveVars()

		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		taskID := fmt.Sprintf("%d", project.Tasks[0].ShortID)
		err := runAttachmentRemove(attachmentRemoveCmd, []string{taskID, "0"})
		if err == nil {
			t.Error("expected error for task with no attachments")
		}
	})

	t.Run("invalid attachment index", func(t *testing.T) {
		resetAttachmentRemoveVars()

		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "test.txt",
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentRemove(attachmentRemoveCmd, []string{taskID, "abc"})
		if err == nil {
			t.Error("expected error for invalid attachment index")
		}
	})
}

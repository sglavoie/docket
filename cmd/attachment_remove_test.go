package cmd

import (
	"fmt"
	"strings"
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
		if !strings.Contains(err.Error(), "tt attachments") {
			t.Fatalf("expected error to mention valid attachments command, got %q", err.Error())
		}
		if strings.Contains(err.Error(), "tt attachment list") {
			t.Fatalf("expected error not to mention missing attachment list command, got %q", err.Error())
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

	// Regression: removing a URL attachment must save the project so the
	// metadata change is persisted. Previously the file deletion happened
	// before save, but for URLs the save is the only persistence step —
	// any save failure or wrong ordering would leave the URL in tasks.md
	// while in-memory metadata claimed it was removed.
	t.Run("url attachment removal persists metadata change", func(t *testing.T) {
		resetAttachmentRemoveVars()
		attachmentRemoveForce = true
		defer func() { attachmentRemoveForce = false }()

		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename: "https://example.com",
			IsURL:    true,
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentRemove(attachmentRemoveCmd, []string{taskID, "0"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		savedProject := deps.Repository.Projects[project.ID]
		var saved *models.Task
		for _, st := range savedProject.Tasks {
			if st.ID == task.ID {
				saved = st
				break
			}
		}
		if saved == nil {
			t.Fatal("saved task not found")
		}
		if len(saved.Attachments) != 0 {
			t.Fatalf("expected attachment to be removed, found %d", len(saved.Attachments))
		}
	})

	t.Run("code reference removal does not attempt file deletion", func(t *testing.T) {
		resetAttachmentRemoveVars()
		attachmentRemoveForce = true
		defer func() { attachmentRemoveForce = false }()

		deps, project := setupAttachmentTest(t)
		defer deps.Cleanup()

		task := project.Tasks[0]
		task.Attachments = append(task.Attachments, models.Attachment{
			Filename:  "src/main.go:42",
			IsCodeRef: true,
		})

		taskID := fmt.Sprintf("%d", task.ShortID)
		err := runAttachmentRemove(attachmentRemoveCmd, []string{taskID, "0"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		savedProject := deps.Repository.Projects[project.ID]
		var saved *models.Task
		for _, st := range savedProject.Tasks {
			if st.ID == task.ID {
				saved = st
				break
			}
		}
		if saved == nil {
			t.Fatal("saved task not found")
		}
		if len(saved.Attachments) != 0 {
			t.Fatalf("expected code-ref attachment removed, found %d", len(saved.Attachments))
		}
	})
}

package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestCleanupTaskAttachments_EmptyIDDoesNotWipeAttachmentsDir guards against a
// data-loss bug where a task with an empty ID would cause CleanupTaskAttachments
// to delete the project's entire .attachments directory (because the joined
// path collapses to the parent attachments dir). Cleanup must be a no-op when
// the task ID is empty.
func TestCleanupTaskAttachments_EmptyIDDoesNotWipeAttachmentsDir(t *testing.T) {
	project := newTestProject(t)

	// Populate the .attachments dir with another task's attachment so we can
	// verify it survives the bad cleanup call.
	good := &models.Task{ID: "real-task-id", Title: "Real"}
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "keep.txt")
	if err := os.WriteFile(srcPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(project, good, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}
	keepPath := GetAttachmentPath(project, good, "keep.txt")

	// Cleanup a task with an empty ID; this must NOT wipe the parent
	// attachments directory.
	bad := &models.Task{ID: "", Title: "Empty ID"}
	if err := CleanupTaskAttachments(project, bad); err != nil {
		t.Fatalf("CleanupTaskAttachments returned error for empty-id task: %v", err)
	}

	if _, err := os.Stat(keepPath); err != nil {
		t.Fatalf("attachment for other task was wiped by cleanup with empty ID: %v", err)
	}
}

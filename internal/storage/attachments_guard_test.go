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

// TestCopyAttachment_EmptyIDDoesNotWriteToAttachmentsRoot guards against
// storing a task attachment directly in the project's .attachments root when a
// malformed task has no UUID. That root is reserved for per-task directories.
func TestCopyAttachment_EmptyIDDoesNotWriteToAttachmentsRoot(t *testing.T) {
	project := newTestProject(t)

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "orphan.txt")
	if err := os.WriteFile(srcPath, []byte("orphan"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	bad := &models.Task{ID: "", Title: "Empty ID"}
	if _, err := CopyAttachment(project, bad, srcPath); err == nil {
		t.Fatal("expected CopyAttachment to reject an empty task ID")
	}

	if _, err := os.Stat(filepath.Join(getAttachmentsDir(project), "orphan.txt")); !os.IsNotExist(err) {
		t.Fatalf("attachment was written to .attachments root, stat err: %v", err)
	}
}

func TestAttachmentHelpersRejectEmptyTaskID(t *testing.T) {
	project := newTestProject(t)
	good := &models.Task{ID: "real-task-id", Title: "Real"}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "keep.txt")
	if err := os.WriteFile(srcPath, []byte("keep me"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(project, good, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	bad := &models.Task{ID: "", Title: "Empty ID"}
	if err := RemoveAttachmentFile(project, bad, "keep.txt"); err == nil {
		t.Fatal("expected RemoveAttachmentFile to reject an empty task ID")
	}
	if AttachmentExists(project, bad, "keep.txt") {
		t.Fatal("AttachmentExists should return false for an empty task ID")
	}
	if size := GetAttachmentSize(project, bad, &models.Attachment{Filename: "keep.txt"}); size != 0 {
		t.Fatalf("GetAttachmentSize with empty task ID = %d, want 0", size)
	}

	if !AttachmentExists(project, good, "keep.txt") {
		t.Fatal("attachment for valid task should remain untouched")
	}
}

func TestAttachmentHelpersHandleNilInputs(t *testing.T) {
	project := newTestProject(t)

	if err := RemoveAttachmentFile(project, nil, "keep.txt"); err == nil {
		t.Fatal("expected RemoveAttachmentFile to reject a nil task")
	}
	if AttachmentExists(project, nil, "keep.txt") {
		t.Fatal("AttachmentExists should return false for a nil task")
	}
	if size := GetAttachmentSize(project, nil, &models.Attachment{Filename: "keep.txt"}); size != 0 {
		t.Fatalf("GetAttachmentSize with nil task = %d, want 0", size)
	}

	task := &models.Task{ID: "real-task-id", Title: "Real"}
	if size := GetAttachmentSize(project, task, nil); size != 0 {
		t.Fatalf("GetAttachmentSize with nil attachment = %d, want 0", size)
	}
}

func TestResolveAttachmentPathRejectsUntrustedNames(t *testing.T) {
	project := newTestProject(t)
	task := &models.Task{ID: "real-task-id", Title: "Real"}

	for _, filename := range []string{"../escape.txt", "/tmp/escape.txt", ".", `nested\\escape.txt`} {
		if path, err := ResolveAttachmentPath(project, task, filename); err == nil {
			t.Errorf("ResolveAttachmentPath(%q) = %q, want error", filename, path)
		}
	}

	path, err := ResolveAttachmentPath(project, task, "safe.txt")
	if err != nil {
		t.Fatalf("ResolveAttachmentPath(safe.txt): %v", err)
	}
	if filepath.Base(path) != "safe.txt" {
		t.Fatalf("resolved path = %q, want safe.txt basename", path)
	}
}

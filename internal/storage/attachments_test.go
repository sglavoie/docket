package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

func newTestTask() *models.Task {
	return &models.Task{
		ID:    "test-task-uuid",
		Title: "Test Task",
	}
}

func TestCopyAttachment(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	// Create a source file
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "hello.txt")
	if err := os.WriteFile(srcPath, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	filename, err := CopyAttachment(project, task, srcPath)
	if err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}
	if filename != "hello.txt" {
		t.Errorf("filename = %q, want %q", filename, "hello.txt")
	}

	// Verify the file was copied
	destPath := GetAttachmentPath(project, task, filename)
	data, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("failed to read copied file: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("copied content = %q, want %q", string(data), "hello world")
	}
}

func TestCopyAttachmentDuplicateBlocked(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "dup.txt")
	if err := os.WriteFile(srcPath, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// First copy succeeds
	if _, err := CopyAttachment(project, task, srcPath); err != nil {
		t.Fatalf("first CopyAttachment failed: %v", err)
	}

	// Second copy should fail with "already exists"
	_, err := CopyAttachment(project, task, srcPath)
	if err == nil {
		t.Fatal("expected error on duplicate attachment, got nil")
	}
}

func TestCopyAttachmentCleansUpOnSourceError(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	// Pass a source path that doesn't exist
	_, err := CopyAttachment(project, task, "/nonexistent/file.txt")
	if err == nil {
		t.Fatal("expected error for nonexistent source, got nil")
	}

	// The destination file should not exist (no leftover partial file)
	destPath := GetAttachmentPath(project, task, "file.txt")
	if _, statErr := os.Stat(destPath); !os.IsNotExist(statErr) {
		t.Error("destination file should not exist after failed copy")
	}
}

func TestCopyAttachmentCleansUpPartialFile(t *testing.T) {
	// This test verifies the fix: if a destination file is created but the
	// copy fails, the partial file is removed so a retry is not blocked.
	project := newTestProject(t)
	task := newTestTask()

	// Create a real source file
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "retry.txt")
	if err := os.WriteFile(srcPath, []byte("good content"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}

	// Pre-create the destination directory and a file to simulate a prior
	// partial write that was cleaned up
	if err := EnsureTaskAttachmentsDir(project, task); err != nil {
		t.Fatalf("EnsureTaskAttachmentsDir failed: %v", err)
	}
	destPath := GetAttachmentPath(project, task, "retry.txt")

	// Write a partial/corrupt file manually
	if err := os.WriteFile(destPath, []byte("partial"), 0o644); err != nil {
		t.Fatalf("failed to create partial file: %v", err)
	}

	// CopyAttachment should fail because the file already exists
	_, err := CopyAttachment(project, task, srcPath)
	if err == nil {
		t.Fatal("expected error when destination already exists, got nil")
	}

	// Remove the file manually to simulate cleanup, then retry
	if rmErr := os.Remove(destPath); rmErr != nil {
		t.Fatalf("failed to remove partial file: %v", rmErr)
	}

	// Retry should succeed
	filename, err := CopyAttachment(project, task, srcPath)
	if err != nil {
		t.Fatalf("retry CopyAttachment failed: %v", err)
	}
	if filename != "retry.txt" {
		t.Errorf("filename = %q, want %q", filename, "retry.txt")
	}
}

func TestRemoveAttachmentFile(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	// Create an attachment
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "remove-me.txt")
	if err := os.WriteFile(srcPath, []byte("bye"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(project, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	// Remove it
	if err := RemoveAttachmentFile(project, task, "remove-me.txt"); err != nil {
		t.Fatalf("RemoveAttachmentFile failed: %v", err)
	}

	// Should not exist
	if AttachmentExists(project, task, "remove-me.txt") {
		t.Error("attachment should not exist after removal")
	}

	// Removing a non-existent file should not error
	if err := RemoveAttachmentFile(project, task, "nonexistent.txt"); err != nil {
		t.Errorf("removing nonexistent file should not error, got: %v", err)
	}
}

func TestRemoveAttachmentFileRejectsPathTraversal(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()
	otherTask := &models.Task{ID: "other-task-uuid", Title: "Other Task"}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "keep.txt")
	if err := os.WriteFile(srcPath, []byte("keep"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(project, otherTask, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	err := RemoveAttachmentFile(project, task, "../"+otherTask.ID+"/keep.txt")
	if err == nil {
		t.Fatal("expected traversal filename to be rejected")
	}

	if !AttachmentExists(project, otherTask, "keep.txt") {
		t.Fatal("attachment for other task should not be removed")
	}
	if AttachmentExists(project, task, "../"+otherTask.ID+"/keep.txt") {
		t.Fatal("traversal filename should not be reported as an attachment")
	}
	traversalAttachment := &models.Attachment{Filename: "../" + otherTask.ID + "/keep.txt"}
	if size := GetAttachmentSize(project, task, traversalAttachment); size != 0 {
		t.Fatalf("traversal attachment size = %d, want 0", size)
	}
}

// TestMoveTaskAttachments_MovesFiles guards against a cross-project move
// (`tt task move-to`) leaving attachment files behind in the source project's
// .attachments/<task-uuid>/ directory. The task UUID is preserved by the move,
// so the destination project must take ownership of the on-disk attachments.
func TestMoveTaskAttachments_MovesFiles(t *testing.T) {
	src := newTestProject(t)
	dest := &models.Project{ID: "dest-project", Name: "Dest", Path: t.TempDir()}
	task := newTestTask()

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "spec.pdf")
	if err := os.WriteFile(srcPath, []byte("pdf-bytes"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(src, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	if err := MoveTaskAttachments(src, dest, task); err != nil {
		t.Fatalf("MoveTaskAttachments failed: %v", err)
	}

	if AttachmentExists(src, task, "spec.pdf") {
		t.Error("attachment should not exist in source after move")
	}
	if !AttachmentExists(dest, task, "spec.pdf") {
		t.Error("attachment should exist in destination after move")
	}

	srcAttachDir := GetTaskAttachmentsDir(src, task)
	if _, err := os.Stat(srcAttachDir); !os.IsNotExist(err) {
		t.Errorf("source task attachments dir should be gone, stat err = %v", err)
	}

	got, err := os.ReadFile(GetAttachmentPath(dest, task, "spec.pdf"))
	if err != nil {
		t.Fatalf("read moved file: %v", err)
	}
	if string(got) != "pdf-bytes" {
		t.Errorf("moved content = %q, want %q", string(got), "pdf-bytes")
	}
}

// TestMoveTaskAttachments_NoSourceDir is a no-op when the task has no
// on-disk attachments (e.g. URL-only or code-ref-only tasks).
func TestMoveTaskAttachments_NoSourceDir(t *testing.T) {
	src := newTestProject(t)
	dest := &models.Project{ID: "dest-project", Name: "Dest", Path: t.TempDir()}
	task := newTestTask()

	if err := MoveTaskAttachments(src, dest, task); err != nil {
		t.Errorf("MoveTaskAttachments with no source dir should be no-op, got: %v", err)
	}
}

func TestCleanupTaskAttachments(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	srcDir := t.TempDir()
	for _, name := range []string{"a.txt", "b.txt"} {
		path := filepath.Join(srcDir, name)
		if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
			t.Fatalf("failed to create %s: %v", name, err)
		}
		if _, err := CopyAttachment(project, task, path); err != nil {
			t.Fatalf("CopyAttachment %s failed: %v", name, err)
		}
	}

	if err := CleanupTaskAttachments(project, task); err != nil {
		t.Fatalf("CleanupTaskAttachments failed: %v", err)
	}

	dir := GetTaskAttachmentsDir(project, task)
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("task attachments directory should not exist after cleanup")
	}
}

func TestAttachmentExists(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	if AttachmentExists(project, task, "nope.txt") {
		t.Error("should return false for non-existent attachment")
	}

	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "exists.txt")
	if err := os.WriteFile(srcPath, []byte("hi"), 0o644); err != nil {
		t.Fatalf("failed to create source file: %v", err)
	}
	if _, err := CopyAttachment(project, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}

	if !AttachmentExists(project, task, "exists.txt") {
		t.Error("should return true for existing attachment")
	}
}

func TestGetAttachmentSize(t *testing.T) {
	project := newTestProject(t)
	task := newTestTask()

	// URL attachments return 0
	urlAttachment := &models.Attachment{Filename: "http://example.com", IsURL: true}
	if size := GetAttachmentSize(project, task, urlAttachment); size != 0 {
		t.Errorf("URL attachment size = %d, want 0", size)
	}

	// Non-existent file returns 0
	missingAttachment := &models.Attachment{Filename: "missing.txt"}
	if size := GetAttachmentSize(project, task, missingAttachment); size != 0 {
		t.Errorf("missing attachment size = %d, want 0", size)
	}

	// Real file returns actual size
	srcDir := t.TempDir()
	srcPath := filepath.Join(srcDir, "sized.txt")
	content := "twelve bytes"
	if err := os.WriteFile(srcPath, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}
	if _, err := CopyAttachment(project, task, srcPath); err != nil {
		t.Fatalf("CopyAttachment failed: %v", err)
	}
	realAttachment := &models.Attachment{Filename: "sized.txt"}
	size := GetAttachmentSize(project, task, realAttachment)
	if size != int64(len(content)) {
		t.Errorf("size = %d, want %d", size, len(content))
	}
}

func TestFormatFileSize(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		got := FormatFileSize(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatFileSize(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestParseCodeReference(t *testing.T) {
	tests := []struct {
		input     string
		wantLine  int
		wantIsRef bool
	}{
		{"plain text", 0, false},
		{"no-colon", 0, false},
		{"file.go:0", 0, false},  // line 0 is invalid
		{"file.go:-1", 0, false}, // negative
		{"file.go:abc", 0, false},
	}
	for _, tt := range tests {
		_, line, isRef := ParseCodeReference(tt.input)
		if line != tt.wantLine || isRef != tt.wantIsRef {
			t.Errorf("ParseCodeReference(%q) = (_, %d, %v), want (_, %d, %v)",
				tt.input, line, isRef, tt.wantLine, tt.wantIsRef)
		}
	}

	// Test with a file that actually exists
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.go")
	if err := os.WriteFile(tmpFile, []byte("package main"), 0o644); err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	path, line, isRef := ParseCodeReference(tmpFile + ":42")
	if !isRef {
		t.Error("expected isCodeRef=true for existing file")
	}
	if line != 42 {
		t.Errorf("line = %d, want 42", line)
	}
	if path != tmpFile {
		t.Errorf("path = %q, want %q", path, tmpFile)
	}
}

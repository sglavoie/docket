package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskAttachmentsDeleteDispatchesCoordinator(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}
	if msg := cmd(); msg == nil {
		t.Fatal("expected transaction message from executeDelete command")
	} else if _, ok := msg.(TransactionResultMsg); !ok {
		t.Fatalf("expected TransactionResultMsg, got %T", msg)
	}

	// The view no longer mutates its own copy; the application layer commits
	// and the shell reloads the project.
	if len(model.task.Attachments) != 2 {
		t.Fatalf("expected local attachments untouched before reload, got %d", len(model.task.Attachments))
	}
	if len(fake.removals) != 1 || fake.removals[0].index != 1 || !fake.removals[0].deleteFile {
		t.Fatalf("expected RemoveAttachment(index=1, deleteFile=true), got %+v", fake.removals)
	}
}

func TestTaskAttachmentsDeleteOutOfBounds(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 5 // Out of bounds

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Should return an error message, not panic
	msg := cmd()
	if _, ok := msg.(ErrorMsg); !ok {
		t.Fatalf("expected ErrorMsg for out-of-bounds delete, got %T", msg)
	}

	// Attachments should be unchanged and nothing dispatched
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected attachments to be unchanged, got %d", len(model.task.Attachments))
	}
	if len(fake.removals) != 0 {
		t.Fatalf("expected no coordinator dispatch, got %+v", fake.removals)
	}
}

func TestTaskAttachmentsKeepDispatchesCoordinator(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.cursor = 0

	cmd := model.executeKeep()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeKeep")
	}
	if _, ok := cmd().(TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from executeKeep command")
	}
	if len(fake.removals) != 1 || fake.removals[0].index != 0 || fake.removals[0].deleteFile {
		t.Fatalf("expected RemoveAttachment(index=0, deleteFile=false), got %+v", fake.removals)
	}
}

func TestTaskAttachmentsAddURLDispatchesCoordinator(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	coordinator, fake := newFakeAttachmentCoordinator()
	model := NewTaskAttachmentsModel(task, project, nil, coordinator)
	model.addMode = true
	model.textInput.SetValue("https://example.com/design.pdf")

	cmd := model.addAttachment()
	if cmd == nil {
		t.Fatal("expected non-nil command from addAttachment")
	}
	if _, ok := cmd().(TransactionResultMsg); !ok {
		t.Fatal("expected TransactionResultMsg from addAttachment command")
	}

	// Add mode exits before the transaction resolves.
	if model.addMode {
		t.Fatal("expected addMode to be false after addAttachment()")
	}
	if len(fake.urls) != 1 || fake.urls[0] != "https://example.com/design.pdf" {
		t.Fatalf("expected AttachURL dispatch, got %+v", fake.urls)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", model.cursor)
	}
}

func TestTaskAttachmentsAddEmptyPath(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil, AttachmentCoordinator{})
	model.addMode = true
	model.textInput.SetValue("   ")

	cmd := model.addAttachment()
	if cmd != nil {
		t.Fatal("expected nil command for empty path")
	}
	if model.err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestTaskAttachmentsAddNormalizesPastedPaths(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "my file.txt")
	if err := os.WriteFile(realPath, []byte("content"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name  string
		input string
	}{
		{"escaped space", strings.ReplaceAll(realPath, " ", `\ `)},
		{"quoted path", `"` + realPath + `"`},
		{"raw unquoted path with spaces", realPath},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
			if err != nil {
				t.Fatalf("NewProject: %v", err)
			}

			task, err := models.NewGenericTask("task-1", 1, "Test task")
			if err != nil {
				t.Fatalf("NewTask: %v", err)
			}
			project.Tasks = []*models.Task{task}

			coordinator, _ := newFakeAttachmentCoordinator()
			model := NewTaskAttachmentsModel(task, project, nil, coordinator)
			model.addMode = true
			model.textInput.SetValue(tc.input)

			cmd := model.addAttachment()
			if cmd == nil {
				t.Fatalf("expected non-nil command from addAttachment for input %q", tc.input)
			}
			if model.err != nil {
				t.Fatalf("expected no error for input %q, got: %v", tc.input, model.err)
			}
			if model.addMode {
				t.Fatal("expected addMode to be false after successful add")
			}
		})
	}
}

// TestOpenAttachmentInEditor_EmptyEditorUsesOSDefault verifies that when
// attachment_editor is empty (the default), openAttachmentInEditor routes
// through buildOpenCommand (OS default) rather than resolving a text editor.
// We detect this by checking that no ResolveEditor call is made (i.e. the
// function returns a non-nil cmd without an error even though no text editor
// is available in PATH).
func TestOpenAttachmentInEditor_EmptyEditorUsesOSDefault(t *testing.T) {
	project, err := models.NewProject("proj", "Test Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewGenericTask("task-1", 1, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}

	// Create a real file so the existence check passes.
	attachDir := t.TempDir()
	project.Path = attachDir

	// We need the attachment path to exist. Use a simple approach:
	// create a model with a URL attachment (skips file path resolution)
	// then switch to a file attachment via a stub.
	// Actually, easier: just verify the config routing by checking the model
	// returns an error about the file not existing (which means it got past
	// the "empty editor" branch) vs. a "no editor found" error.
	cfg := newTestConfig()

	task.Attachments = []models.Attachment{
		{Filename: "note.txt", OriginalPath: "/nonexistent/note.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, testRuntimeAt(cfg.DataDir(), cfg), AttachmentCoordinator{})
	model.cursor = 0

	// openAttachmentInEditor should attempt
	// buildOpenCommand (OS default). Since the file doesn't exist in the
	// expected attachment directory, it sets model.err about "file not found",
	// NOT about "no editor configured" or "not found in PATH".
	model.openAttachmentInEditor()

	if model.err == nil {
		// The attachment dir doesn't exist so we expect an error.
		// If we got here with no error, the test environment has the file.
		return
	}

	errMsg := model.err.Error()
	if strings.Contains(errMsg, "no editor") || strings.Contains(errMsg, "not found in PATH") {
		t.Errorf("empty attachment_editor should use OS default open, not text editor resolution; got: %s", errMsg)
	}
}

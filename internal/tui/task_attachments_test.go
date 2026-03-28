package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
)

func TestTaskAttachmentsDeleteMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil)
	model.cursor = 1

	cmd := model.executeDelete()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeDelete")
	}

	// Verify state was mutated synchronously before cmd() runs
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected 1 attachment after delete, got %d", len(model.task.Attachments))
	}
	if model.task.Attachments[0].Filename != "file1.txt" {
		t.Fatalf("expected remaining attachment to be 'file1.txt', got %q", model.task.Attachments[0].Filename)
	}
	if model.cursor != 0 {
		t.Fatalf("expected cursor at 0 after deleting last attachment, got %d", model.cursor)
	}
}

func TestTaskAttachmentsDeleteOutOfBounds(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil)
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

	// Attachments should be unchanged
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected attachments to be unchanged, got %d", len(model.task.Attachments))
	}
}

func TestTaskAttachmentsKeepMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.Attachments = []models.Attachment{
		{Filename: "file1.txt", OriginalPath: "/tmp/file1.txt", AddedAt: time.Now()},
		{Filename: "file2.txt", OriginalPath: "/tmp/file2.txt", AddedAt: time.Now()},
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil)
	model.cursor = 0

	cmd := model.executeKeep()
	if cmd == nil {
		t.Fatal("expected non-nil command from executeKeep")
	}

	// Verify state was mutated synchronously before cmd() runs
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected 1 attachment after unlink, got %d", len(model.task.Attachments))
	}
	if model.task.Attachments[0].Filename != "file2.txt" {
		t.Fatalf("expected remaining attachment to be 'file2.txt', got %q", model.task.Attachments[0].Filename)
	}
}

func TestTaskAttachmentsAddURLMutatesSynchronously(t *testing.T) {
	project, err := models.NewProject("project-1", "Attachments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil)
	model.addMode = true
	model.textInput.SetValue("https://example.com/design.pdf")

	cmd := model.addAttachment()
	if cmd == nil {
		t.Fatal("expected non-nil command from addAttachment")
	}

	// Verify state was mutated synchronously before cmd() runs
	if model.addMode {
		t.Fatal("expected addMode to be false after addAttachment()")
	}
	if len(model.task.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(model.task.Attachments))
	}
	if !model.task.Attachments[0].IsURL {
		t.Fatal("expected attachment to be a URL")
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

	task, err := models.NewTask("task-1", 1, models.TaskTypeTask, "Test task")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	project.Tasks = []*models.Task{task}

	model := NewTaskAttachmentsModel(task, project, nil)
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

package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestCaptureCreatesInboxTask verifies that a plain quick-capture input (no
// section sigil) produces a result that routes to the Inbox section. This
// mirrors the first step of the primary user flow: capture → Inbox.
func TestCaptureCreatesInboxTask(t *testing.T) {
	project := testutil.NewTestProject("smoke")
	project.EnsureDefaultSections()

	result, err := parseQuickCapture("Buy milk", project, nil, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if result.title != "Buy milk" {
		t.Errorf("title: got %q, want %q", result.title, "Buy milk")
	}
	if result.taskType != models.TaskTypeTask {
		t.Errorf("taskType: got %v, want %v", result.taskType, models.TaskTypeTask)
	}
	// Empty section in result means createTask routes the task to SectionInbox.
	if result.section != "" {
		t.Errorf("section: got %q, want %q (empty = Inbox)", result.section, "")
	}
}

// TestMoveToTodayUpdatesStatus verifies the inbox→today flow: moving a task
// from the Inbox section to the Next section (the "today" queue in GTD terms)
// updates its section without touching its status.
func TestMoveToTodayUpdatesStatus(t *testing.T) {
	task, err := models.NewTask("t1", 1, models.TaskTypeTask, "Finish report")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.SetSection(models.SectionInbox)
	if !task.InSection(models.SectionInbox) {
		t.Fatal("precondition: task should be in Inbox")
	}

	task.SetSection(models.SectionNext)

	if !task.InSection(models.SectionNext) {
		t.Errorf("section: got %q, want %q", task.Section, models.SectionNext)
	}
	if task.Status != models.StatusTodo {
		t.Errorf("status: got %q, want %q (moving section must not change status)", task.Status, models.StatusTodo)
	}
}

// TestCompleteTodayTaskUpdatesProgress verifies the today→complete flow:
// completing a task that lives in the Next (today) section sets it to Done,
// moves it to the Completed section, and records the completion time.
func TestCompleteTodayTaskUpdatesProgress(t *testing.T) {
	task, err := models.NewTask("t2", 2, models.TaskTypeTask, "Ship feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.SetSection(models.SectionNext)
	task.Status = models.StatusTodo

	task.Complete()

	if task.Status != models.StatusDone {
		t.Errorf("status: got %q, want %q", task.Status, models.StatusDone)
	}
	if task.Section != models.SectionCompleted {
		t.Errorf("section: got %q, want %q", task.Section, models.SectionCompleted)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt: expected non-nil after Complete()")
	}
}

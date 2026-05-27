package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestHandleArchiveCompletedTasksUsesLiveCount verifies that
// handleArchiveCompletedTasks computes the completed-task count inside the
// returned tea.Cmd closure (when it actually runs), not at the moment the
// command is created. Otherwise stale data captured at command-creation
// time can cause the closure to either skip a real archive or attempt to
// archive an already-empty project.
func TestHandleArchiveCompletedTasksUsesLiveCount(t *testing.T) {
	app, project := newTestAppModelWithProject()
	mock := NewMockPersister()
	app.persister = mock

	// Sanity: the test project starts with at least one completed task.
	if project.CountCompletedTasks() == 0 {
		t.Fatal("test project should start with completed tasks")
	}

	_, cmd := app.handleArchiveCompletedTasks()
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Between command creation and execution, flip every completed task
	// back to todo. With the bug present (count captured outside the
	// closure), the cmd would still proceed to backup + archive based on
	// the stale non-zero count.
	for _, task := range project.Tasks {
		if task.Status == models.StatusDone {
			task.Status = models.StatusTodo
			task.CompletedAt = nil
		}
	}

	msg := cmd()

	archivedMsg, ok := msg.(TasksArchivedMsg)
	if !ok {
		t.Fatalf("expected TasksArchivedMsg, got %T", msg)
	}
	if archivedMsg.Count != 0 {
		t.Errorf("expected Count=0 when no tasks are completed at execution time, got %d", archivedMsg.Count)
	}

	// With the fix, the early-return for completedCount==0 fires inside
	// the closure, so no backup or archive call should occur.
	if len(mock.BackupCalls) != 0 {
		t.Errorf("expected no Backup calls when live count is zero, got %d", len(mock.BackupCalls))
	}
}

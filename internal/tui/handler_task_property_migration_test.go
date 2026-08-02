package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestHandleTaskPropertyChangedNeedsSaveCommitsAsync verifies the command-palette
// in-place property-change path (TaskPropertyChangedMsg with NeedsSave) now commits
// the already-mutated current project through the TaskItemCoordinator / application
// layer instead of the retired SaveProjectAsyncWithOptions wrapper: the mutation is
// committed on disk via the async BatchTransactionResultMsg and the workspace ledger
// restores the prior state. A non-zero baseline avoids the omitted-empty-field undo
// gap.
func TestHandleTaskPropertyChangedNeedsSaveCommitsAsync(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "priority task")
	task.Priority = models.PriorityLow
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	app.currentProject = env.projects[0]

	// Simulate a palette handler having applied the property change in memory.
	env.projects[0].MutateTask(task.ID, func(tk *models.Task) { tk.Priority = 2 })

	handled, cmd := app.handleTaskStatusPropertyMessages(TaskPropertyChangedMsg{
		Task:        task,
		NeedsSave:   true,
		UndoAction:  "Set Priority",
		UndoContext: "#1 priority task",
	})
	if !handled {
		t.Fatal("TaskPropertyChangedMsg should be handled")
	}

	runBatchResult(t, cmd)

	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Priority != 2 {
		t.Fatalf("committed priority = %v, want 2", got)
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(task.ID); got == nil || got.Priority != models.PriorityLow {
		t.Fatalf("undo priority = %v, want %d", got, models.PriorityLow)
	}
}

// TestHandleTaskPropertyChangedNoSaveSkipsCommit verifies a property change that
// does not request a save (NeedsSave=false) does not commit anything: the handler
// still reports handled but returns no batch-transaction command, so no async
// write is issued.
func TestHandleTaskPropertyChangedNoSaveSkipsCommit(t *testing.T) {
	task := newBulkPickerTestTask(t, 1, "no save task")
	env := newAppBulkPickerEnv(t, []*models.Task{task})
	app := env.appModel()
	app.currentProject = env.projects[0]

	handled, cmd := app.handleTaskStatusPropertyMessages(TaskPropertyChangedMsg{Task: task})
	if !handled {
		t.Fatal("TaskPropertyChangedMsg should be handled")
	}
	if cmd != nil {
		if _, ok := cmd().(BatchTransactionResultMsg); ok {
			t.Fatal("NeedsSave=false should not issue a batch-transaction commit")
		}
	}
}

package tui

import (
	"reflect"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// TestHandleBulkTagSelectorDoneAppliesAcrossProjects verifies the shared tag
// palette now commits one asynchronous cross-project transaction through the
// BulkCoordinator: the batch result carries the selection to clear and the
// per-view summary, both projects' tags are committed on disk, the shell clears
// the cached All Tasks multi-select on commit, and the workspace ledger undoes
// the whole batch.
func TestHandleBulkTagSelectorDoneAppliesAcrossProjects(t *testing.T) {
	taskA := newBulkPickerTestTask(t, 1, "alpha task", "old")
	taskB := newBulkPickerTestTask(t, 1, "beta task", "other")
	env := newAppBulkPickerEnv(t, []*models.Task{taskA}, []*models.Task{taskB})
	app := env.appModel()

	app.allTasksView = NewAllTasksViewModel(app.runtime, app.configManager)
	app.allTasksView.multiSelectMode = true
	app.allTasksView.selectedTasks[taskA.ID] = true
	app.allTasksView.selectedTasks[taskB.ID] = true
	app.views.Register(ViewAllTasks, app.allTasksView)

	app.pushView(ViewBulkTagSelector)
	handled, cmd := app.handleTagMessages(BulkTagSelectorDoneMsg{
		TaskRefs:     append(env.refs(0, taskA), env.refs(1, taskB)...),
		SelectedTags: []string{"shared", "next"},
	})
	if !handled {
		t.Fatal("BulkTagSelectorDoneMsg should be handled")
	}

	msg := runBatchResult(t, cmd)
	if msg.Summary != "Tagged 2 tasks" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Tagged 2 tasks")
	}
	if len(msg.ClearSelectionTaskIDs) != 2 {
		t.Errorf("clear selection ids = %v, want 2", msg.ClearSelectionTaskIDs)
	}
	if len(msg.TouchedProjectIDs) != 2 {
		t.Errorf("touched projects = %v, want 2 (cross-project)", msg.TouchedProjectIDs)
	}

	if got := env.reload(t, 0).GetTaskByID(taskA.ID).Tags; !reflect.DeepEqual(got, []string{"shared", "next"}) {
		t.Fatalf("projectA task tags = %v, want [shared next]", got)
	}
	if got := env.reload(t, 1).GetTaskByID(taskB.ID).Tags; !reflect.DeepEqual(got, []string{"shared", "next"}) {
		t.Fatalf("projectB task tags = %v, want [shared next]", got)
	}

	app.handleBatchTransactionResult(msg)
	if app.allTasksView.multiSelectMode || app.allTasksView.SelectedCount() != 0 {
		t.Fatal("committed batch should clear cached All Tasks multi-select")
	}

	env.undo(t)
	if got := env.reload(t, 0).GetTaskByID(taskA.ID).Tags; !reflect.DeepEqual(got, []string{"old"}) {
		t.Fatalf("undo projectA tags = %v, want [old]", got)
	}
	if got := env.reload(t, 1).GetTaskByID(taskB.ID).Tags; !reflect.DeepEqual(got, []string{"other"}) {
		t.Fatalf("undo projectB tags = %v, want [other]", got)
	}
}

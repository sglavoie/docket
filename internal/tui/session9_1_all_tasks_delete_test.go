package tui

import (
	"slices"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// Session 9.1 repair (canonical-all-tasks-delete, FR-009): the legacy All Tasks
// fallback path (AllTasksViewModel) must stop overriding the shared uppercase-X
// delete. It now supplies Origin ViewAllTasks and relies on the shared
// selection-or-cursor target resolver, so uppercase X emits the shell-owned
// TaskDeleteRequestedMsg — which prompts before any mutation and reports the exact
// target count — instead of confirming the cursor task and then deleting the live
// multi-selection. These tests drive the built keymap's "X" action and the real
// shell confirmation over the transactional stack (deleteRequestFromCmd /
// requestTaskIDs live in session3_uppercase_delete_test.go; newTaskDeleteEnv lives
// in session2_task_delete_transaction_test.go).

// newAllTasksDeleteModel registers an AllTasksViewModel over the shared delete env
// with the given tasks selected (or none, for a cursor-only test), so the fallback
// surface can exercise the canonical shell path end to end.
func newAllTasksDeleteModel(t *testing.T, e *taskDeleteEnv, selected ...TaskProjectRef) (*AppModel, *AllTasksViewModel) {
	t.Helper()
	m := e.appModel()
	atv := NewAllTasksViewModel(m.runtime, e.mgr)
	atv.bulk = m.secondaryFeatures.Bulk

	tasks := make([]CrossProjectTask, 0, len(selected))
	for _, ref := range selected {
		tasks = append(tasks, CrossProjectTask{Task: ref.Task, Project: ref.Project})
	}
	atv.groups = []AllTasksGroup{{Name: "All", Tasks: tasks}}
	if len(selected) > 0 {
		atv.multiSelectMode = true
		for _, ref := range selected {
			atv.selectedTasks[ref.Task.ID] = true
		}
	}

	m.allTasksView = atv
	m.views.Register(ViewAllTasks, atv)
	return m, atv
}

// TestAllTasksUppercaseXDeleteEmitsCanonicalRequest proves the fallback surface,
// with no active selection, emits the canonical request for the cursor task and
// stamps Origin ViewAllTasks — no view-local confirm modal, no pre-confirmation
// mutation.
func TestAllTasksUppercaseXDeleteEmitsCanonicalRequest(t *testing.T) {
	e := newTaskDeleteEnv(t)
	_, atv := newAllTasksDeleteModel(t, e)
	atv.groups = []AllTasksGroup{{Name: "All", Tasks: []CrossProjectTask{
		{Task: e.sibling, Project: e.alpha},
	}}}
	atv.cursor = 1 // 0 = group header, 1 = the sibling row

	action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if got := requestTaskIDs(msg); !slices.Equal(got, []string{e.sibling.ID}) {
		t.Fatalf("delete targets = %v, want cursor [%s]", got, e.sibling.ID)
	}
	if msg.Refs[0].Project.ID != e.alpha.ID {
		t.Errorf("ref project = %q, want %q", msg.Refs[0].Project.ID, e.alpha.ID)
	}
	if msg.Origin != ViewAllTasks {
		t.Errorf("origin = %v, want ViewAllTasks", msg.Origin)
	}
}

// TestAllTasksUppercaseXDeleteTargetsSelection proves the fix for the FR-009
// prompt/target mismatch: with a cross-project multi-selection, uppercase X
// targets the full selection (not the cursor task), and the shell prompt reports
// the true multi-target count instead of a single cursor short ID.
func TestAllTasksUppercaseXDeleteTargetsSelection(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m, atv := newAllTasksDeleteModel(t, e,
		TaskProjectRef{Task: e.sibling, Project: e.alpha},
		TaskProjectRef{Task: e.lone, Project: e.beta},
	)
	atv.cursor = 1 // cursor rests on the sibling — the old override confirmed only it

	action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)

	want := []string{e.sibling.ID, e.lone.ID}
	slices.Sort(want)
	if got := requestTaskIDs(msg); !slices.Equal(got, want) {
		t.Fatalf("delete targets = %v, want the full selection %v", got, want)
	}
	if msg.Origin != ViewAllTasks {
		t.Errorf("origin = %v, want ViewAllTasks", msg.Origin)
	}

	m.handleTaskDeleteRequested(msg)
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Errorf("prompt = %q, want the multi-target count %q", got, "Delete 2 tasks?")
	}
}

// TestAllTasksCanonicalDeleteCancelPreservesDataAndSelection proves the fallback
// path never mutates before confirmation: cancelling the shell confirm (n or Esc)
// leaves every selected task on disk and the multi-selection intact for a retry.
func TestAllTasksCanonicalDeleteCancelPreservesDataAndSelection(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		e := newTaskDeleteEnv(t)
		m, atv := newAllTasksDeleteModel(t, e,
			TaskProjectRef{Task: e.sibling, Project: e.alpha},
			TaskProjectRef{Task: e.lone, Project: e.beta},
		)

		action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
		_, cmd := action()
		m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))

		handled, out := m.handleTaskDeleteConfirmKey(key)
		if !handled {
			t.Fatalf("%q should be handled by the modal confirm", key)
		}
		if out != nil {
			t.Errorf("%q cancel should not dispatch a delete command", key)
		}
		if m.taskDeleteConfirm.IsActive() || len(m.pendingTaskDelete.roots) != 0 {
			t.Errorf("%q should clear the pending deletion", key)
		}
		if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) == nil || e.reload(t, e.beta).GetTaskByID(e.lone.ID) == nil {
			t.Errorf("%q cancel must preserve tasks on disk", key)
		}
		if !atv.multiSelectMode || atv.SelectedCount() != 2 {
			t.Errorf("%q cancel must preserve the selection", key)
		}
	}
}

// TestAllTasksCanonicalDeleteFailurePreservesSelection proves a rejected delete
// (a root that is not on disk) surfaces a TransactionErrorMsg, leaves the real
// tasks untouched, and keeps the selection so the user can retry.
func TestAllTasksCanonicalDeleteFailurePreservesSelection(t *testing.T) {
	e := newTaskDeleteEnv(t)
	ghost := seedTask(t, 99, "Ghost", models.StateReady)
	m, atv := newAllTasksDeleteModel(t, e, TaskProjectRef{Task: ghost, Project: e.alpha})

	action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
	_, reqCmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, reqCmd))

	handled, cmd := m.handleTaskDeleteConfirmKey("y")
	if !handled || cmd == nil {
		t.Fatal("confirm should dispatch a command even for a doomed delete")
	}
	if _, ok := cmd().(TransactionErrorMsg); !ok {
		t.Fatalf("failed delete should yield TransactionErrorMsg, got %T", cmd())
	}
	if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) == nil || e.reload(t, e.alpha).GetTaskByID(e.parent.ID) == nil {
		t.Error("a failed delete must leave existing tasks on disk")
	}
	if !atv.multiSelectMode || atv.SelectedCount() != 1 {
		t.Error("a failed delete must preserve the selection for a retry")
	}
}

// TestAllTasksCanonicalDeleteCommitClearsSelectionAndUndoes proves a committed
// delete removes the selected tasks, clears the multiselect, and remains undoable
// through the workspace ledger.
func TestAllTasksCanonicalDeleteCommitClearsSelectionAndUndoes(t *testing.T) {
	e := newTaskDeleteEnv(t)
	m, atv := newAllTasksDeleteModel(t, e,
		TaskProjectRef{Task: e.sibling, Project: e.alpha},
		TaskProjectRef{Task: e.lone, Project: e.beta},
	)

	action := findSharedTaskKeymapAction(t, atv.buildKeyMap(), "X")
	_, reqCmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, reqCmd))

	_, cmd := m.handleTaskDeleteConfirmKey("y")
	msg := runBatchResult(t, cmd)
	if len(msg.RemovedTaskIDs) != 2 {
		t.Fatalf("removed ids = %v, want sibling+lone", msg.RemovedTaskIDs)
	}
	m.handleBatchTransactionResult(msg)

	if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) != nil || e.reload(t, e.beta).GetTaskByID(e.lone.ID) != nil {
		t.Error("committed delete should remove the selected tasks from disk")
	}
	if atv.multiSelectMode || atv.SelectedCount() != 0 {
		t.Error("committed delete should clear the multiselect")
	}

	// The delete is undoable through the workspace ledger.
	if _, err := e.ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger undo: %v", err)
	}
	if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) == nil || e.reload(t, e.beta).GetTaskByID(e.lone.ID) == nil {
		t.Error("undo should restore the deleted tasks")
	}
}

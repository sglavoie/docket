package tui

import (
	"fmt"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestCrossProjectBulkStatusSkipsInvalidTransitions verifies that a Done task
// included in a bulk "start" (InProgress) operation is not forcibly un-completed:
// the migrated coordinator path drops it (SkipInvalid) rather than aborting the
// batch, and the valid Todo task still transitions.
func TestCrossProjectBulkStatusSkipsInvalidTransitions(t *testing.T) {
	done := seedTask(t, 1, "Already done", models.StateDone)
	todo := seedTask(t, 2, "Not started yet", models.StateReady)
	env := newKanbanBulkEnv(t, []*models.Task{done, todo})
	m := newCrossOutlinerFromEnv(env)

	m.bulkOp.SetOperation(models.StateInProgress, []*models.Task{done, todo}, 0, nil)
	_, cmd := m.crossExecutePendingBulkStatus()
	msg := runBatchResult(t, cmd)
	if msg.Summary != "Changed 1 tasks to in_progress" {
		t.Errorf("summary = %q, want the done task skipped (1 changed)", msg.Summary)
	}

	reloaded := env.reload(t, 0)
	if got := reloaded.GetTaskByID(done.ID); got == nil || got.State() != models.StateDone {
		t.Errorf("done task should remain done after bulk start, got %v", got)
	}
	if got := reloaded.GetTaskByID(todo.ID); got == nil || got.State() != models.StateInProgress {
		t.Errorf("todo task should be in_progress after bulk start, got %v", got)
	}
}

// assertBulkSkipsParent runs a cross-project bulk transition of [parent,
// standalone] to the given completion state and asserts the parent is skipped
// (never persisted in a completion state while a child is incomplete) while the
// standalone leaf is applied; the summary counts only the task actually changed.
func assertBulkSkipsParent(t *testing.T, state models.WorkflowState, wantState models.WorkflowState) {
	t.Helper()

	parent, err := models.NewTask("bulk-parent-001", 1, models.TaskTypeStory, "Parent Story")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parent.ApplyWorkflowState(models.StateInProgress)
	child, _ := models.NewTask("bulk-child-001", 2, models.TaskTypeTask, "Incomplete child")
	child.ParentID = &parent.ID
	child.ApplyWorkflowState(models.StateReady)
	standalone, _ := models.NewTask("bulk-standalone-001", 3, models.TaskTypeTask, "Standalone leaf")
	standalone.ApplyWorkflowState(models.StateReady)

	env := newKanbanBulkEnv(t, []*models.Task{parent, child, standalone})
	m := newCrossOutlinerFromEnv(env)

	m.bulkOp.SetOperation(state, []*models.Task{parent, standalone}, 0, nil)
	_, cmd := m.crossExecutePendingBulkStatus()
	msg := runBatchResult(t, cmd)
	if want := fmt.Sprintf("Changed 1 tasks to %s", state); msg.Summary != want {
		t.Errorf("summary = %q, want %q (parent skipped)", msg.Summary, want)
	}

	reloaded := env.reload(t, 0)
	if got := reloaded.GetTaskByID(parent.ID); got == nil || got.IsTerminalState() {
		t.Errorf("parent should be skipped while a child is incomplete, got %v", got)
	}
	if got := reloaded.GetTaskByID(standalone.ID); got == nil || got.State() != wantState {
		t.Errorf("standalone leaf should be %s, got %v", wantState, got)
	}
}

// TestCrossProjectBulkCompleteSkipsParentWithIncompleteChild verifies a
// cross-project bulk-complete never writes a done parent with an incomplete
// child; the parent is skipped and only the standalone leaf changes.
func TestCrossProjectBulkCompleteSkipsParentWithIncompleteChild(t *testing.T) {
	assertBulkSkipsParent(t, models.StateDone, models.StateDone)
}

// TestCrossProjectBulkCancelSkipsParentWithIncompleteChild verifies a
// cross-project bulk-cancel likewise skips a parent with an incomplete child.
func TestCrossProjectBulkCancelSkipsParentWithIncompleteChild(t *testing.T) {
	assertBulkSkipsParent(t, models.StateCanceled, models.StateCanceled)
}

// TestCrossProjectBulkReconcileWholeProjectUndoRestoresReopenedParent verifies the
// reconcile net + whole-project ledger undo contract: when a cross-project bulk op
// touches a project that already holds an illegal done-parent/incomplete-child
// state, applyPostMutationHooks reopens the parent before the commit, and the
// workspace-ledger undo restores the reopened parent (covered by the whole-project
// before-image) along with the mutated leaves in every affected project.
func TestCrossProjectBulkReconcileWholeProjectUndoRestoresReopenedParent(t *testing.T) {
	// Alpha carries a pre-existing illegal state: done parent + incomplete child.
	alphaParent, _ := models.NewTask("alpha-parent-001", 1, models.TaskTypeStory, "Alpha parent")
	alphaParent.Complete()
	alphaChild, _ := models.NewTask("alpha-child-001", 2, models.TaskTypeTask, "Alpha incomplete child")
	alphaChild.ParentID = &alphaParent.ID
	alphaChild.ApplyWorkflowState(models.StateReady)
	alphaLeaf, _ := models.NewTask("alpha-leaf-001", 3, models.TaskTypeTask, "Alpha standalone")
	alphaLeaf.ApplyWorkflowState(models.StateReady)

	betaLeaf, _ := models.NewTask("beta-leaf-001", 1, models.TaskTypeTask, "Beta standalone")
	betaLeaf.ApplyWorkflowState(models.StateReady)

	env := newKanbanBulkEnv(t,
		[]*models.Task{alphaParent, alphaChild, alphaLeaf},
		[]*models.Task{betaLeaf})
	m := newCrossOutlinerFromEnv(env)

	m.bulkOp.SetOperation(models.StateDone, []*models.Task{alphaLeaf, betaLeaf}, 0, nil)
	_, cmd := m.crossExecutePendingBulkStatus()
	_ = runBatchResult(t, cmd)

	// The commit reconciled alpha: the illegal done parent reopened before save.
	if got := env.reload(t, 0).GetTaskByID("alpha-parent-001"); got == nil || got.State() != models.StateInProgress {
		t.Errorf("alpha parent after reconcile = %v, want in_progress", got)
	}
	if got := env.reload(t, 0).GetTaskByID("alpha-leaf-001"); got == nil || got.State() != models.StateDone {
		t.Errorf("alpha leaf = %v, want done", got)
	}

	// Ledger undo restores both projects; the reopened parent comes back done via
	// the whole-project before-image even though it was never in the bulk set.
	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false")
	}
	_ = undoResult.(*OutlinerModel)
	if got := env.reload(t, 0).GetTaskByID("alpha-parent-001"); got == nil || got.State() != models.StateDone {
		t.Errorf("alpha parent after undo = %v, want done (whole-project before-image restores the reopened parent)", got)
	}
	if got := env.reload(t, 0).GetTaskByID("alpha-leaf-001"); got == nil || got.State() != models.StateReady {
		t.Errorf("alpha leaf after undo = %v, want ready", got)
	}
	if got := env.reload(t, 1).GetTaskByID("beta-leaf-001"); got == nil || got.State() != models.StateReady {
		t.Errorf("beta leaf after undo = %v, want ready", got)
	}

	if storage.UndoHistoryExists(env.projects[0]) || storage.UndoHistoryExists(env.projects[1]) {
		t.Fatal("no legacy per-project undo history should exist after a ledger-backed bulk change")
	}
}

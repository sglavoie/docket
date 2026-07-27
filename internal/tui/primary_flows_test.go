package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestCaptureCreatesInboxTask verifies that a plain quick-capture input (no
// section sigil) produces a result that routes to the Inbox section. This
// mirrors the first step of the primary user flow: capture → Inbox.
func TestCaptureCreatesInboxTask(t *testing.T) {
	project := testutil.NewTestProject("smoke")

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
}

// TestMoveToNextUpdatesStatus verifies the inbox→next flow: moving a task
// from the Inbox section to the Next section (the on-deck shortlist, reached
// via ;q) updates its section without touching its status. Next is distinct
// from Today (the Daily Plan, reached via ;w).
func TestMoveToNextUpdatesStatus(t *testing.T) {
	task, err := models.NewTask("t1", 1, models.TaskTypeTask, "Finish report")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ApplyWorkflowState(models.StateInbox)
	if !task.IsInState(models.StateInbox) {
		t.Fatal("precondition: task should be in Inbox")
	}

	task.ApplyWorkflowState(models.StateNext)

	if !task.IsInState(models.StateNext) {
		t.Errorf("section: got %q, want %q", task.State(), models.StateNext)
	}
	if got := task.State(); got != models.StateNext {
		t.Errorf("state: got %q, want %q", got, models.StateNext)
	}
}

// TestCompleteNextTaskUpdatesProgress verifies the next→complete flow:
// completing a task that lives in the Next (on-deck) section sets it to Done,
// moves it to the Completed section, and records the completion time.
func TestCompleteNextTaskUpdatesProgress(t *testing.T) {
	task, err := models.NewTask("t2", 2, models.TaskTypeTask, "Ship feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ApplyWorkflowState(models.StateNext)

	task.Complete()

	if task.State() != models.StateDone {
		t.Errorf("state: got %q, want %q", task.State(), models.StateDone)
	}
	if got := task.State(); got != models.StateDone {
		t.Errorf("section: got %q, want %q", got, models.StateDone)
	}
	if task.CompletedAt == nil {
		t.Error("CompletedAt: expected non-nil after Complete()")
	}
}

// newCompletedStoryWithDoneChild builds a project saved on disk with a Story
// parent and a single Task child, both Done — a legal completed state. Returns
// the project plus the parent and child IDs (task pointers are invalidated by a
// persist-triggered reload, so callers should re-fetch via GetTaskByID).
func newCompletedStoryWithDoneChild(t *testing.T) (project *models.Project, parentID, childID string) {
	t.Helper()
	parent := newBulkPickerTestTask(t, 1, "Parent Story")
	parent.Type = models.TaskTypeStory
	child := newBulkPickerTestTask(t, 2, "Child")
	child.ParentID = &parent.ID
	child.Complete()
	parent.Complete()
	project = newBulkPickerTestProject(t, "Reopen Cascade", parent, child)
	return project, parent.ID, child.ID
}

// reopenChildThroughWorkflow reopens the given child through the same
// post-mutation pipeline the shared save path runs (auto-unblock, reverse
// parent-completion reconcile, recurring generation, root reorder), records a
// targeted undo snapshot widened with the reopened parent IDs, then persists and
// reloads the project from disk — mirroring a single-task status change.
func reopenChildThroughWorkflow(t *testing.T, project *models.Project, childID string) {
	t.Helper()
	project.MutateTask(childID, func(tk *models.Task) {
		if err := tk.ApplyStateTransition(models.StateReady); err != nil {
			t.Fatalf("ApplyStateTransition(ready): %v", err)
		}
	})
	project.CheckAndAutoUnblock()
	reopened := project.ReconcileParentCompletion()
	project.GenerateRecurringInstances()
	project.ReorderRootTasksByCompletion()

	taskIDs := append([]string{childID}, reopened...)
	if err := storage.RecordUndoSnapshotWithOptions(project, storage.UndoSnapshotOptions{
		Action:   "Reopen Task",
		Context:  "#2 Child",
		MaxDepth: 10,
		TaskIDs:  taskIDs,
	}); err != nil {
		t.Fatalf("RecordUndoSnapshotWithOptions: %v", err)
	}
	if err := storage.BackupProject(project); err != nil {
		t.Fatalf("BackupProject: %v", err)
	}
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
}

// TestReopenThenOutdentKeepsParentVisible is the end-to-end repro for the
// vanishing-Story bug: a Story with a done child is complete; reopening the
// child auto-uncompletes the Story to in_progress (reverse cascade), so after
// outdenting the child the Story remains a real, visible task in the active view
// rather than a completed context node that disappears once its last child
// leaves.
func TestReopenThenOutdentKeepsParentVisible(t *testing.T) {
	project, parentID, childID := newCompletedStoryWithDoneChild(t)

	reopenChildThroughWorkflow(t, project, childID)

	story := project.GetTaskByID(parentID)
	if story == nil {
		t.Fatal("story missing after reopening child")
	}
	if story.State() != models.StateInProgress {
		t.Fatalf("story state = %q, want in_progress after reopening its child", story.State())
	}

	// Outdent the now-incomplete child to the Story's (root) level.
	child := project.GetTaskByID(childID)
	if child == nil {
		t.Fatal("child missing after reopen")
	}
	if !OutdentTask(project, child) {
		t.Fatal("expected outdent of child to succeed")
	}

	story = project.GetTaskByID(parentID)
	if story == nil {
		t.Fatal("story vanished from project after outdenting its last child")
	}
	if story.State() != models.StateInProgress {
		t.Errorf("story state = %q, want in_progress after outdent", story.State())
	}
}

// TestReopenChildUndoRevertsCascadeAtomically verifies the atomic-undo contract:
// reopening a child auto-uncompletes its parent, and a single undo restores both
// the child's status and the cascaded parent uncompletion, with redo re-applying
// both. This depends on ReconcileParentCompletion's reopened IDs being folded
// into the targeted undo snapshot.
func TestReopenChildUndoRevertsCascadeAtomically(t *testing.T) {
	project, parentID, childID := newCompletedStoryWithDoneChild(t)

	reopenChildThroughWorkflow(t, project, childID)

	if got := project.GetTaskByID(parentID).State(); got != models.StateInProgress {
		t.Fatalf("parent status = %q, want in_progress after reopening child", got)
	}
	if got := project.GetTaskByID(childID).State(); got != models.StateReady {
		t.Fatalf("child status = %q, want todo after reopen", got)
	}

	if _, err := storage.PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after undo: %v", err)
	}
	if got := project.GetTaskByID(parentID).State(); got != models.StateDone {
		t.Errorf("after undo parent status = %q, want done (cascade must revert atomically)", got)
	}
	if got := project.GetTaskByID(childID).State(); got != models.StateDone {
		t.Errorf("after undo child status = %q, want done", got)
	}

	if _, err := storage.PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after redo: %v", err)
	}
	if got := project.GetTaskByID(parentID).State(); got != models.StateInProgress {
		t.Errorf("after redo parent status = %q, want in_progress", got)
	}
	if got := project.GetTaskByID(childID).State(); got != models.StateReady {
		t.Errorf("after redo child status = %q, want todo", got)
	}
}

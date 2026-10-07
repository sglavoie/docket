package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/capturetext"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/testutil"
)

// TestCaptureCreatesInboxTask verifies that a plain quick-capture input (no
// section sigil) produces a result that routes to the Inbox section. This
// mirrors the first step of the primary user flow: capture → Inbox.
func TestCaptureCreatesInboxTask(t *testing.T) {
	project := testutil.NewTestProject("smoke")

	result, err := capturetext.ParseQuickCapture("Buy milk", project, nil, false)
	if err != nil {
		t.Fatalf("parseQuickCapture: %v", err)
	}
	if result.Title != "Buy milk" {
		t.Errorf("Title: got %q, want %q", result.Title, "Buy milk")
	}
}

// TestMoveToNextUpdatesStatus verifies the inbox→next flow: moving a task
// from the Inbox section to the Next section (the on-deck shortlist, reached
// via ;q) updates its section without touching its status. Next is distinct
// from Today (the Daily Plan, reached via ;w).
func TestMoveToNextUpdatesStatus(t *testing.T) {
	task, err := models.NewGenericTask("t1", 1, "Finish report")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ApplyWorkflowState(models.StateInbox)
	if !task.IsInState(models.StateInbox) {
		t.Fatal("precondition: task should be in Inbox")
	}

	task.ApplyWorkflowState(models.StateBacklog)

	if !task.IsInState(models.StateBacklog) {
		t.Errorf("section: got %q, want %q", task.State(), models.StateBacklog)
	}
	if got := task.State(); got != models.StateBacklog {
		t.Errorf("state: got %q, want %q", got, models.StateBacklog)
	}
}

// TestCompleteNextTaskUpdatesProgress verifies the next→complete flow:
// completing a task that lives in the Next (on-deck) section sets it to Done,
// moves it to the Completed section, and records the completion time.
func TestCompleteNextTaskUpdatesProgress(t *testing.T) {
	task, err := models.NewGenericTask("t2", 2, "Ship feature")
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	task.ApplyWorkflowState(models.StateBacklog)

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
	child := newBulkPickerTestTask(t, 2, "Child")
	child.ParentID = &parent.ID
	child.Complete()
	parent.Complete()
	project = newBulkPickerTestProject(t, "Reopen Cascade", parent, child)
	return project, parent.ID, child.ID
}

// reopenChildThroughWorkflow reopens the given child through the same
// post-mutation pipeline the shared save path runs (reverse parent-completion
// reconcile and root reorder), then persists and reloads the
// project from disk.
func reopenChildThroughWorkflow(t *testing.T, project *models.Project, childID string) {
	t.Helper()
	project.MutateTask(childID, func(tk *models.Task) {
		if err := tk.ApplyStateTransition(models.StateBacklog); err != nil {
			t.Fatalf("ApplyStateTransition(ready): %v", err)
		}
	})
	project.ReconcileParentCompletion()
	project.ReorderRootTasksByCompletion()

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
	if story.State() != models.StateDoing {
		t.Fatalf("story state = %q, want in_progress after reopening its child", story.State())
	}

	// Outdent the now-incomplete child to the Story's (root) level.
	child := project.GetTaskByID(childID)
	if child == nil {
		t.Fatal("child missing after reopen")
	}
	if ok, reason := child.CanOutdent(); !ok {
		t.Fatalf("expected outdent of child to be allowed: %s", reason)
	}
	if err := project.OutdentTask(child); err != nil {
		t.Fatalf("outdent child: %v", err)
	}

	story = project.GetTaskByID(parentID)
	if story == nil {
		t.Fatal("story vanished from project after outdenting its last child")
	}
	if story.State() != models.StateDoing {
		t.Errorf("story state = %q, want in_progress after outdent", story.State())
	}
}

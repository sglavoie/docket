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

// newParentChildProject builds a project with a Story parent (in_progress) and a
// single Task child, wiring up the tree so HasIncompleteChildren works. The
// child status is caller-controlled. Returns the project and the parent task.
func newParentChildProject(t *testing.T, childStatus models.TaskStatus) (*models.Project, *models.Task) {
	t.Helper()
	project := testutil.NewTestProject("bulk")

	parent, err := models.NewTask("parent-1", 1, models.TaskTypeStory, "Parent Story")
	if err != nil {
		t.Fatalf("NewTask parent: %v", err)
	}
	parent.Status = models.StatusInProgress
	parent.Priority = 3

	child, err := models.NewTask("child-1", 2, models.TaskTypeTask, "Child")
	if err != nil {
		t.Fatalf("NewTask child: %v", err)
	}
	child.ParentID = &parent.ID
	child.Status = childStatus

	project.Tasks = []*models.Task{parent, child}
	project.BuildTaskTree()
	return project, project.GetTaskByID(parent.ID)
}

// TestBulkEditSkipsParentCompletionButAppliesOtherFields verifies the bulk-edit
// path: a status→done change is skipped for a parent with an incomplete child,
// but other pending fields (priority, tag) still apply on the same pass.
func TestBulkEditSkipsParentCompletionButAppliesOtherFields(t *testing.T) {
	project, parent := newParentChildProject(t, models.StatusTodo)
	m := NewProjectTreeModel(project, nil, nil)

	done := models.StatusDone
	newPriority := 1
	m.applyBulkChanges(PendingChanges{
		SetStatus:   &done,
		SetPriority: &newPriority,
		AddTags:     []string{"urgent"},
	}, []*models.Task{parent})

	if parent.Status != models.StatusInProgress {
		t.Errorf("status: got %q, want %q (completion must be skipped for parent with incomplete child)", parent.Status, models.StatusInProgress)
	}
	if parent.Priority != newPriority {
		t.Errorf("priority: got %d, want %d (other fields must still apply)", parent.Priority, newPriority)
	}
	if !parent.HasTag("urgent") {
		t.Error("tag: expected 'urgent' to be applied despite skipped status change")
	}
}

// TestBulkEditCompletesParentWhenChildrenDone is the positive control: with all
// children complete, the same status→done bulk edit is applied to the parent.
func TestBulkEditCompletesParentWhenChildrenDone(t *testing.T) {
	project, parent := newParentChildProject(t, models.StatusDone)
	m := NewProjectTreeModel(project, nil, nil)

	done := models.StatusDone
	m.applyBulkChanges(PendingChanges{SetStatus: &done}, []*models.Task{parent})

	if parent.Status != models.StatusDone {
		t.Errorf("status: got %q, want %q (parent should complete when all children done)", parent.Status, models.StatusDone)
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

// reopenChildThroughWorkflow reopens the given child through the full TUI save
// pipeline (persistWithWorkflow), mirroring a single-task status change with a
// targeted undo snapshot. It reloads the project from disk on return.
func reopenChildThroughWorkflow(t *testing.T, project *models.Project, childID string) {
	t.Helper()
	project.MutateTask(childID, func(tk *models.Task) { tk.ApplyTransition(models.StatusTodo) })
	if err := persistWithWorkflow(WorkflowSaveOptions{
		Persister:   NewProjectService(),
		Project:     project,
		UndoAction:  "Reopen Task",
		UndoContext: "#2 Child",
		UndoDepth:   10,
		UndoTaskIDs: []string{childID},
	}); err != nil {
		t.Fatalf("persistWithWorkflow: %v", err)
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
	if story.Status != models.StatusInProgress {
		t.Fatalf("story status = %q, want in_progress after reopening its child", story.Status)
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
	if story.Status != models.StatusInProgress {
		t.Errorf("story status = %q, want in_progress after outdent", story.Status)
	}

	// In the default active view a completed task is hidden; an in_progress
	// Story stays visible, which is the whole point of the fix.
	m := &ProjectTreeModel{project: project, viewMode: ViewModeActive}
	if !m.passesFilters(&TreeNode{Task: story, Visible: true}) {
		t.Error("story should remain visible in the active view after outdent")
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

	if got := project.GetTaskByID(parentID).Status; got != models.StatusInProgress {
		t.Fatalf("parent status = %q, want in_progress after reopening child", got)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusTodo {
		t.Fatalf("child status = %q, want todo after reopen", got)
	}

	if _, err := storage.PerformUndo(project); err != nil {
		t.Fatalf("PerformUndo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after undo: %v", err)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusDone {
		t.Errorf("after undo parent status = %q, want done (cascade must revert atomically)", got)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusDone {
		t.Errorf("after undo child status = %q, want done", got)
	}

	if _, err := storage.PerformRedo(project); err != nil {
		t.Fatalf("PerformRedo: %v", err)
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatalf("LoadProject after redo: %v", err)
	}
	if got := project.GetTaskByID(parentID).Status; got != models.StatusInProgress {
		t.Errorf("after redo parent status = %q, want in_progress", got)
	}
	if got := project.GetTaskByID(childID).Status; got != models.StatusTodo {
		t.Errorf("after redo child status = %q, want todo", got)
	}
}

package tui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// This file exercises the secondary-view feature coordinators
// (feature_coordinator_secondary.go) end to end. The task/artifact coordinators
// (attachment, bulk) run through the real transactional storage stack and undo
// ledger — the same mutation→reload→render→undo cycle the primary scenarios honor
// against the retained task/artifact coordinator seam.

// secondaryEnv extends scenarioEnv with the secondary coordinators bound to the
// same application service.
func newSecondaryEnv(t *testing.T, slug string, tasks ...*models.Task) (*scenarioEnv, SecondaryCoordinators) {
	t.Helper()
	env := newScenarioEnv(t, slug, tasks...)
	return env, newSecondaryCoordinators(env.svc)
}

// runBatchResult executes a coordinator command and asserts it produced a
// committed batch-transaction message, returning it.
func runBatchResult(t *testing.T, cmd tea.Cmd) BatchTransactionResultMsg {
	t.Helper()
	if cmd == nil {
		t.Fatal("coordinator returned a nil command")
	}
	switch msg := cmd().(type) {
	case BatchTransactionResultMsg:
		return msg
	case TransactionErrorMsg:
		t.Fatalf("expected a committed batch, got error: %v", msg.Err)
	default:
		t.Fatalf("expected BatchTransactionResultMsg, got %T", msg)
	}
	return BatchTransactionResultMsg{}
}

// TestAttachmentScenario covers the transactional attachment overlay: attaching a
// URL commits the metadata, the reloaded task shows it, and undo removes it.
func TestAttachmentScenario(t *testing.T) {
	task := seedTask(t, 1, "Attach here", models.StateBacklog)
	env, features := newSecondaryEnv(t, "attach", task)

	res := runResult(t, features.Attachment.AttachURL(taskIdentityFor(task, env.project), "https://example.com/spec"))
	if res.Revision == "" {
		t.Fatal("attachment result carried no committed revision")
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Attachments) != 1 {
		t.Fatalf("reloaded task attachments = %v, want 1", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Attachments) != 0 {
		t.Fatalf("undo did not remove the attachment: %v", got)
	}
}

// TestBulkTransitionScenario covers the transactional bulk overlay: transitioning
// two tasks commits one batch, the reloaded project shows both moved, the summary
// counts them, and undo restores their prior states.
func TestBulkTransitionScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateBacklog)
	b := seedTask(t, 2, "Second", models.StateBacklog)
	env, features := newSecondaryEnv(t, "bulk", a, b)

	msg := runBatchResult(t, features.Bulk.Transition([]string{a.ID, b.ID}, models.StateDoing))
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}
	if msg.Summary == "" {
		t.Fatal("batch result carried no summary")
	}

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateDoing {
			t.Fatalf("task %s state = %v, want in_progress", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.State() != models.StateBacklog {
			t.Fatalf("undo state for %s = %v, want ready", id, got)
		}
	}
}

// TestBulkDeleteScenario covers the transactional bulk delete overlay: deleting two
// subtrees commits one batch, the reloaded project loses both, and undo restores
// them.
func TestBulkDeleteScenario(t *testing.T) {
	a := seedTask(t, 1, "Doomed A", models.StateBacklog)
	b := seedTask(t, 2, "Doomed B", models.StateBacklog)
	env, features := newSecondaryEnv(t, "bulkdel", a, b)

	msg := runBatchResult(t, features.Bulk.Delete([]string{a.ID, b.ID}))
	if len(msg.TouchedProjectIDs) == 0 {
		t.Fatal("bulk delete touched no projects")
	}

	reloaded := env.reload(t)
	if reloaded.GetTaskByID(a.ID) != nil || reloaded.GetTaskByID(b.ID) != nil {
		t.Fatal("bulk delete left a task behind")
	}

	env.undo(t)
	reloaded = env.reload(t)
	if reloaded.GetTaskByID(a.ID) == nil || reloaded.GetTaskByID(b.ID) == nil {
		t.Fatal("undo did not restore both deleted tasks")
	}
}

// TestBulkSetPriorityScenario covers the transactional bulk priority overlay:
// setting the priority on two tasks commits one batch, the reloaded project shows
// both changed, and undo restores their prior priorities.
func TestBulkSetPriorityScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateBacklog)
	b := seedTask(t, 2, "Second", models.StateBacklog)
	a.Priority = 3
	b.Priority = 3
	env, features := newSecondaryEnv(t, "bulk-priority", a, b)

	msg := runBatchResult(t, features.Bulk.SetPriority([]string{a.ID, b.ID}, 1))
	if len(msg.TouchedProjectIDs) != 1 || msg.TouchedProjectIDs[0] != env.project.ID {
		t.Fatalf("touched projects = %v, want [%s]", msg.TouchedProjectIDs, env.project.ID)
	}

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 1 {
			t.Fatalf("task %s priority = %v, want 1", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.Priority != 3 {
			t.Fatalf("undo priority for %s = %v, want 3", id, got)
		}
	}
}

// TestBulkDueDateScenario covers the transactional bulk due-date overlay: setting
// a due date on two tasks commits one batch, clearing it commits another, and undo
// restores the set date.
func TestBulkDueDateScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateBacklog)
	b := seedTask(t, 2, "Second", models.StateBacklog)
	env, features := newSecondaryEnv(t, "bulk-due", a, b)

	due := time.Date(2026, 8, 1, 0, 0, 0, 0, time.Local)
	runBatchResult(t, features.Bulk.SetDueDate([]string{a.ID, b.ID}, due))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-08-01" {
			t.Fatalf("task %s due date = %v, want 2026-08-01", id, got)
		}
	}

	runBatchResult(t, features.Bulk.ClearDueDate([]string{a.ID, b.ID}))
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate != nil {
			t.Fatalf("task %s due date not cleared: %v", id, got)
		}
	}

	// Undo the clear: the set date returns.
	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DueDate == nil || got.DueDate.Format("2006-01-02") != "2026-08-01" {
			t.Fatalf("undo did not restore due date for %s: %v", id, got)
		}
	}
}

// TestBulkSnoozeScenario covers the transactional bulk snooze overlay: snoozing
// two tasks commits one batch, clearing the snooze commits another, and undo
// restores the snooze.
func TestBulkSnoozeScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateBacklog)
	b := seedTask(t, 2, "Second", models.StateBacklog)
	env, features := newSecondaryEnv(t, "bulk-snooze", a, b)

	until := time.Date(2026, 8, 10, 0, 0, 0, 0, time.Local)
	runBatchResult(t, features.Bulk.Defer([]string{a.ID, b.ID}, &until))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DeferredUntil == nil || got.DeferredUntil.Format("2006-01-02") != "2026-08-10" {
			t.Fatalf("task %s snoozed = %v, want 2026-08-10", id, got)
		}
	}

	runBatchResult(t, features.Bulk.Defer([]string{a.ID, b.ID}, nil))
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DeferredUntil != nil {
			t.Fatalf("task %s snooze not cleared: %v", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		if got := reloaded.GetTaskByID(id); got == nil || got.DeferredUntil == nil || got.DeferredUntil.Format("2006-01-02") != "2026-08-10" {
			t.Fatalf("undo did not restore snooze for %s: %v", id, got)
		}
	}
}

// TestBulkSetTagsScenario covers the transactional bulk tags overlay: replacing
// the tag set on two tasks commits one batch, the reloaded project shows the new
// tags, and undo restores the originals.
func TestBulkSetTagsScenario(t *testing.T) {
	a := seedTask(t, 1, "First", models.StateBacklog)
	b := seedTask(t, 2, "Second", models.StateBacklog)
	a.AddTag("old")
	env, features := newSecondaryEnv(t, "bulk-tags", a, b)

	runBatchResult(t, features.Bulk.SetTags([]string{a.ID, b.ID}, []string{"backend", "urgent"}))

	reloaded := env.reload(t)
	for _, id := range []string{a.ID, b.ID} {
		got := reloaded.GetTaskByID(id)
		if got == nil || len(got.Tags) != 2 || got.Tags[0] != "backend" || got.Tags[1] != "urgent" {
			t.Fatalf("task %s tags = %v, want [backend urgent]", id, got)
		}
	}

	env.undo(t)
	reloaded = env.reload(t)
	if got := reloaded.GetTaskByID(a.ID); got == nil || len(got.Tags) != 1 || got.Tags[0] != "old" {
		t.Fatalf("undo did not restore task a tags: %v", reloaded.GetTaskByID(a.ID))
	}
	if got := reloaded.GetTaskByID(b.ID); got == nil || len(got.Tags) != 0 {
		t.Fatalf("undo did not restore task b tags: %v", reloaded.GetTaskByID(b.ID))
	}
}

// TestSubviewCommentScenario covers the transactional comments panel: adding a
// comment commits it, the reloaded task shows it, and undo removes it.
func TestSubviewCommentScenario(t *testing.T) {
	task := seedTask(t, 1, "Comment here", models.StateBacklog)
	env, features := newSecondaryEnv(t, "subview-comment", task)

	res := runResult(t, features.Subview.AddComment(taskIdentityFor(task, env.project), "found the root cause"))
	if res.Revision == "" {
		t.Fatal("comment result carried no committed revision")
	}

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Comments) != 1 || got.Comments[0].Text != "found the root cause" {
		t.Fatalf("reloaded task comments = %v, want the added comment", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Comments) != 0 {
		t.Fatalf("undo did not remove the comment: %v", got)
	}
}

// TestSubviewChecklistScenario covers the transactional checklist panel: adding
// and toggling an item commit separately, the reloaded task shows the toggled
// item, and undo reverts the toggle while keeping the item.
func TestSubviewChecklistScenario(t *testing.T) {
	task := seedTask(t, 1, "Check here", models.StateBacklog)
	env, features := newSecondaryEnv(t, "subview-checklist", task)

	runResult(t, features.Subview.AddChecklistItem(taskIdentityFor(task, env.project), "write the test"))
	runResult(t, features.Subview.ToggleChecklistItem(taskIdentityFor(task, env.project), 0))

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Checklist) != 1 || !got.Checklist[0].Done {
		t.Fatalf("reloaded checklist = %v, want one done item", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || len(got.Checklist) != 1 || got.Checklist[0].Done {
		t.Fatalf("undo did not revert the toggle: %v", got)
	}
}

// TestAttachmentReorderScenario covers the transactional attachment reorder: the
// swap commits, the reloaded task shows the new order, and undo restores it.
func TestAttachmentReorderScenario(t *testing.T) {
	task := seedTask(t, 1, "Reorder here", models.StateBacklog)
	env, features := newSecondaryEnv(t, "attach-reorder", task)

	runResult(t, features.Attachment.AttachURL(taskIdentityFor(task, env.project), "https://example.com/a"))
	runResult(t, features.Attachment.AttachURL(taskIdentityFor(task, env.project), "https://example.com/b"))
	runResult(t, features.Attachment.Reorder(taskIdentityFor(task, env.project), 0, 1))

	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Attachments[0].Filename != "https://example.com/b" {
		t.Fatalf("reloaded attachments = %v, want b first", got)
	}

	env.undo(t)
	if got := env.reload(t).GetTaskByID(task.ID); got == nil || got.Attachments[0].Filename != "https://example.com/a" {
		t.Fatalf("undo did not restore the order: %v", got)
	}
}

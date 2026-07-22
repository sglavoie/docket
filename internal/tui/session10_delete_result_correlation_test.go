package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// Session 10.1 repair (correlated-delete-postcommit, FR-011 + FR-012): canonical
// deletes commit asynchronously, but the Inbox post-commit slot and the Weekly
// Review deletion count were consumed by WHICHEVER batch result or transaction
// error arrived first. These tests correlate that post-commit bookkeeping with
// the exact delete via a per-request token (deleteRequestSeq, carried on the
// committing BatchTransactionResultMsg / TransactionErrorMsg): an unrelated
// success or error must leave an in-flight delete armed, and only the matching
// result may apply queue bookkeeping / accounting, exactly once.
//
// Shared helpers: inboxDeleteEnv/newInboxDeleteEnv live in
// session4_inbox_delete_test.go; taskDeleteEnv/newWeeklyReviewDeleteModel /
// runBatchResult / deleteRequestFromCmd / findSharedTaskKeymapAction /
// buildWeeklyReviewKeyMap live in the session2/9 delete tests.

// armLocalInboxDelete drives the local inbox processor through the canonical
// shell path up to (but not including) result delivery: it requests the delete
// of the task at currentIndex, confirms the shell dialog, and returns the
// committed matching batch. The pending Inbox bookkeeping is armed with the
// minted request token when this returns.
func armLocalInboxDelete(t *testing.T, e *inboxDeleteEnv) (matching BatchTransactionResultMsg, deletedID, nextID string) {
	t.Helper()
	m, proc := e.m, e.processor
	deletedID = e.tasks[proc.currentIndex].ID
	nextID = e.tasks[proc.currentIndex+1].ID

	reqCmd := proc.requestDelete()
	if reqCmd == nil {
		t.Fatal("requestDelete returned nil")
	}
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	m.handleTaskDeleteRequested(req)
	if _, cmd := m.handleTaskDeleteConfirmKey("y"); cmd != nil {
		matching = runBatchResult(t, cmd)
	} else {
		t.Fatal("confirm should dispatch the delete command")
	}
	pending, ok := singleArmedInboxDelete(m)
	if !ok || pending.taskID != deletedID || pending.token == 0 {
		t.Fatalf("pending Inbox delete not armed with a token, got %+v (armed=%v)", pending, ok)
	}
	if matching.DeleteRequestToken != pending.token {
		t.Fatalf("matching batch token %d != armed token %d", matching.DeleteRequestToken, pending.token)
	}
	return matching, deletedID, nextID
}

// assertInboxDeleteApplies feeds the matching batch and verifies the deferred
// bookkeeping runs exactly once: the task is processed, gone from disk, the
// cursor advances to the natural next task, and the pending slot clears.
func assertInboxDeleteApplies(t *testing.T, e *inboxDeleteEnv, matching BatchTransactionResultMsg, deletedID, nextID string) {
	t.Helper()
	e.m.handleBatchTransactionResult(matching)
	if e.reload(t).GetTaskByID(deletedID) != nil {
		t.Error("deleted task should be removed from disk")
	}
	if !e.processor.processedIDs[deletedID] {
		t.Error("matching result should mark the task processed")
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("pending slot should clear after the matching apply, got %+v", e.m.pendingInboxDeletes)
	}
	if got, _ := e.processor.currentTaskRef(); got == nil || got.ID != nextID {
		t.Errorf("cursor should advance to the next task, got %v", got)
	}
}

// assertInboxPendingIntact asserts an unrelated message left the in-flight delete
// armed and applied no processor bookkeeping.
func assertInboxPendingIntact(t *testing.T, e *inboxDeleteEnv, deletedID string) {
	t.Helper()
	if pending, ok := singleArmedInboxDelete(e.m); !ok || pending.taskID != deletedID {
		t.Fatalf("unrelated message consumed the in-flight Inbox delete, pending=%+v", e.m.pendingInboxDeletes)
	}
	if e.processor.processedIDs[deletedID] {
		t.Error("unrelated message must not mark the task processed")
	}
	if got, _ := e.processor.currentTaskRef(); got == nil || got.ID != deletedID {
		t.Errorf("cursor must stay on the pending task, got %v", got)
	}
}

// TestInboxDeleteUnrelatedResultDoesNotConsumePending: a committed delete from a
// DIFFERENT request (a different non-zero token) must not consume the in-flight
// Inbox delete; the matching result still applies afterward.
func TestInboxDeleteUnrelatedResultDoesNotConsumePending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	e.processor.currentIndex = 0
	matching, deletedID, nextID := armLocalInboxDelete(t, e)

	e.m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature:            "bulk",
		Action:             "Bulk Delete",
		RemovedTaskIDs:     []string{"unrelated-task"},
		DeleteRequestToken: matching.DeleteRequestToken + 100,
	})
	assertInboxPendingIntact(t, e, deletedID)

	assertInboxDeleteApplies(t, e, matching, deletedID, nextID)
}

// TestInboxDeleteUnrelatedBatchDoesNotConsumePending: a non-delete batch success
// (token 0) must not consume the in-flight Inbox delete.
func TestInboxDeleteUnrelatedBatchDoesNotConsumePending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	e.processor.currentIndex = 0
	matching, deletedID, nextID := armLocalInboxDelete(t, e)

	// A bulk transition / priority change carries no removed IDs and token 0.
	e.m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature: "bulk",
		Action:  "Bulk Set Priority",
		Summary: "Set priority on 3 task(s)",
	})
	assertInboxPendingIntact(t, e, deletedID)

	assertInboxDeleteApplies(t, e, matching, deletedID, nextID)
}

// TestInboxDeleteUnrelatedErrorDoesNotConsumePending: a transaction error from an
// unrelated command (token 0) must not disarm the in-flight Inbox delete.
func TestInboxDeleteUnrelatedErrorDoesNotConsumePending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	e.processor.currentIndex = 0
	matching, deletedID, nextID := armLocalInboxDelete(t, e)

	if handled, _ := e.m.handleGlobalSaveMessage(TransactionErrorMsg{
		Feature: "task-item",
		Action:  "Set Priority",
		Err:     errors.New("unrelated failure"),
	}); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	assertInboxPendingIntact(t, e, deletedID)

	assertInboxDeleteApplies(t, e, matching, deletedID, nextID)
}

// TestInboxDeleteOutOfOrderGlobalAutoCloseAfterInterleaved covers the Global
// Inbox processor: an unrelated success and an unrelated error arrive before the
// matching delete result, yet the matching result still rebuilds the queue,
// marks the task processed exactly once, and auto-closes (no unprocessed task
// remains) reporting the counts the header showed.
func TestInboxDeleteOutOfOrderGlobalAutoCloseAfterInterleaved(t *testing.T) {
	e := newInboxDeleteEnv(t)
	gp := NewGlobalInboxProcessorModel(e.m.configManager, e.m.runtime)
	if len(gp.inboxTasks) < 3 {
		t.Fatalf("global processor should aggregate the 3 seeded inbox tasks, got %d", len(gp.inboxTasks))
	}
	e.m.globalInboxProcessor = gp
	e.m.views.Register(ViewGlobalInboxProcessor, gp)
	e.m.pushView(ViewGlobalInboxProcessor)

	// Handle every other task so deleting the current one closes the session.
	gp.currentIndex = 0
	target := gp.inboxTasks[0].Task
	gp.processedIDs[gp.inboxTasks[1].Task.ID] = true
	gp.skippedIDs[gp.inboxTasks[2].Task.ID] = true

	reqCmd := gp.requestDelete()
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	if req.Origin != ViewGlobalInboxProcessor {
		t.Fatalf("origin = %v, want ViewGlobalInboxProcessor", req.Origin)
	}
	e.m.handleTaskDeleteRequested(req)
	_, confirmCmd := e.m.handleTaskDeleteConfirmKey("y")
	matching := runBatchResult(t, confirmCmd)
	if pending, ok := singleArmedInboxDelete(e.m); !ok || pending.origin != ViewGlobalInboxProcessor || pending.token == 0 {
		t.Fatalf("pending Global Inbox delete not armed, got %+v", e.m.pendingInboxDeletes)
	}

	// Interleave an unrelated committed delete (different token) and an unrelated
	// transaction error (token 0) before the matching result.
	e.m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature:            "bulk",
		Action:             "Bulk Delete",
		RemovedTaskIDs:     []string{"ghost"},
		DeleteRequestToken: matching.DeleteRequestToken + 100,
	})
	e.m.handleGlobalSaveMessage(TransactionErrorMsg{Feature: "x", Action: "y", Err: errors.New("boom")})
	if _, ok := singleArmedInboxDelete(e.m); !ok {
		t.Fatal("interleaved unrelated results must leave the Global Inbox delete armed")
	}

	// The matching result: rebuild reads committed data (the task is gone) and the
	// processor auto-closes. Pull the raw close command from the correlated apply.
	for _, pid := range matching.TouchedProjectIDs {
		e.m.reloadProjectByID(pid)
	}
	closeCmd := e.m.applyPendingInboxDelete(matching.DeleteRequestToken, matching.RemovedTaskIDs)
	if closeCmd == nil {
		t.Fatal("matching Global Inbox delete should auto-close when no unprocessed task remains")
	}
	if _, ok := closeCmd().(CloseGlobalInboxProcessorMsg); !ok {
		t.Fatalf("expected CloseGlobalInboxProcessorMsg, got %T", closeCmd())
	}
	if !gp.processedIDs[target.ID] {
		t.Error("matching result should mark the deleted task processed")
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("pending slot should clear after the matching apply, got %+v", e.m.pendingInboxDeletes)
	}
	// Exactly-once: a second matching apply is a no-op (pending already cleared).
	if cmd := e.m.applyPendingInboxDelete(matching.DeleteRequestToken, matching.RemovedTaskIDs); cmd != nil {
		t.Errorf("re-applying a consumed delete should be a no-op, got %v", cmd)
	}
}

// --- Weekly Review (FR-012) ---------------------------------------------------

// armWeeklyReviewDelete drives the Weekly Review surface through the canonical
// shell path: it selects the given tasks, presses X, confirms, and returns the
// committed matching batch. The pending count is armed with the minted token when
// this returns.
func armWeeklyReviewDelete(t *testing.T, e *taskDeleteEnv, m *AppModel, wr *WeeklyReviewModel) BatchTransactionResultMsg {
	t.Helper()
	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))
	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	matching := runBatchResult(t, confirmCmd)
	tok, count, armed := singleArmedWeeklyReviewDelete(m)
	if !armed || count == 0 {
		t.Fatalf("Weekly Review accounting not armed with a token, count=%d armed=%v", count, armed)
	}
	if matching.DeleteRequestToken != tok {
		t.Fatalf("matching batch token %d != armed token %d", matching.DeleteRequestToken, tok)
	}
	return matching
}

func assertWeeklyReviewPendingIntact(t *testing.T, m *AppModel, wr *WeeklyReviewModel) {
	t.Helper()
	if _, _, armed := singleArmedWeeklyReviewDelete(m); !armed {
		t.Fatalf("unrelated message consumed the in-flight Weekly Review count, pending=%v", m.pendingWeeklyReviewDeletes)
	}
	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Errorf("unrelated message must not credit accounting, got %d", got)
	}
}

// TestWeeklyReviewDeleteUnrelatedResultDoesNotConsumePending: a committed delete
// from a different request must neither credit nor clear the in-flight count.
func TestWeeklyReviewDeleteUnrelatedResultDoesNotConsumePending(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{{Task: e.sibling, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling)
	matching := armWeeklyReviewDelete(t, e, m, wr)

	m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature:            "bulk",
		Action:             "Bulk Delete",
		RemovedTaskIDs:     []string{e.lone.ID},
		DeleteRequestToken: matching.DeleteRequestToken + 100,
	})
	assertWeeklyReviewPendingIntact(t, m, wr)

	m.handleBatchTransactionResult(matching)
	if got := wr.mutationCounts["delete"]; got != 1 {
		t.Errorf("matching result should credit exactly one deletion, got %d", got)
	}
	if len(m.pendingWeeklyReviewDeletes) != 0 {
		t.Error("matching result should consume the pending count")
	}
}

// TestWeeklyReviewDeleteUnrelatedBatchDoesNotConsumePending: a non-delete batch
// success (token 0) must not credit or clear the in-flight count.
func TestWeeklyReviewDeleteUnrelatedBatchDoesNotConsumePending(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{{Task: e.sibling, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling)
	matching := armWeeklyReviewDelete(t, e, m, wr)

	m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature: "bulk",
		Action:  "Bulk Transition",
		Summary: "Moved 3 task(s)",
	})
	assertWeeklyReviewPendingIntact(t, m, wr)

	m.handleBatchTransactionResult(matching)
	if got := wr.mutationCounts["delete"]; got != 1 {
		t.Errorf("matching result should credit exactly one deletion, got %d", got)
	}
}

// TestWeeklyReviewDeleteUnrelatedErrorDoesNotConsumePending: a transaction error
// from an unrelated command (token 0) must not erase the in-flight count.
func TestWeeklyReviewDeleteUnrelatedErrorDoesNotConsumePending(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{{Task: e.sibling, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling)
	matching := armWeeklyReviewDelete(t, e, m, wr)

	if handled, _ := m.handleGlobalSaveMessage(TransactionErrorMsg{
		Feature: "task-item",
		Action:  "Set Priority",
		Err:     errors.New("unrelated failure"),
	}); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	assertWeeklyReviewPendingIntact(t, m, wr)

	m.handleBatchTransactionResult(matching)
	if got := wr.mutationCounts["delete"]; got != 1 {
		t.Errorf("matching result should still credit exactly one deletion, got %d", got)
	}
}

// TestWeeklyReviewDeleteOutOfOrderSuccessCreditsExactlyOnce: an unrelated success
// and an unrelated failure arrive before the matching successful Weekly Review
// delete; the summary is credited exactly once and reflects it.
func TestWeeklyReviewDeleteOutOfOrderSuccessCreditsExactlyOnce(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{
		{Task: e.sibling, Project: e.alpha},
		{Task: e.lone, Project: e.beta},
	}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling, e.lone)
	matching := armWeeklyReviewDelete(t, e, m, wr)

	// Interleave: unrelated committed delete (different token) then unrelated error.
	m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature:            "bulk",
		Action:             "Bulk Delete",
		RemovedTaskIDs:     []string{"ghost-a", "ghost-b"},
		DeleteRequestToken: matching.DeleteRequestToken + 100,
	})
	m.handleGlobalSaveMessage(TransactionErrorMsg{Feature: "x", Action: "y", Err: errors.New("boom")})
	assertWeeklyReviewPendingIntact(t, m, wr)

	m.handleBatchTransactionResult(matching)
	if got := wr.mutationCounts["delete"]; got != 2 {
		t.Fatalf("Deletions accounting = %d, want 2 roots", got)
	}
	// A duplicate delivery of the same result must not double-credit.
	m.handleBatchTransactionResult(matching)
	if got := wr.mutationCounts["delete"]; got != 2 {
		t.Errorf("duplicate matching result must not double-credit, got %d", got)
	}
	summary := wr.renderCompletionSummary()
	if !strings.Contains(summary, "Deletions: 2") {
		t.Errorf("summary Deletions row missing:\n%s", summary)
	}
}

// TestWeeklyReviewDeleteOutOfOrderFailureStaysUncounted: an unrelated success and
// an unrelated failure arrive before the matching FAILED Weekly Review delete;
// the matching failure clears the count and credits nothing.
func TestWeeklyReviewDeleteOutOfOrderFailureStaysUncounted(t *testing.T) {
	e := newTaskDeleteEnv(t)
	ghost := seedTask(t, 99, "Ghost", models.StateReady)
	rows := []WeeklyReviewTask{{Task: ghost, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, ghost)

	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))
	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	if armedWeeklyReviewDeleteTotal(m) != 1 {
		t.Fatalf("confirm arms the pending count, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	armedTok, _, _ := singleArmedWeeklyReviewDelete(m)
	out := confirmCmd()
	matchingErr, ok := out.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("a doomed delete should yield TransactionErrorMsg, got %T", out)
	}
	if matchingErr.DeleteRequestToken != armedTok {
		t.Fatalf("matching error token %d != armed token %d", matchingErr.DeleteRequestToken, armedTok)
	}

	// Interleave unrelated success + error before the matching failure.
	m.handleBatchTransactionResult(BatchTransactionResultMsg{
		Feature:            "bulk",
		Action:             "Bulk Delete",
		RemovedTaskIDs:     []string{"ghost-x"},
		DeleteRequestToken: matchingErr.DeleteRequestToken + 100,
	})
	m.handleGlobalSaveMessage(TransactionErrorMsg{Feature: "x", Action: "y", Err: errors.New("boom")})
	assertWeeklyReviewPendingIntact(t, m, wr)

	// The matching failure clears the count without crediting.
	if handled, _ := m.handleGlobalSaveMessage(matchingErr); !handled {
		t.Fatal("matching TransactionErrorMsg should be handled by the save-message path")
	}
	if len(m.pendingWeeklyReviewDeletes) != 0 {
		t.Errorf("matching failure should drop the armed count, got pending=%v", m.pendingWeeklyReviewDeletes)
	}
	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Errorf("a failed delete must not count a deletion, got %d", got)
	}
}

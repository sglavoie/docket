package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Session 14.1 repair (same-task-inbox-delete-correlation, FR-016): the shell
// keys Inbox post-commit bookkeeping by a unique per-request token
// (AppModel.pendingInboxDeletes), but the inbox processor keys the matching
// per-task focus/auto-close slot only by task ID (inboxCore.pendingDeletes).
// Because a confirmed Inbox task stays visible until its asynchronous delete
// result returns, the SAME task can be confirmed twice, minting two tokens that
// alias one processor slot. Before the fix, delivering the duplicate's failure
// first ran token-specific cleanup that deleted that shared slot, so the later
// successful result could not rebuild the queue, mark the task processed, repair
// focus, or auto-close.
//
// The shell now drops the processor's per-task slot only when no OTHER in-flight
// request still targets the same task (clearPendingDeleteBookkeeping /
// cancelInboxPendingDelete via inboxTaskHasOtherPendingDelete), so the slot
// survives until the last referencing request resolves and the committed delete's
// bookkeeping runs exactly once regardless of delivery order.
//
// Shared helpers: inboxDeleteEnv/newInboxDeleteEnv live in
// session4_inbox_delete_test.go; newGlobalInboxDeleteProcessor lives in
// session13_1_precommit_delete_bookkeeping_test.go.

// deliverInboxDeleteOutcome routes a raw delete outcome to the same shell paths
// production uses: a committed batch through handleBatchTransactionResult and a
// rejected transaction through the global save-message path (which reaches
// clearPendingDeleteBookkeeping).
func deliverInboxDeleteOutcome(t *testing.T, m *AppModel, msg tea.Msg) {
	t.Helper()
	switch typed := msg.(type) {
	case BatchTransactionResultMsg:
		m.handleBatchTransactionResult(typed)
	case TransactionErrorMsg:
		if handled, _ := m.handleGlobalSaveMessage(typed); !handled {
			t.Fatalf("TransactionErrorMsg should be handled by the save-message path")
		}
	default:
		t.Fatalf("unexpected delete outcome %T", msg)
	}
}

// confirmInboxDeleteRaw drives request -> confirm for the local processor at index
// WITHOUT delivering the result, returning the raw committed message. Unlike
// confirmInboxDeleteAt it does not assert the message type, so a duplicate
// same-task delete whose target a prior in-flight delete already committed (and so
// resolves as an empty batch or a TransactionErrorMsg) can be captured.
func confirmInboxDeleteRaw(t *testing.T, e *inboxDeleteEnv, index int) tea.Msg {
	t.Helper()
	e.processor.currentIndex = index
	req := requestInboxDelete(t, &e.processor.inboxCore)
	e.m.handleTaskDeleteRequested(req)
	_, cmd := e.m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return cmd()
}

// confirmGlobalInboxDeleteRaw is the Global Inbox analog of confirmInboxDeleteRaw.
func confirmGlobalInboxDeleteRaw(t *testing.T, m *AppModel, gp *GlobalInboxProcessorModel, index int) tea.Msg {
	t.Helper()
	gp.currentIndex = index
	req := requestInboxDelete(t, &gp.inboxCore)
	if req.Origin != ViewGlobalInboxProcessor {
		t.Fatalf("origin = %v, want ViewGlobalInboxProcessor", req.Origin)
	}
	m.handleTaskDeleteRequested(req)
	_, cmd := m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return cmd()
}

// TestLocalInboxDuplicateSameTaskDelete confirms the same local Inbox task for
// deletion twice while it is still visible, minting two tokens that alias one
// processor slot, then delivers the matching success and the duplicate's outcome
// in both orders. The committed delete must mark the task processed exactly once,
// rebuild the queue, re-anchor the cursor on the natural next task, and clear every
// bookkeeping slot regardless of which result lands first.
func TestLocalInboxDuplicateSameTaskDelete(t *testing.T) {
	for _, tc := range []struct {
		name         string
		duplicateFst bool // deliver the duplicate outcome before the matching success
	}{
		{name: "duplicate outcome first", duplicateFst: true},
		{name: "matching success first", duplicateFst: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newInboxDeleteEnv(t)
			idA, nextID := e.tasks[0].ID, e.tasks[1].ID
			identityA := TaskIdentity{ProjectID: e.project.ID, TaskID: idA}

			// First (real) delete of A commits to disk; its result is NOT delivered, so
			// A stays visible in the intact queue.
			realOut := confirmInboxDeleteRaw(t, e, 0)
			success, ok := realOut.(BatchTransactionResultMsg)
			if !ok {
				t.Fatalf("the first delete of A should commit, got %T", realOut)
			}

			// Confirm a SECOND delete of the still-visible A: a distinct token aliasing
			// the one processor slot — the FR-016 collision.
			dupOut := confirmInboxDeleteRaw(t, e, 0)
			if len(e.m.pendingInboxDeletes) != 2 {
				t.Fatalf("both same-task tokens should be armed, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(e.processor.pendingDeletes) != 1 {
				t.Fatalf("the two same-task tokens should alias ONE processor slot, got %d", len(e.processor.pendingDeletes))
			}

			if tc.duplicateFst {
				deliverInboxDeleteOutcome(t, e.m, dupOut)
				deliverInboxDeleteOutcome(t, e.m, success)
			} else {
				deliverInboxDeleteOutcome(t, e.m, success)
				deliverInboxDeleteOutcome(t, e.m, dupOut)
			}

			if !e.processor.processedIDs[identityA] {
				t.Errorf("the committed task should be marked processed, got %v", e.processor.processedIDs)
			}
			if len(e.m.pendingInboxDeletes) != 0 {
				t.Errorf("both token slots should clear after their outcomes, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(e.processor.pendingDeletes) != 0 {
				t.Errorf("the aliased processor slot should clear once the delete applies, got %d", len(e.processor.pendingDeletes))
			}
			if e.reload(t).GetTaskByID(idA) != nil {
				t.Error("the deleted task should be gone from disk")
			}
			if got, _ := e.processor.currentTaskRef(); got == nil || got.ID != nextID {
				t.Errorf("cursor should rest on the natural next task, got %v", got)
			}
		})
	}
}

// TestGlobalInboxDuplicateSameTaskDelete is the Global Inbox analog: two tokens
// for one aggregated task alias one processor slot, and the committed delete must
// mark the task processed exactly once and clear all bookkeeping in either order.
func TestGlobalInboxDuplicateSameTaskDelete(t *testing.T) {
	for _, tc := range []struct {
		name         string
		duplicateFst bool
	}{
		{name: "duplicate outcome first", duplicateFst: true},
		{name: "matching success first", duplicateFst: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newInboxDeleteEnv(t)
			gp := newGlobalInboxDeleteProcessor(t, e)
			idA := gp.inboxTasks[0].Task.ID
			identityA := taskIdentityFor(gp.inboxTasks[0].Task, gp.inboxTasks[0].Project)

			realOut := confirmGlobalInboxDeleteRaw(t, e.m, gp, 0)
			success, ok := realOut.(BatchTransactionResultMsg)
			if !ok {
				t.Fatalf("the first global delete of A should commit, got %T", realOut)
			}

			dupOut := confirmGlobalInboxDeleteRaw(t, e.m, gp, 0)
			if len(e.m.pendingInboxDeletes) != 2 {
				t.Fatalf("both same-task tokens should be armed, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(gp.pendingDeletes) != 1 {
				t.Fatalf("the two same-task tokens should alias ONE processor slot, got %d", len(gp.pendingDeletes))
			}

			if tc.duplicateFst {
				deliverInboxDeleteOutcome(t, e.m, dupOut)
				deliverInboxDeleteOutcome(t, e.m, success)
			} else {
				deliverInboxDeleteOutcome(t, e.m, success)
				deliverInboxDeleteOutcome(t, e.m, dupOut)
			}

			if !gp.processedIDs[identityA] {
				t.Errorf("the committed task should be marked processed, got %v", gp.processedIDs)
			}
			if len(e.m.pendingInboxDeletes) != 0 {
				t.Errorf("both token slots should clear after their outcomes, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(gp.pendingDeletes) != 0 {
				t.Errorf("the aliased processor slot should clear once the delete applies, got %d", len(gp.pendingDeletes))
			}
			if e.reload(t).GetTaskByID(idA) != nil {
				t.Error("the deleted task should be gone from disk")
			}
		})
	}
}

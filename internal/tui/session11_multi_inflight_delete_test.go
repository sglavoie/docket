package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Session 11.1 repair (multi-inflight-delete-bookkeeping, FR-013): the session
// 10.1 per-request tokens correlate an in-flight delete against UNRELATED
// results, but the token-associated Inbox and Weekly Review bookkeeping still
// lived in one singleton slot per surface. Confirming a SECOND same-surface
// delete before the first asynchronous result arrived overwrote that slot, so the
// first commit missed its processor accounting (Inbox) or completion-summary
// credit (Weekly Review). The bookkeeping is now keyed per request token (Inbox
// and Weekly Review) and per task ID (the inbox processor's focus/auto-close
// metadata), so two overlapping deletes each retain their own accounting in
// either completion order and with mixed success/failure.
//
// Shared helpers: inboxDeleteEnv/newInboxDeleteEnv live in
// session4_inbox_delete_test.go; taskDeleteEnv/newTaskDeleteEnv /
// newWeeklyReviewDeleteModel / runBatchResult live in the session2/9 delete tests.

// singleArmedInboxDelete returns the one armed inbox bookkeeping entry when
// exactly one delete is in flight (the single-delete case most tests use); it
// reports ok=false when none — or several — are armed.
func singleArmedInboxDelete(m *AppModel) (pendingInboxDelete, bool) {
	if len(m.pendingInboxDeletes) != 1 {
		return pendingInboxDelete{}, false
	}
	for _, p := range m.pendingInboxDeletes {
		return p, true
	}
	return pendingInboxDelete{}, false
}

// armedWeeklyReviewDeleteTotal sums the root counts of every in-flight Weekly
// Review delete (0 when none is armed).
func armedWeeklyReviewDeleteTotal(m *AppModel) int {
	total := 0
	for _, c := range m.pendingWeeklyReviewDeletes {
		total += c
	}
	return total
}

// singleArmedWeeklyReviewDelete returns the one armed Weekly Review token/count
// when exactly one delete is in flight; ok=false when none — or several — are armed.
func singleArmedWeeklyReviewDelete(m *AppModel) (token uint64, count int, ok bool) {
	if len(m.pendingWeeklyReviewDeletes) != 1 {
		return 0, 0, false
	}
	for tok, c := range m.pendingWeeklyReviewDeletes {
		return tok, c, true
	}
	return 0, 0, false
}

// confirmInboxDeleteAt drives the processor at index through the canonical shell
// path (request -> confirm) WITHOUT delivering the result, returning the
// committed matching batch. Both the AppModel token slot and the processor's
// per-task focus slot are armed when this returns, so callers can arm a second
// overlapping delete before delivering either.
func confirmInboxDeleteAt(t *testing.T, e *inboxDeleteEnv, index int) BatchTransactionResultMsg {
	t.Helper()
	e.processor.currentIndex = index
	reqCmd := e.processor.requestDelete()
	if reqCmd == nil {
		t.Fatal("requestDelete returned nil")
	}
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	e.m.handleTaskDeleteRequested(req)
	_, cmd := e.m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return runBatchResult(t, cmd)
}

// TestInboxDeleteBackToBackInFlightAppliesBothOrders arms two real local-inbox
// delete commands (task A then task B) before delivering either result, then
// delivers them in both orders. Each committed delete must mark its own task
// processed exactly once and clear only its own bookkeeping — the second confirm
// must not clobber the first's per-token/per-task slot.
func TestInboxDeleteBackToBackInFlightAppliesBothOrders(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deliverBAtA bool // deliver B before A when true
	}{
		{name: "A then B", deliverBAtA: false},
		{name: "B then A", deliverBAtA: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newInboxDeleteEnv(t)
			idA, idB, survivor := e.tasks[0].ID, e.tasks[1].ID, e.tasks[2].ID

			// Arm both deletes: A at index 0, then B at index 1 (queue not yet rebuilt,
			// so both roots are still selectable). Neither result has arrived.
			matchingA := confirmInboxDeleteAt(t, e, 0)
			matchingB := confirmInboxDeleteAt(t, e, 1)
			if len(e.m.pendingInboxDeletes) != 2 {
				t.Fatalf("both overlapping deletes should be armed by token, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(e.processor.pendingDeletes) != 2 {
				t.Fatalf("processor should track both per-task focus slots, got %d", len(e.processor.pendingDeletes))
			}
			if matchingA.DeleteRequestToken == matchingB.DeleteRequestToken {
				t.Fatalf("each delete must mint a distinct token, both = %d", matchingA.DeleteRequestToken)
			}

			first, second := matchingA, matchingB
			if tc.deliverBAtA {
				first, second = matchingB, matchingA
			}
			e.m.handleBatchTransactionResult(first)
			e.m.handleBatchTransactionResult(second)

			if !e.processor.processedIDs[idA] || !e.processor.processedIDs[idB] {
				t.Errorf("both deleted tasks should be marked processed, got %v", e.processor.processedIDs)
			}
			if len(e.m.pendingInboxDeletes) != 0 {
				t.Errorf("both token slots should clear after their results, got %d", len(e.m.pendingInboxDeletes))
			}
			if len(e.processor.pendingDeletes) != 0 {
				t.Errorf("processor per-task slots should clear after their results, got %d", len(e.processor.pendingDeletes))
			}
			fresh := e.reload(t)
			if fresh.GetTaskByID(idA) != nil || fresh.GetTaskByID(idB) != nil {
				t.Error("both deleted tasks should be gone from disk")
			}
			// The cursor lands on the sole survivor regardless of delivery order.
			if got, _ := e.processor.currentTaskRef(); got == nil || got.ID != survivor {
				t.Errorf("cursor should rest on the surviving task, got %v", got)
			}
		})
	}
}

// TestGlobalInboxDeleteBackToBackInFlightAppliesBothOrders is the Global Inbox
// analog: two overlapping deletes across the aggregated queue must each mark
// their own task processed exactly once, in either completion order.
func TestGlobalInboxDeleteBackToBackInFlightAppliesBothOrders(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deliverBAtA bool
	}{
		{name: "A then B", deliverBAtA: false},
		{name: "B then A", deliverBAtA: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newInboxDeleteEnv(t)
			gp := NewGlobalInboxProcessorModel(e.m.configManager, e.m.runtime)
			if len(gp.inboxTasks) < 3 {
				t.Fatalf("global processor should aggregate the 3 seeded inbox tasks, got %d", len(gp.inboxTasks))
			}
			e.m.globalInboxProcessor = gp
			e.m.views.Register(ViewGlobalInboxProcessor, gp)
			e.m.pushView(ViewGlobalInboxProcessor)

			idA := gp.inboxTasks[0].Task.ID
			idB := gp.inboxTasks[1].Task.ID

			matchingA := confirmGlobalInboxDeleteAt(t, e.m, gp, 0)
			matchingB := confirmGlobalInboxDeleteAt(t, e.m, gp, 1)
			if len(e.m.pendingInboxDeletes) != 2 {
				t.Fatalf("both overlapping Global deletes should be armed by token, got %d", len(e.m.pendingInboxDeletes))
			}

			first, second := matchingA, matchingB
			if tc.deliverBAtA {
				first, second = matchingB, matchingA
			}
			e.m.handleBatchTransactionResult(first)
			e.m.handleBatchTransactionResult(second)

			if !gp.processedIDs[idA] || !gp.processedIDs[idB] {
				t.Errorf("both deleted tasks should be marked processed, got %v", gp.processedIDs)
			}
			if len(e.m.pendingInboxDeletes) != 0 {
				t.Errorf("both token slots should clear after their results, got %d", len(e.m.pendingInboxDeletes))
			}
		})
	}
}

// confirmGlobalInboxDeleteAt mirrors confirmInboxDeleteAt for the Global Inbox
// processor.
func confirmGlobalInboxDeleteAt(t *testing.T, m *AppModel, gp *GlobalInboxProcessorModel, index int) BatchTransactionResultMsg {
	t.Helper()
	gp.currentIndex = index
	reqCmd := gp.requestDelete()
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	if req.Origin != ViewGlobalInboxProcessor {
		t.Fatalf("origin = %v, want ViewGlobalInboxProcessor", req.Origin)
	}
	m.handleTaskDeleteRequested(req)
	_, cmd := m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return runBatchResult(t, cmd)
}

// confirmWeeklyReviewDelete arms a single-root Weekly Review-origin delete of the
// given task WITHOUT delivering the result, returning the confirm command's raw
// message so the caller can deliver a success (BatchTransactionResultMsg) or a
// failure (TransactionErrorMsg) in a chosen order.
func confirmWeeklyReviewDelete(t *testing.T, m *AppModel, task *models.Task, project *models.Project) tea.Msg {
	t.Helper()
	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:   []TaskProjectRef{{Task: task, Project: project}},
		Origin: ViewWeeklyReview,
	})
	_, cmd := m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return cmd()
}

// TestWeeklyReviewDeleteBackToBackInFlightCreditsEachOnce arms two real Weekly
// Review deletes before delivering either result, then delivers both successes in
// both orders. Each overlapping delete must be credited to the completion summary
// exactly once — the second confirm must not overwrite the first's count.
func TestWeeklyReviewDeleteBackToBackInFlightCreditsEachOnce(t *testing.T) {
	for _, tc := range []struct {
		name        string
		deliverBAtA bool
	}{
		{name: "A then B", deliverBAtA: false},
		{name: "B then A", deliverBAtA: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTaskDeleteEnv(t)
			rows := []WeeklyReviewTask{
				{Task: e.sibling, Project: e.alpha},
				{Task: e.lone, Project: e.beta},
			}
			m, wr := newWeeklyReviewDeleteModel(t, e, rows)

			outA := confirmWeeklyReviewDelete(t, m, e.sibling, e.alpha)
			outB := confirmWeeklyReviewDelete(t, m, e.lone, e.beta)
			if len(m.pendingWeeklyReviewDeletes) != 2 {
				t.Fatalf("both overlapping Weekly Review deletes should be armed by token, got %d", len(m.pendingWeeklyReviewDeletes))
			}
			matchingA, ok := outA.(BatchTransactionResultMsg)
			if !ok {
				t.Fatalf("delete A should commit, got %T", outA)
			}
			matchingB, ok := outB.(BatchTransactionResultMsg)
			if !ok {
				t.Fatalf("delete B should commit, got %T", outB)
			}

			first, second := matchingA, matchingB
			if tc.deliverBAtA {
				first, second = matchingB, matchingA
			}
			m.handleBatchTransactionResult(first)
			m.handleBatchTransactionResult(second)

			if got := wr.mutationCounts["delete"]; got != 2 {
				t.Fatalf("each overlapping delete should be credited once (want 2), got %d", got)
			}
			if len(m.pendingWeeklyReviewDeletes) != 0 {
				t.Errorf("both counts should be consumed after their results, got %d", len(m.pendingWeeklyReviewDeletes))
			}
			// A duplicate delivery of either result must not double-credit.
			m.handleBatchTransactionResult(first)
			if got := wr.mutationCounts["delete"]; got != 2 {
				t.Errorf("duplicate result must not double-credit, got %d", got)
			}
		})
	}
}

// TestWeeklyReviewDeleteBackToBackInFlightMixedOutcomes arms two overlapping
// Weekly Review deletes where one commits and one fails (a root missing from
// disk), then delivers the pair in both orders. The success must credit exactly
// one deletion and the failure must discard only its own count, never the other's.
func TestWeeklyReviewDeleteBackToBackInFlightMixedOutcomes(t *testing.T) {
	for _, tc := range []struct {
		name         string
		failureFirst bool
	}{
		{name: "failure then success", failureFirst: true},
		{name: "success then failure", failureFirst: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTaskDeleteEnv(t)
			ghost := seedTask(t, 99, "Ghost", models.StateReady) // never persisted -> delete fails
			rows := []WeeklyReviewTask{
				{Task: e.sibling, Project: e.alpha},
				{Task: ghost, Project: e.alpha},
			}
			m, wr := newWeeklyReviewDeleteModel(t, e, rows)

			okOut := confirmWeeklyReviewDelete(t, m, e.sibling, e.alpha)
			failOut := confirmWeeklyReviewDelete(t, m, ghost, e.alpha)
			if len(m.pendingWeeklyReviewDeletes) != 2 {
				t.Fatalf("both overlapping deletes should be armed by token, got %d", len(m.pendingWeeklyReviewDeletes))
			}
			success, ok := okOut.(BatchTransactionResultMsg)
			if !ok {
				t.Fatalf("the on-disk delete should commit, got %T", okOut)
			}
			failure, ok := failOut.(TransactionErrorMsg)
			if !ok {
				t.Fatalf("the missing-root delete should fail, got %T", failOut)
			}
			if success.DeleteRequestToken == failure.DeleteRequestToken {
				t.Fatalf("success and failure must carry distinct tokens, both = %d", success.DeleteRequestToken)
			}

			deliver := func() {
				if tc.failureFirst {
					if handled, _ := m.handleGlobalSaveMessage(failure); !handled {
						t.Fatal("TransactionErrorMsg should be handled by the save-message path")
					}
					m.handleBatchTransactionResult(success)
				} else {
					m.handleBatchTransactionResult(success)
					if handled, _ := m.handleGlobalSaveMessage(failure); !handled {
						t.Fatal("TransactionErrorMsg should be handled by the save-message path")
					}
				}
			}
			deliver()

			if got := wr.mutationCounts["delete"]; got != 1 {
				t.Fatalf("only the committed delete should be credited (want 1), got %d", got)
			}
			if len(m.pendingWeeklyReviewDeletes) != 0 {
				t.Errorf("both the credited and the discarded count should be gone, got %d", len(m.pendingWeeklyReviewDeletes))
			}
		})
	}
}

// TestWeeklyReviewDeleteBackToBackInFlightIsolatesFailureCredit guards the exact
// FR-013 regression in isolation: arming a second delete must not disturb the
// first's armed count, and a failure on one token must leave the other's credit
// pending until its own success arrives.
func TestWeeklyReviewDeleteBackToBackInFlightIsolatesFailureCredit(t *testing.T) {
	e := newTaskDeleteEnv(t)
	ghost := seedTask(t, 99, "Ghost", models.StateReady)
	rows := []WeeklyReviewTask{
		{Task: e.sibling, Project: e.alpha},
		{Task: ghost, Project: e.alpha},
	}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows)

	// Arm the doomed delete FIRST, then the good one: the good arm must not be
	// clobbered by the earlier token still occupying a slot.
	failOut := confirmWeeklyReviewDelete(t, m, ghost, e.alpha)
	failure, ok := failOut.(TransactionErrorMsg)
	if !ok {
		t.Fatalf("the missing-root delete should fail, got %T", failOut)
	}
	if _, count, armed := singleArmedWeeklyReviewDelete(m); !armed || count != 1 {
		t.Fatalf("the doomed delete should be the only armed count (1), got armed=%v count=%d", armed, count)
	}

	okOut := confirmWeeklyReviewDelete(t, m, e.sibling, e.alpha)
	success, ok := okOut.(BatchTransactionResultMsg)
	if !ok {
		t.Fatalf("the on-disk delete should commit, got %T", okOut)
	}
	if armedWeeklyReviewDeleteTotal(m) != 2 {
		t.Fatalf("both counts should be armed, got %d", armedWeeklyReviewDeleteTotal(m))
	}

	// The failure clears ONLY its token; the good delete's credit survives.
	if handled, _ := m.handleGlobalSaveMessage(failure); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if _, count, armed := singleArmedWeeklyReviewDelete(m); !armed || count != 1 {
		t.Fatalf("the good delete's count must survive the unrelated failure, got armed=%v count=%d", armed, count)
	}
	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Fatalf("nothing should be credited before the good delete's result, got %d", got)
	}

	m.handleBatchTransactionResult(success)
	if got := wr.mutationCounts["delete"]; got != 1 {
		t.Errorf("the good delete should credit exactly one deletion, got %d", got)
	}
	if len(m.pendingWeeklyReviewDeletes) != 0 {
		t.Errorf("all counts should be consumed, got %d", len(m.pendingWeeklyReviewDeletes))
	}
}

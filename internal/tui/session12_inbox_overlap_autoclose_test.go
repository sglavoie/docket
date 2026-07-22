package tui

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// Session 12.1 repair (overlapping-inbox-autoclose, FR-014): deleting the final
// two Inbox tasks back-to-back left the processor open on an empty queue. Each
// delete snapshotted willClose against the intact pre-commit queue, so with the
// other task still queued both captured willClose=false; applyCommittedDelete
// rebuilt an empty committed queue but never recomputed closure, so neither
// result emitted the close message. Closure is now recomputed at apply time
// (shouldCloseAfterDelete) against the rebuilt committed queue plus the remaining
// in-flight deletes, so the final pair closes exactly once in either result order
// and a mixed success/failure pair keeps the processor open until a retry.
//
// Shared helpers: inboxDeleteEnv/newInboxDeleteEnv and confirmInboxDeleteAt live
// in session4_inbox_delete_test.go and session11_multi_inflight_delete_test.go;
// collectCommandMessages lives in inbox_session4_test.go.

// deliverInboxDeleteResult replays the inbox-relevant half of
// handleBatchTransactionResult for a committed delete: it reloads the touched
// projects (so the processor's rebuild reads committed data) and applies the
// token-correlated post-commit bookkeeping, returning the inbox command (a close
// command, or nil). It deliberately skips the unrelated save/refresh fan-out —
// whose status-timeout tick blocks a synchronous message drain — because
// auto-close is emitted solely by applyPendingInboxDelete.
func deliverInboxDeleteResult(m *AppModel, msg BatchTransactionResultMsg) tea.Cmd {
	for _, pid := range msg.TouchedProjectIDs {
		m.reloadProjectByID(pid)
	}
	return m.applyPendingInboxDelete(msg.DeleteRequestToken, msg.RemovedTaskIDs)
}

// inboxCloseCounts runs each command and reports how many local/global inbox
// close messages were emitted, plus the counts carried by the last one seen.
func inboxCloseCounts(t *testing.T, cmds ...tea.Cmd) (closes, processed, skipped int) {
	t.Helper()
	for _, cmd := range cmds {
		for _, msg := range collectCommandMessages(cmd) {
			switch c := msg.(type) {
			case CloseInboxProcessorMsg:
				closes++
				processed, skipped = c.ProcessedCount, c.SkippedCount
			case CloseGlobalInboxProcessorMsg:
				closes++
				processed, skipped = c.ProcessedCount, c.SkippedCount
			}
		}
	}
	return
}

// armInboxDeleteNoCommit arms a canonical delete for the task at index through
// the shell (request -> confirm) but does NOT run the commit command, so the
// task stays on disk. It returns the minted request token so a caller can later
// deliver a fabricated TransactionErrorMsg for it (the failing half of a mixed
// overlapping pair).
func armInboxDeleteNoCommit(t *testing.T, m *AppModel, c *inboxCore, index int) uint64 {
	t.Helper()
	c.currentIndex = index
	reqCmd := c.requestDelete()
	if reqCmd == nil {
		t.Fatal("requestDelete returned nil")
	}
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	m.handleTaskDeleteRequested(req)
	if _, cmd := m.handleTaskDeleteConfirmKey("y"); cmd == nil {
		t.Fatal("confirm should dispatch the delete command")
	}
	return m.deleteRequestSeq
}

// TestInboxDeleteFinalPairBackToBackAutoCloses arms two overlapping local-inbox
// deletes of the FINAL two unprocessed tasks (a third task is pre-skipped) before
// delivering either result, then delivers them in both orders. The processor must
// auto-close exactly once — from the last result to commit — with the complete
// processed/skipped counts, regardless of delivery order.
func TestInboxDeleteFinalPairBackToBackAutoCloses(t *testing.T) {
	for _, tc := range []struct {
		name          string
		deliverBFirst bool
	}{
		{name: "A then B", deliverBFirst: false},
		{name: "B then A", deliverBFirst: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newInboxDeleteEnv(t)
			idA, idB, idC := e.tasks[0].ID, e.tasks[1].ID, e.tasks[2].ID

			// Skip C so A and B are the final two unprocessed inbox tasks.
			e.processor.skippedIDs[idC] = true

			matchingA := confirmInboxDeleteAt(t, e, 0)
			matchingB := confirmInboxDeleteAt(t, e, 1)
			if len(e.processor.pendingDeletes) != 2 {
				t.Fatalf("both final-pair deletes should be armed, got %d", len(e.processor.pendingDeletes))
			}

			first, second := matchingA, matchingB
			if tc.deliverBFirst {
				first, second = matchingB, matchingA
			}
			cmd1 := deliverInboxDeleteResult(e.m, first)
			cmd2 := deliverInboxDeleteResult(e.m, second)

			closes, processed, skipped := inboxCloseCounts(t, cmd1, cmd2)
			if closes != 1 {
				t.Fatalf("processor should auto-close exactly once, got %d close messages", closes)
			}
			if processed != 2 || skipped != 1 {
				t.Errorf("close counts = (processed=%d skipped=%d), want (2 1)", processed, skipped)
			}
			if !e.processor.processedIDs[idA] || !e.processor.processedIDs[idB] {
				t.Errorf("both deleted tasks should be marked processed, got %v", e.processor.processedIDs)
			}
			if len(e.processor.pendingDeletes) != 0 {
				t.Errorf("all per-task slots should clear, got %d", len(e.processor.pendingDeletes))
			}
			if len(e.m.pendingInboxDeletes) != 0 {
				t.Errorf("both token slots should clear, got %d", len(e.m.pendingInboxDeletes))
			}
			fresh := e.reload(t)
			if fresh.GetTaskByID(idA) != nil || fresh.GetTaskByID(idB) != nil {
				t.Error("both deleted tasks should be gone from disk")
			}
		})
	}
}

// TestGlobalInboxDeleteFinalPairBackToBackAutoCloses is the Global Inbox analog:
// overlapping deletes of the aggregated queue's final two unprocessed tasks must
// auto-close the global processor exactly once, in either result order.
func TestGlobalInboxDeleteFinalPairBackToBackAutoCloses(t *testing.T) {
	for _, tc := range []struct {
		name          string
		deliverBFirst bool
	}{
		{name: "A then B", deliverBFirst: false},
		{name: "B then A", deliverBFirst: true},
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
			idC := gp.inboxTasks[2].Task.ID

			// Skip C so A and B are the final two unprocessed tasks in the queue.
			gp.skippedIDs[idC] = true

			matchingA := confirmGlobalInboxDeleteAt(t, e.m, gp, 0)
			matchingB := confirmGlobalInboxDeleteAt(t, e.m, gp, 1)
			if len(gp.pendingDeletes) != 2 {
				t.Fatalf("both final-pair deletes should be armed, got %d", len(gp.pendingDeletes))
			}

			first, second := matchingA, matchingB
			if tc.deliverBFirst {
				first, second = matchingB, matchingA
			}
			cmd1 := deliverInboxDeleteResult(e.m, first)
			cmd2 := deliverInboxDeleteResult(e.m, second)

			closes, processed, skipped := inboxCloseCounts(t, cmd1, cmd2)
			if closes != 1 {
				t.Fatalf("global processor should auto-close exactly once, got %d close messages", closes)
			}
			if processed != 2 || skipped != 1 {
				t.Errorf("close counts = (processed=%d skipped=%d), want (2 1)", processed, skipped)
			}
			if !gp.processedIDs[idA] || !gp.processedIDs[idB] {
				t.Errorf("both deleted tasks should be marked processed, got %v", gp.processedIDs)
			}
			if len(gp.pendingDeletes) != 0 {
				t.Errorf("all per-task slots should clear, got %d", len(gp.pendingDeletes))
			}
		})
	}
}

// TestInboxDeleteFinalPairBackToBackMixedOutcomeThenAutoCloses arms two
// overlapping local-inbox deletes of the final pair where one commits and one
// fails. The failure must leave its task in the queue and the processor open;
// deleting that surviving task then closes the processor normally with the full
// counts. This guards that closure is gated on live in-flight state, not a stale
// snapshot: the successful delete of the pair must NOT close on its own while the
// failed task remains.
func TestInboxDeleteFinalPairBackToBackMixedOutcomeThenAutoCloses(t *testing.T) {
	e := newInboxDeleteEnv(t)
	idA, idB, idC := e.tasks[0].ID, e.tasks[1].ID, e.tasks[2].ID

	// Skip C so A and B are the final two unprocessed inbox tasks.
	e.processor.skippedIDs[idC] = true

	// A commits (real transaction); B is armed but never committed, so it stays on
	// disk and its result arrives as a rejection.
	matchingA := confirmInboxDeleteAt(t, e, 0)
	tokenB := armInboxDeleteNoCommit(t, e.m, &e.processor.inboxCore, 1)
	if len(e.processor.pendingDeletes) != 2 || len(e.m.pendingInboxDeletes) != 2 {
		t.Fatalf("both overlapping deletes should be armed, got proc=%d shell=%d",
			len(e.processor.pendingDeletes), len(e.m.pendingInboxDeletes))
	}

	// Deliver A's success first: the processor must stay open because B is still
	// in flight (and still queued).
	closes, _, _ := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matchingA))
	if closes != 0 {
		t.Fatalf("the processor must stay open while the other delete is in flight, got %d closes", closes)
	}
	if !e.processor.processedIDs[idA] {
		t.Error("the committed delete should be marked processed")
	}

	// Deliver B's failure: it clears only its own bookkeeping and leaves B in the
	// queue, so the processor stays open.
	failure := TransactionErrorMsg{Err: errors.New("delete rejected"), DeleteRequestToken: tokenB}
	if handled, _ := e.m.handleGlobalSaveMessage(failure); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if len(e.processor.pendingDeletes) != 0 {
		t.Errorf("the failed delete should drop its per-task slot, got %d", len(e.processor.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("the failed delete should drop its token slot, got %d", len(e.m.pendingInboxDeletes))
	}
	if e.reload(t).GetTaskByID(idB) == nil {
		t.Fatal("the failed delete's task should remain on disk")
	}

	// Retry deleting the surviving task B: it is the final unprocessed task, so the
	// processor now closes exactly once with the complete counts.
	e.processor.rebuildTasks()
	bIndex := -1
	for i := 0; i < e.processor.taskCount(); i++ {
		if task, _ := e.processor.taskRef(i); task.ID == idB {
			bIndex = i
			break
		}
	}
	if bIndex < 0 {
		t.Fatalf("surviving task B should still be in the queue")
	}
	matchingB := confirmInboxDeleteAt(t, e, bIndex)
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matchingB))
	if closes != 1 {
		t.Fatalf("retrying the surviving task should close the processor exactly once, got %d", closes)
	}
	if processed != 2 || skipped != 1 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (2 1)", processed, skipped)
	}
	if fresh := e.reload(t); fresh.GetTaskByID(idA) != nil || fresh.GetTaskByID(idB) != nil {
		t.Error("both deleted tasks should be gone from disk after the retry")
	}
}

// TestGlobalInboxDeleteFinalPairBackToBackMixedOutcomeThenAutoCloses is the Global
// Inbox analog of the mixed-outcome case: a failed overlapping delete keeps the
// global processor open with its task, and retrying that task closes exactly once.
func TestGlobalInboxDeleteFinalPairBackToBackMixedOutcomeThenAutoCloses(t *testing.T) {
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
	idC := gp.inboxTasks[2].Task.ID
	gp.skippedIDs[idC] = true

	matchingA := confirmGlobalInboxDeleteAt(t, e.m, gp, 0)
	tokenB := armInboxDeleteNoCommit(t, e.m, &gp.inboxCore, 1)
	if len(gp.pendingDeletes) != 2 || len(e.m.pendingInboxDeletes) != 2 {
		t.Fatalf("both overlapping deletes should be armed, got proc=%d shell=%d",
			len(gp.pendingDeletes), len(e.m.pendingInboxDeletes))
	}

	closes, _, _ := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matchingA))
	if closes != 0 {
		t.Fatalf("the global processor must stay open while the other delete is in flight, got %d closes", closes)
	}
	if !gp.processedIDs[idA] {
		t.Error("the committed delete should be marked processed")
	}

	failure := TransactionErrorMsg{Err: errors.New("delete rejected"), DeleteRequestToken: tokenB}
	if handled, _ := e.m.handleGlobalSaveMessage(failure); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if len(gp.pendingDeletes) != 0 {
		t.Errorf("the failed delete should drop its per-task slot, got %d", len(gp.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("the failed delete should drop its token slot, got %d", len(e.m.pendingInboxDeletes))
	}

	gp.rebuildTasks()
	bIndex := -1
	for i := 0; i < gp.taskCount(); i++ {
		if task, _ := gp.taskRef(i); task.ID == idB {
			bIndex = i
			break
		}
	}
	if bIndex < 0 {
		t.Fatalf("surviving task B should still be in the queue")
	}
	matchingB := confirmGlobalInboxDeleteAt(t, e.m, gp, bIndex)
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matchingB))
	if closes != 1 {
		t.Fatalf("retrying the surviving task should close the global processor exactly once, got %d", closes)
	}
	if processed != 2 || skipped != 1 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (2 1)", processed, skipped)
	}
}

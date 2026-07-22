package tui

import (
	"os"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// Session 13.1 repair (inbox-precommit-delete-bookkeeping, FR-015): an Inbox
// delete arms in-flight bookkeeping in two layers before its transaction is even
// created. requestDelete records the processor's per-task focus slot
// (inboxCore.pendingDeletes) at request time, and confirmTaskDelete arms the
// shell's per-token slot (AppModel.pendingInboxDeletes) before it builds the
// revision-checked planning artifact. Two abort paths used to strand that state:
//
//   - Canceling the shell confirmation cleared only pendingTaskDelete, leaving the
//     processor's per-task slot behind (no token exists yet to key a cleanup).
//   - A planning-artifact build failure returned an ErrorMsg — never the
//     TransactionErrorMsg that drives clearPendingDeleteBookkeeping — so both the
//     processor- and shell-level slots stayed armed.
//
// Either orphan makes shouldCloseAfterDelete treat the aborted request as a live
// delete (len(pendingDeletes) > 0), suppressing a later final-task auto-close so
// the processor is left open on an empty queue. The fixes clear only the aborted
// request's own slots, so an overlapping delete on a different task is untouched.
//
// Shared helpers: inboxDeleteEnv/newInboxDeleteEnv live in
// session4_inbox_delete_test.go; confirmInboxDeleteAt / confirmGlobalInboxDeleteAt
// live in session11_multi_inflight_delete_test.go; deliverInboxDeleteResult /
// inboxCloseCounts live in session12_inbox_overlap_autoclose_test.go.

// TestLocalInboxDeleteCancelClearsPending arms a local Inbox delete, cancels the
// shell confirmation, and asserts the canceled request's processor slot is dropped
// with task data and cursor untouched. It then skips the remaining tasks and
// deletes the final one to prove the canceled request no longer suppresses the
// auto-close.
func TestLocalInboxDeleteCancelClearsPending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	idA, idB, idC := e.tasks[0].ID, e.tasks[1].ID, e.tasks[2].ID
	proc := e.processor
	proc.currentIndex = 0

	req := requestInboxDelete(t, &proc.inboxCore)
	e.m.handleTaskDeleteRequested(req)
	if _, ok := proc.pendingDeletes[idA]; !ok {
		t.Fatal("request should arm the processor per-task slot")
	}
	if handled, _ := e.m.handleTaskDeleteConfirmKey("esc"); !handled {
		t.Fatal("cancel key should be handled by the shell confirm")
	}

	// Cancellation clears only the canceled request's processor slot; no token was
	// minted, and task data and cursor state are unchanged.
	if len(proc.pendingDeletes) != 0 {
		t.Errorf("cancel should clear the processor per-task slot, got %d", len(proc.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("cancel arms no shell token, got %d", len(e.m.pendingInboxDeletes))
	}
	if e.reload(t).GetTaskByID(idA) == nil {
		t.Error("canceled delete must leave the task on disk")
	}
	if got, _ := proc.currentTaskRef(); got == nil || got.ID != idA {
		t.Errorf("cursor should stay on the canceled task, got %v", got)
	}

	// The orphan no longer suppresses a later close: skip A and B, then delete the
	// final unprocessed task C — the processor closes exactly once.
	proc.skippedIDs[idA] = true
	proc.skippedIDs[idB] = true
	matching := confirmInboxDeleteAt(t, e, 2)
	if matching.RemovedTaskIDs[0] != idC {
		t.Fatalf("final delete should target C, removed %v", matching.RemovedTaskIDs)
	}
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matching))
	if closes != 1 {
		t.Fatalf("deleting the final task after a canceled delete should close once, got %d", closes)
	}
	if processed != 1 || skipped != 2 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (1 2)", processed, skipped)
	}
}

// TestGlobalInboxDeleteCancelClearsPending is the Global Inbox analogue: canceling
// a global Inbox delete drops the global processor's per-task slot, leaves task
// data intact, and does not suppress the final-task auto-close.
func TestGlobalInboxDeleteCancelClearsPending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	gp := newGlobalInboxDeleteProcessor(t, e)
	idA := gp.inboxTasks[0].Task.ID
	idB := gp.inboxTasks[1].Task.ID
	idC := gp.inboxTasks[2].Task.ID
	gp.currentIndex = 0

	req := requestInboxDelete(t, &gp.inboxCore)
	e.m.handleTaskDeleteRequested(req)
	if _, ok := gp.pendingDeletes[idA]; !ok {
		t.Fatal("request should arm the global processor per-task slot")
	}
	if handled, _ := e.m.handleTaskDeleteConfirmKey("n"); !handled {
		t.Fatal("cancel key should be handled by the shell confirm")
	}

	if len(gp.pendingDeletes) != 0 {
		t.Errorf("cancel should clear the global processor per-task slot, got %d", len(gp.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("cancel arms no shell token, got %d", len(e.m.pendingInboxDeletes))
	}
	if e.reload(t).GetTaskByID(idA) == nil {
		t.Error("canceled delete must leave the task on disk")
	}
	if got, _ := gp.currentTaskRef(); got == nil || got.ID != idA {
		t.Errorf("cursor should stay on the canceled task, got %v", got)
	}

	gp.skippedIDs[idA] = true
	gp.skippedIDs[idB] = true
	matching := confirmGlobalInboxDeleteAt(t, e.m, gp, 2)
	if matching.RemovedTaskIDs[0] != idC {
		t.Fatalf("final delete should target C, removed %v", matching.RemovedTaskIDs)
	}
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matching))
	if closes != 1 {
		t.Fatalf("deleting the final global task after a cancel should close once, got %d", closes)
	}
	if processed != 1 || skipped != 2 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (1 2)", processed, skipped)
	}
}

// TestLocalInboxDeleteArtifactBuildErrorClearsPending forces the planning-artifact
// build in confirmTaskDelete to fail (a corrupt planning.json makes LoadPlanning
// error) after both the processor- and shell-level slots are armed. The aborted
// request must clear both, leave the task on disk, and no longer suppress the
// final-task auto-close.
func TestLocalInboxDeleteArtifactBuildErrorClearsPending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	idA, idB, idC := e.tasks[0].ID, e.tasks[1].ID, e.tasks[2].ID
	proc := e.processor
	proc.currentIndex = 0

	restore := armArtifactBuildFailure(t, e, idA)

	req := requestInboxDelete(t, &proc.inboxCore)
	e.m.handleTaskDeleteRequested(req)
	_, cmd := e.m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch a command")
	}
	if _, ok := cmd().(ErrorMsg); !ok {
		t.Fatalf("artifact build failure should surface as ErrorMsg, got %T", cmd())
	}

	// Both the processor per-task slot (armed in requestDelete) and the shell token
	// slot (armed in confirmTaskDelete) are cleared for the aborted request.
	if len(proc.pendingDeletes) != 0 {
		t.Errorf("artifact failure should clear the processor per-task slot, got %d", len(proc.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("artifact failure should clear the shell token slot, got %d", len(e.m.pendingInboxDeletes))
	}
	if e.reload(t).GetTaskByID(idA) == nil {
		t.Error("aborted delete must leave the task on disk")
	}

	restore()
	proc.skippedIDs[idA] = true
	proc.skippedIDs[idB] = true
	matching := confirmInboxDeleteAt(t, e, 2)
	if matching.RemovedTaskIDs[0] != idC {
		t.Fatalf("final delete should target C, removed %v", matching.RemovedTaskIDs)
	}
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matching))
	if closes != 1 {
		t.Fatalf("deleting the final task after an aborted delete should close once, got %d", closes)
	}
	if processed != 1 || skipped != 2 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (1 2)", processed, skipped)
	}
}

// TestGlobalInboxDeleteArtifactBuildErrorClearsPending is the Global Inbox
// analogue of the artifact-build failure abort path.
func TestGlobalInboxDeleteArtifactBuildErrorClearsPending(t *testing.T) {
	e := newInboxDeleteEnv(t)
	gp := newGlobalInboxDeleteProcessor(t, e)
	idA := gp.inboxTasks[0].Task.ID
	idB := gp.inboxTasks[1].Task.ID
	idC := gp.inboxTasks[2].Task.ID
	gp.currentIndex = 0

	restore := armArtifactBuildFailure(t, e, idA)

	req := requestInboxDelete(t, &gp.inboxCore)
	e.m.handleTaskDeleteRequested(req)
	_, cmd := e.m.handleTaskDeleteConfirmKey("y")
	if cmd == nil {
		t.Fatal("confirm should dispatch a command")
	}
	if _, ok := cmd().(ErrorMsg); !ok {
		t.Fatalf("artifact build failure should surface as ErrorMsg, got %T", cmd())
	}

	if len(gp.pendingDeletes) != 0 {
		t.Errorf("artifact failure should clear the global processor per-task slot, got %d", len(gp.pendingDeletes))
	}
	if len(e.m.pendingInboxDeletes) != 0 {
		t.Errorf("artifact failure should clear the shell token slot, got %d", len(e.m.pendingInboxDeletes))
	}
	if e.reload(t).GetTaskByID(idA) == nil {
		t.Error("aborted delete must leave the task on disk")
	}

	restore()
	gp.skippedIDs[idA] = true
	gp.skippedIDs[idB] = true
	matching := confirmGlobalInboxDeleteAt(t, e.m, gp, 2)
	if matching.RemovedTaskIDs[0] != idC {
		t.Fatalf("final delete should target C, removed %v", matching.RemovedTaskIDs)
	}
	closes, processed, skipped := inboxCloseCounts(t, deliverInboxDeleteResult(e.m, matching))
	if closes != 1 {
		t.Fatalf("deleting the final global task after an aborted delete should close once, got %d", closes)
	}
	if processed != 1 || skipped != 2 {
		t.Errorf("close counts = (processed=%d skipped=%d), want (1 2)", processed, skipped)
	}
}

// requestInboxDelete drives an inbox core's requestDelete and returns the emitted
// canonical delete request message, failing the test if the command is nil or
// carries the wrong message type.
func requestInboxDelete(t *testing.T, c *inboxCore) TaskDeleteRequestedMsg {
	t.Helper()
	reqCmd := c.requestDelete()
	if reqCmd == nil {
		t.Fatal("requestDelete returned nil")
	}
	req, ok := reqCmd().(TaskDeleteRequestedMsg)
	if !ok {
		t.Fatalf("requestDelete emitted %T, want TaskDeleteRequestedMsg", reqCmd())
	}
	return req
}

// newGlobalInboxDeleteProcessor builds a Global Inbox processor over the shared
// env, registers it, and makes it the active view — the setup every Global Inbox
// delete test repeats.
func newGlobalInboxDeleteProcessor(t *testing.T, e *inboxDeleteEnv) *GlobalInboxProcessorModel {
	t.Helper()
	gp := NewGlobalInboxProcessorModel(e.m.configManager, e.m.runtime)
	if len(gp.inboxTasks) < 3 {
		t.Fatalf("global processor should aggregate the 3 seeded inbox tasks, got %d", len(gp.inboxTasks))
	}
	e.m.globalInboxProcessor = gp
	e.m.views.Register(ViewGlobalInboxProcessor, gp)
	e.m.pushView(ViewGlobalInboxProcessor)
	return gp
}

// armArtifactBuildFailure seeds a durable focus-queue reference for taskID (so the
// delete removes at least one durable planning ref and the artifact build runs)
// and corrupts planning.json on disk (so DomainCoordinator.LoadPlanning fails
// inside planningDomainArtifact). It returns a restore func that reinstates the
// original planning.json and drops the seeded ref, so a follow-up delete of a
// ref-free task commits normally.
func armArtifactBuildFailure(t *testing.T, e *inboxDeleteEnv, taskID string) (restore func()) {
	t.Helper()
	planningPath := config.PlanningStatePath(e.m.runtime.DataDir())
	original, err := os.ReadFile(planningPath)
	if err != nil {
		t.Fatalf("read seeded planning.json: %v", err)
	}
	e.m.runtime.Planning().FocusQueue = []config.FocusQueueItem{{ProjectID: e.project.ID, TaskID: taskID}}
	if err := os.WriteFile(planningPath, []byte("{ not valid json"), 0o644); err != nil {
		t.Fatalf("corrupt planning.json: %v", err)
	}
	return func() {
		if err := os.WriteFile(planningPath, original, 0o644); err != nil {
			t.Fatalf("restore planning.json: %v", err)
		}
		e.m.runtime.Planning().FocusQueue = nil
	}
}

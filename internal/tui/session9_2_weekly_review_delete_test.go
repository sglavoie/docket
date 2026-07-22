package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
)

// Session 9.2 repair (weekly-review-delete-accounting, FR-010): Weekly Review
// deletions route through the shell-owned canonical TaskDeleteRequestedMsg, which
// has no Weekly Review post-commit callback, so a successful deletion was omitted
// from the completion summary's total and Deletions row. The shell now arms a
// per-request root count in confirmTaskDelete (Weekly Review origin only) and
// credits the summary from handleBatchTransactionResult — only after commit — so
// cancellation, transaction failures, and unrelated-origin deletes stay
// uncounted. These tests drive the built keymap's "X" action through the real
// transactional shell path. taskDeleteEnv/appModel live in
// session2_task_delete_transaction_test.go; findSharedTaskKeymapAction /
// deleteRequestFromCmd / runBatchResult are shared test helpers.

// newWeeklyReviewDeleteModel registers a WeeklyReviewModel over the shared delete
// env with the given rows visible and the given tasks multi-selected, so the
// Weekly Review surface can exercise the canonical shell path end to end.
func newWeeklyReviewDeleteModel(t *testing.T, e *taskDeleteEnv, rows []WeeklyReviewTask, selected ...*models.Task) (*AppModel, *WeeklyReviewModel) {
	t.Helper()
	m := e.appModel()
	// Build through the real constructor so every map/mixin is initialized (a bare
	// struct panics on the post-commit reload), then override the visible rows and
	// selection so the delete targets are deterministic.
	wr := NewWeeklyReviewModel(m.runtime, e.mgr)
	wr.mutationCounts = make(map[string]int)
	wr.staleTasks = rows
	wr.priorityMismatch = nil
	wr.upcomingWeek = nil
	wr.recurringOverview = nil
	if len(selected) > 0 {
		wr.multiSelectMode = true
		for _, tk := range selected {
			wr.selectedTasks[tk.ID] = true
		}
	}
	m.views.Register(ViewWeeklyReview, wr)
	return m, wr
}

// TestWeeklyReviewDeleteCommitCreditsAccountingByRootCount proves a committed
// Weekly Review-origin deletion increments the Deletions accounting by the number
// of requested roots (the confirmation-prompt count), NOT the number of removed
// descendant tasks: deleting the parent epic removes parent+child on disk but
// credits exactly one deletion, matching the per-task unit of the other summary
// counters.
func TestWeeklyReviewDeleteCommitCreditsAccountingByRootCount(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{
		{Task: e.parent, Project: e.alpha},
		{Task: e.sibling, Project: e.alpha},
	}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.parent)

	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	msg := deleteRequestFromCmd(t, cmd)
	if msg.Origin != ViewWeeklyReview {
		t.Fatalf("origin = %v, want ViewWeeklyReview", msg.Origin)
	}

	m.handleTaskDeleteRequested(msg)
	if armedWeeklyReviewDeleteTotal(m) != 0 {
		t.Errorf("arming the confirm must not credit accounting yet, got %d", armedWeeklyReviewDeleteTotal(m))
	}

	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	if armedWeeklyReviewDeleteTotal(m) != 1 {
		t.Fatalf("confirm should arm one pending root-count, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	batch := runBatchResult(t, confirmCmd)
	if len(batch.RemovedTaskIDs) != 2 {
		t.Fatalf("removed ids = %v, want parent+child", batch.RemovedTaskIDs)
	}

	m.handleBatchTransactionResult(batch)
	if armedWeeklyReviewDeleteTotal(m) != 0 {
		t.Errorf("pending count should be consumed after commit, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	if got := wr.mutationCounts["delete"]; got != 1 {
		t.Errorf("Deletions accounting = %d, want 1 root (not %d removed descendants)", got, len(batch.RemovedTaskIDs))
	}
}

// TestWeeklyReviewDeleteCommitSummaryReportsDeletions proves the rendered
// completion summary reflects a committed multi-selection delete in both the
// overall total and the Deletions row.
func TestWeeklyReviewDeleteCommitSummaryReportsDeletions(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{
		{Task: e.sibling, Project: e.alpha},
		{Task: e.lone, Project: e.beta},
	}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling, e.lone)

	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))
	if got := m.taskDeleteConfirm.Prompt(); got != "Delete 2 tasks?" {
		t.Fatalf("prompt = %q, want %q", got, "Delete 2 tasks?")
	}

	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	batch := runBatchResult(t, confirmCmd)
	m.handleBatchTransactionResult(batch)

	if got := wr.mutationCounts["delete"]; got != 2 {
		t.Fatalf("Deletions accounting = %d, want 2", got)
	}
	summary := wr.renderCompletionSummary()
	if !strings.Contains(summary, "Changes made during this review: 2") {
		t.Errorf("summary total missing the committed deletions:\n%s", summary)
	}
	if !strings.Contains(summary, "Deletions: 2") {
		t.Errorf("summary Deletions row missing:\n%s", summary)
	}
}

// TestWeeklyReviewDeleteCancellationSkipsAccounting proves cancelling the shell
// confirm (n or Esc) never arms the accounting and preserves the tasks on disk.
func TestWeeklyReviewDeleteCancellationSkipsAccounting(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		e := newTaskDeleteEnv(t)
		rows := []WeeklyReviewTask{{Task: e.sibling, Project: e.alpha}}
		m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.sibling)

		action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
		_, cmd := action()
		m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))

		handled, out := m.handleTaskDeleteConfirmKey(key)
		if !handled {
			t.Fatalf("%q should be handled by the modal confirm", key)
		}
		if out != nil {
			t.Errorf("%q cancel should not dispatch a delete command", key)
		}
		if armedWeeklyReviewDeleteTotal(m) != 0 {
			t.Errorf("%q cancel must not arm accounting, got %d", key, armedWeeklyReviewDeleteTotal(m))
		}
		if got := wr.mutationCounts["delete"]; got != 0 {
			t.Errorf("%q cancel must not count a deletion, got %d", key, got)
		}
		if e.reload(t, e.alpha).GetTaskByID(e.sibling.ID) == nil {
			t.Errorf("%q cancel must preserve the task on disk", key)
		}
	}
}

// TestWeeklyReviewDeleteFailureSkipsAccounting proves a rejected delete (a root
// not on disk) surfaces a TransactionErrorMsg, whose save-message handler drops
// the armed accounting so a failed delete stays uncounted.
func TestWeeklyReviewDeleteFailureSkipsAccounting(t *testing.T) {
	e := newTaskDeleteEnv(t)
	ghost := seedTask(t, 99, "Ghost", models.StateReady)
	rows := []WeeklyReviewTask{{Task: ghost, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, ghost)

	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))

	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	if confirmCmd == nil {
		t.Fatal("confirm should dispatch a command even for a doomed delete")
	}
	if armedWeeklyReviewDeleteTotal(m) != 1 {
		t.Fatalf("confirm arms the pending count before commit, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	out := confirmCmd()
	if _, ok := out.(TransactionErrorMsg); !ok {
		t.Fatalf("failed delete should yield TransactionErrorMsg, got %T", out)
	}
	if handled, _ := m.handleGlobalSaveMessage(out); !handled {
		t.Fatal("TransactionErrorMsg should be handled by the save-message path")
	}
	if armedWeeklyReviewDeleteTotal(m) != 0 {
		t.Errorf("a rejected delete must drop the armed accounting, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Errorf("a failed delete must not count a deletion, got %d", got)
	}
}

// TestWeeklyReviewDeleteUnrelatedOriginSkipsAccounting proves a committed delete
// that did NOT originate from Weekly Review leaves the Weekly Review accounting
// untouched, even while a WeeklyReviewModel is registered.
func TestWeeklyReviewDeleteUnrelatedOriginSkipsAccounting(t *testing.T) {
	e := newTaskDeleteEnv(t)
	rows := []WeeklyReviewTask{{Task: e.sibling, Project: e.alpha}}
	// Register the Weekly Review surface but drive the delete from All Tasks.
	m, wr := newWeeklyReviewDeleteModel(t, e, rows)

	m.handleTaskDeleteRequested(TaskDeleteRequestedMsg{
		Refs:   []TaskProjectRef{{Task: e.sibling, Project: e.alpha}},
		Origin: ViewAllTasks,
	})
	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	if armedWeeklyReviewDeleteTotal(m) != 0 {
		t.Fatalf("a non-Weekly-Review origin must not arm accounting, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	batch := runBatchResult(t, confirmCmd)
	m.handleBatchTransactionResult(batch)

	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Errorf("an unrelated-origin delete must not credit Weekly Review, got %d", got)
	}
}

// TestWeeklyReviewDeleteArtifactBuildErrorSkipsAccounting proves an artifact-build
// failure (config planning load error) aborts confirmTaskDelete before it arms the
// accounting, so a delete that never commits stays uncounted. Arming is placed
// after the artifact build precisely so this early return leaves the count at
// zero.
func TestWeeklyReviewDeleteArtifactBuildErrorSkipsAccounting(t *testing.T) {
	e := newTaskDeleteEnv(t)
	// The parent subtree contains the durable focus-queue ref (the child), so
	// confirmTaskDelete must build the checked planning artifact — corrupting the
	// config file makes that build fail.
	rows := []WeeklyReviewTask{{Task: e.parent, Project: e.alpha}}
	m, wr := newWeeklyReviewDeleteModel(t, e, rows, e.parent)

	if err := os.WriteFile(config.PlanningStatePath(e.dataDir), []byte("{ not valid json"), 0o600); err != nil {
		t.Fatalf("corrupt planning.json: %v", err)
	}

	action := findSharedTaskKeymapAction(t, buildWeeklyReviewKeyMap(wr), "X")
	_, cmd := action()
	m.handleTaskDeleteRequested(deleteRequestFromCmd(t, cmd))

	_, confirmCmd := m.handleTaskDeleteConfirmKey("y")
	if confirmCmd == nil {
		t.Fatal("confirm should dispatch a command")
	}
	if _, ok := confirmCmd().(TransactionErrorMsg); ok {
		t.Fatal("expected an artifact-build error command, not a transaction result")
	}
	if armedWeeklyReviewDeleteTotal(m) != 0 {
		t.Errorf("an artifact-build error must not arm accounting, got %d", armedWeeklyReviewDeleteTotal(m))
	}
	if got := wr.mutationCounts["delete"]; got != 0 {
		t.Errorf("an aborted delete must not count a deletion, got %d", got)
	}
}

package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// newCrossOutlinerFromEnv builds a cross-project OutlinerModel wired to the
// seeded projects' BulkCoordinator and workspace ledger (via cfg.DataDir()), so its
// bulk mutations commit through the application layer and its u/ctrl+r handlers
// restore from the same ledger the coordinator wrote to.
func newCrossOutlinerFromEnv(env *kanbanBulkEnv) *OutlinerModel {
	cfg := newTestConfigAt(env.dataDir)
	setTestProjects(cfg, env.projects)
	m := &OutlinerModel{
		PrimaryViewBundle: NewPrimaryViewBundle(nil, testRuntimeAt(cfg.DataDir(), cfg)),
		collapsedSections: make(map[string]bool),
		expandedNodes:     make(map[string]bool),
		MultiSelectState:  MultiSelectState{selectedTasks: make(map[string]bool)},
		activePerspective: 1,
		loadedProjects:    env.projects,
	}
	m.bulk = env.bulk
	m.taskItem = env.taskItem
	return m
}

// TestCrossProjectUndoOrchestration pins the migrated cross-project bulk status
// change: it commits ONE workspace-ledger transaction across two projects through
// the BulkCoordinator (no legacy per-project .undo backups), and the cross-project
// u / ctrl+r handlers undo and redo it from the ledger end to end.
func TestCrossProjectUndoOrchestration(t *testing.T) { //nolint:gocyclo // integration-style test pins a full undo/redo orchestration path.
	alpha1 := seedTask(t, 1, "Implement login endpoint", models.StateReady)
	alpha2 := seedTask(t, 2, "Fix typo in readme", models.StateReady)
	beta1 := seedTask(t, 1, "Configure lint", models.StateReady)
	beta2 := seedTask(t, 2, "Write API docs", models.StateReady)

	env := newKanbanBulkEnv(t, []*models.Task{alpha1, alpha2}, []*models.Task{beta1, beta2})
	m := newCrossOutlinerFromEnv(env)

	m.bulkOp.SetOperation(models.StateInProgress, []*models.Task{alpha1, beta1}, 0, nil)

	resultModel, cmd := m.crossExecutePendingBulkStatus()
	m = resultModel.(*OutlinerModel)
	msg := runBatchResult(t, cmd)
	if msg.Summary != "Changed 2 tasks to in_progress" {
		t.Errorf("summary = %q, want %q", msg.Summary, "Changed 2 tasks to in_progress")
	}
	if len(msg.TouchedProjectIDs) != 2 {
		t.Errorf("touched projects = %v, want 2 (cross-project)", msg.TouchedProjectIDs)
	}

	// The batch committed the transition to disk in both projects.
	if got := env.reload(t, 0).GetTaskByID(alpha1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("alpha1 committed state = %v, want in_progress", got)
	}
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("beta1 committed state = %v, want in_progress", got)
	}

	// The workspace ledger backs the batch (its newest entry is the bulk change),
	// and no legacy per-project .undo backups were written.
	entry, ok, err := env.ledger.SelectNewestUndo(storage.GlobalScope())
	if err != nil || !ok {
		t.Fatalf("ledger has no undo entry after bulk change (ok=%v err=%v)", ok, err)
	}
	if entry.Action != "Bulk Status Change" {
		t.Errorf("newest ledger entry action = %q, want %q", entry.Action, "Bulk Status Change")
	}
	if storage.UndoHistoryExists(env.projects[0]) || storage.UndoHistoryExists(env.projects[1]) {
		t.Fatal("no legacy per-project undo history should exist after a ledger-backed bulk change")
	}

	// Undo via the cross-project handler restores both projects from the ledger.
	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Undid: Bulk Status Change") {
		t.Errorf("undo status should describe the ledger restore, got %q", m.statusMessage)
	}
	if got := env.reload(t, 0).GetTaskByID(alpha1.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("alpha1 state after undo = %v, want ready", got)
	}
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("beta1 state after undo = %v, want ready", got)
	}

	// Redo via the cross-project handler replays the batch from the ledger.
	redoResult, _, ok := m.crossHandleRedo("ctrl+r")
	if !ok {
		t.Fatal("crossHandleRedo returned ok=false in cross-project mode")
	}
	m = redoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Redo: Bulk Status Change") {
		t.Errorf("redo status should describe the ledger replay, got %q", m.statusMessage)
	}
	if got := env.reload(t, 0).GetTaskByID(alpha1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("alpha1 state after redo = %v, want in_progress", got)
	}
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("beta1 state after redo = %v, want in_progress", got)
	}
}

// TestCrossProjectSingleTaskUndoOrchestration pins the migrated cross-project
// single-task status change: crossHandleStateMutation routes ONE task through
// the TaskItemCoordinator, committing a single workspace-ledger transaction (no
// legacy per-project .undo backups), and the cross-project u / ctrl+r handlers
// undo and redo it from that same ledger end to end.
func TestCrossProjectSingleTaskUndoOrchestration(t *testing.T) {
	alpha1 := seedTask(t, 1, "Implement login endpoint", models.StateReady)
	beta1 := seedTask(t, 1, "Configure lint", models.StateReady)

	env := newKanbanBulkEnv(t, []*models.Task{alpha1}, []*models.Task{beta1})
	m := newCrossOutlinerFromEnv(env)

	// Place the cursor on beta1 (project index 1) as a single-task target.
	m.rows = []outlinerRow{{task: beta1, ProjectID: env.projects[1].ID}}
	m.cursor = 0

	_, cmd, ok := m.crossHandleStateMutation(models.StateInProgress)
	if !ok {
		t.Fatal("crossHandleStateMutation returned ok=false in cross-project mode")
	}
	if res := runResult(t, cmd); res.Title != "Configure lint" {
		t.Errorf("result title = %q, want %q", res.Title, "Configure lint")
	}

	// The single-task transition committed to disk in the owning project only.
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("beta1 committed state = %v, want in_progress", got)
	}
	if got := env.reload(t, 0).GetTaskByID(alpha1.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("alpha1 state = %v, want untouched ready", got)
	}

	// The workspace ledger backs the change; no legacy per-project .undo backups.
	if _, has, err := env.ledger.SelectNewestUndo(storage.GlobalScope()); err != nil || !has {
		t.Fatalf("ledger has no undo entry after single-task change (has=%v err=%v)", has, err)
	}
	if storage.UndoHistoryExists(env.projects[0]) || storage.UndoHistoryExists(env.projects[1]) {
		t.Fatal("no legacy per-project undo history should exist after a ledger-backed change")
	}

	// Undo via the cross-project handler restores the task from the ledger.
	undoResult, _, ok := m.crossHandleUndo("u")
	if !ok {
		t.Fatal("crossHandleUndo returned ok=false in cross-project mode")
	}
	m = undoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Undid:") {
		t.Errorf("undo status = %q, want an Undid: message", m.statusMessage)
	}
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateReady {
		t.Fatalf("beta1 state after undo = %v, want ready", got)
	}

	// Redo via the cross-project handler replays it from the ledger.
	redoResult, _, ok := m.crossHandleRedo("ctrl+r")
	if !ok {
		t.Fatal("crossHandleRedo returned ok=false in cross-project mode")
	}
	m = redoResult.(*OutlinerModel)
	if !strings.HasPrefix(m.statusMessage, "Redo:") {
		t.Errorf("redo status = %q, want a Redo: message", m.statusMessage)
	}
	if got := env.reload(t, 1).GetTaskByID(beta1.ID); got == nil || got.State() != models.StateInProgress {
		t.Fatalf("beta1 state after redo = %v, want in_progress", got)
	}
}

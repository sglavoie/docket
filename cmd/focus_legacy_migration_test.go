package cmd

import (
	"testing"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
)

// TestFocusQueueFirstUseMigratesBeforePrune locks FR-009-003 for the CLI listing
// path: a legacy Focus Queue member (listed in the ordering-only index but whose
// workflow state predates the state-driven queue) is backfilled to StateNext
// BEFORE `tt focus queue` prunes non-StateNext entries. The first invocation must
// therefore migrate the member and return it, never delete the unmigrated queue
// that is its only evidence. Repeated invocations stay idempotent.
func TestFocusQueueFirstUseMigratesBeforePrune(t *testing.T) {
	project, configPath, cleanup := setupFocusMutateCLI(t)
	defer cleanup()
	taskID := project.Tasks[0].ID

	// Seed a LEGACY queue: the task is listed in the order index, but its state is
	// StateReady (not StateNext) — the pre-state-driven situation the migration
	// exists to repair.
	workspace, err := LoadCLIWorkspaceStrict()
	if err != nil {
		t.Fatalf("load workspace: %v", err)
	}
	domains, err := cloneCLIWorkspaceDomains(workspace)
	if err != nil {
		t.Fatalf("clone domains: %v", err)
	}
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{{TaskID: taskID, ProjectID: project.ID}})
	if err := commitCLIPlanning(workspace, domains.Planning); err != nil {
		t.Fatalf("seed legacy queue: %v", err)
	}
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateReady {
		t.Fatalf("precondition state = %s, want ready (legacy, unmigrated)", st)
	}

	// First `tt focus queue` listing migrates before pruning.
	if _, _, err := captureDoctorOutput(t, func() error {
		return runFocusQueue(focusQueueCmd, nil)
	}); err != nil {
		t.Fatalf("focus queue: %v", err)
	}

	// The legacy member was backfilled to StateNext — migrated, not dropped.
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("after listing state = %s, want next (migrated on first use)", st)
	}
	// The ordering-only entry survived: the prune did not delete the queue.
	if q := reloadFocusConfig(t, configPath).GetFocusQueue(); len(q) != 1 || q[0].TaskID != taskID {
		t.Fatalf("after listing queue = %+v, want the migrated entry preserved", q)
	}

	// A second listing is idempotent: no state change, no duplicate order entry.
	if _, _, err := captureDoctorOutput(t, func() error {
		return runFocusQueue(focusQueueCmd, nil)
	}); err != nil {
		t.Fatalf("focus queue (2nd): %v", err)
	}
	if st := reloadFocusTaskState(t, project, taskID); st != models.StateNext {
		t.Fatalf("after 2nd listing state = %s, want next", st)
	}
	if q := reloadFocusConfig(t, configPath).GetFocusQueue(); len(q) != 1 || q[0].TaskID != taskID {
		t.Fatalf("after 2nd listing queue = %+v, want no duplicate entries", q)
	}
}

// TestFocusQueueFirstUseMigrationUsesWorkspaceTransaction locks FR-013-003 for the
// CLI listing path: the automatic first-use legacy migration must journal its
// StateNext backfill through ONE revision-checked workspace transaction with a
// durable ledger entry (not raw per-project saves), commit every changed project
// atomically, survive a restart, and be restorable. Two projects each hold a legacy
// queue member (indexed but still pre-state-driven), so the batch spans projects and
// a single ledger undo must revert both together.
func TestFocusQueueFirstUseMigrationUsesWorkspaceTransaction(t *testing.T) {
	tmpDir := t.TempDir()
	depth := 10

	projA := newFileBackedProject(t, tmpDir, "proj-a", "Project A")
	taskA := addTestTask(t, projA, "task-a", "Alpha", models.StateReady)
	saveProjectForGroupedCLITest(t, projA)

	projB := newFileBackedProject(t, tmpDir, "proj-b", "Project B")
	taskB := addTestTask(t, projB, "task-b", "Beta", models.StateReady)
	saveProjectForGroupedCLITest(t, projB)

	// Both legacy members are recorded in the ordering-only index but neither is
	// StateNext yet — the pre-state-driven situation the migration repairs.
	preA := taskA.State()
	preB := taskB.State()
	if preA == models.StateNext || preB == models.StateNext {
		t.Fatalf("precondition: legacy members must not start StateNext (A=%s, B=%s)", preA, preB)
	}

	domains := config.NewDomains()
	domains.Catalog.UpsertProject(projA)
	domains.Catalog.UpsertProject(projB)
	domains.Settings.UndoHistoryDepth = &depth
	domains.Machine.CurrentProject = projA.ID
	domains.Planning.SetFocusQueue([]config.FocusQueueItem{
		{TaskID: taskA.ID, ProjectID: projA.ID},
		{TaskID: taskB.ID, ProjectID: projB.ID},
	})
	configureFileBackedGroupedCLI(t, config.NewRuntimeState(tmpDir, domains))

	// The first listing migrates both legacy members before pruning.
	if _, _, err := captureDoctorOutput(t, func() error {
		return runFocusQueue(focusQueueCmd, nil)
	}); err != nil {
		t.Fatalf("focus queue: %v", err)
	}

	// Both members were backfilled to StateNext, and the change survives a restart
	// (a fresh disk read reports StateNext).
	if st := reloadFocusTaskState(t, projA, taskA.ID); st != models.StateNext {
		t.Fatalf("task A after migration = %s, want next (survives restart)", st)
	}
	if st := reloadFocusTaskState(t, projB, taskB.ID); st != models.StateNext {
		t.Fatalf("task B after migration = %s, want next (survives restart)", st)
	}

	// The migration committed through the workspace undo ledger — not a raw save —
	// so exactly one restorable transaction is recorded.
	ledger := storage.NewWorkspaceLedger(config.UndoLedgerDir(tmpDir), depth, []string{tmpDir})
	if count, err := ledger.UndoCount(storage.GlobalScope()); err != nil || count != 1 {
		t.Fatalf("ledger UndoCount = %d, err=%v, want exactly 1 migration transaction", count, err)
	}

	// The transaction is restorable and atomic: one ledger undo reverts BOTH
	// projects to their pre-migration state in a single step, proving every changed
	// project committed in one workspace transaction rather than separate saves.
	if _, err := ledger.Undo(storage.GlobalScope()); err != nil {
		t.Fatalf("ledger.Undo: %v", err)
	}
	if st := reloadFocusTaskState(t, projA, taskA.ID); st != preA {
		t.Fatalf("task A after undo = %s, want %s (pre-migration)", st, preA)
	}
	if st := reloadFocusTaskState(t, projB, taskB.ID); st != preB {
		t.Fatalf("task B after undo = %s, want %s (pre-migration)", st, preB)
	}
}

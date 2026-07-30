package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/transaction"
)

func newArtifactLedgerTestService(t *testing.T) (*CommitService, *WorkspaceLedger, string) {
	t.Helper()
	dataDir := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	return service, ledger, dataDir
}

// TestCommitArtifactTransaction_MixedChangesUndoRedoByteExact proves an
// artifact-only transaction can create, replace, and delete files as one ledger
// entry, and that either affected project's history selection reverses and
// replays every byte without a tasks.md rewrite.
func TestCommitArtifactTransaction_MixedChangesUndoRedoByteExact(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	existingPath := filepath.Join(dataDir, "notes", "existing.md")
	createdPath := filepath.Join(dataDir, "notes", "created.md")
	deletedPath := filepath.Join(dataDir, "notes", "deleted.md")
	existingBefore := []byte{'o', 'l', 'd', 0, 0xff, '\n'}
	existingAfter := []byte{'n', 'e', 'w', 0, 0xfe, '\n'}
	createdAfter := []byte{'c', 'r', 'e', 'a', 't', 'e', 'd', 0}
	deletedBefore := []byte{'d', 'e', 'l', 'e', 't', 'e', 'd', 0, 0xfd}
	writeArtifactBytes(t, existingPath, existingBefore)
	writeArtifactBytes(t, deletedPath, deletedBefore)

	txnID, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts: []ArtifactChange{
			{Path: existingPath, Data: existingAfter, ExpectedRevision: strptr(RawArtifactRevision(existingBefore))},
			{Path: createdPath, Data: createdAfter, ExpectedRevision: strptr("")},
			{Path: deletedPath, Delete: true, ExpectedRevision: strptr(RawArtifactRevision(deletedBefore))},
		},
		ProjectIDs:  []string{"project-a", "project-b", "project-a"},
		UndoAction:  "Update notes",
		UndoContext: "three artifacts",
	})
	if err != nil {
		t.Fatalf("CommitArtifactTransaction: %v", err)
	}
	if txnID == "" {
		t.Fatal("transaction id is empty")
	}
	assertArtifactBytes(t, existingPath, existingAfter)
	assertArtifactBytes(t, createdPath, createdAfter)
	requireAbsent(t, deletedPath)

	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo(global) ok=%v err=%v", ok, err)
	}
	if entry.TransactionID != txnID {
		t.Fatalf("ledger transaction = %q, want %q", entry.TransactionID, txnID)
	}
	if len(entry.ProjectIDs) != 2 || entry.ProjectIDs[0] != "project-a" || entry.ProjectIDs[1] != "project-b" {
		t.Fatalf("ledger project ids = %v, want [project-a project-b]", entry.ProjectIDs)
	}
	for _, projectID := range []string{"project-a", "project-b"} {
		if _, selectable, selectErr := ledger.SelectNewestUndo(ProjectScope(projectID)); selectErr != nil || !selectable {
			t.Fatalf("SelectNewestUndo(%s) ok=%v err=%v", projectID, selectable, selectErr)
		}
	}

	description, err := ledger.Undo(ProjectScope("project-b"))
	if err != nil {
		t.Fatalf("Undo(project-b): %v", err)
	}
	if description != "Update notes: three artifacts" {
		t.Fatalf("undo description = %q", description)
	}
	assertArtifactBytes(t, existingPath, existingBefore)
	requireAbsent(t, createdPath)
	assertArtifactBytes(t, deletedPath, deletedBefore)

	if _, err := ledger.Redo(ProjectScope("project-a")); err != nil {
		t.Fatalf("Redo(project-a): %v", err)
	}
	assertArtifactBytes(t, existingPath, existingAfter)
	assertArtifactBytes(t, createdPath, createdAfter)
	requireAbsent(t, deletedPath)
	assertJournalClean(t, dataDir)
}

// TestCommitArtifactTransaction_StaleRevisionIsAllOrNone proves every revision
// is checked before a create, write, delete, or ledger append can land.
func TestCommitArtifactTransaction_StaleRevisionIsAllOrNone(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	createdPath := filepath.Join(dataDir, "notes", "must-stay-absent.md")
	currentPath := filepath.Join(dataDir, "notes", "current.md")
	deletedPath := filepath.Join(dataDir, "notes", "must-stay.md")
	current := []byte("concurrent bytes")
	deleted := []byte("keep bytes")
	writeArtifactBytes(t, currentPath, current)
	writeArtifactBytes(t, deletedPath, deleted)

	_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts: []ArtifactChange{
			{Path: createdPath, Data: []byte("create"), ExpectedRevision: strptr("")},
			{Path: currentPath, Data: []byte("clobber"), ExpectedRevision: strptr(RawArtifactRevision([]byte("stale")))},
			{Path: deletedPath, Delete: true, ExpectedRevision: strptr(RawArtifactRevision(deleted))},
		},
		ProjectIDs: []string{"project-a"},
		UndoAction: "stale batch",
	})
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("CommitArtifactTransaction err = %v, want ErrStaleArtifact", err)
	}
	requireAbsent(t, createdPath)
	assertArtifactBytes(t, currentPath, current)
	assertArtifactBytes(t, deletedPath, deleted)
	if _, ok, selectErr := ledger.SelectNewestUndo(GlobalScope()); selectErr != nil || ok {
		t.Fatalf("ledger changed after stale revision: ok=%v err=%v", ok, selectErr)
	}
	assertJournalClean(t, dataDir)
}

func TestCommitArtifactTransaction_PreconditionsAreReadOnlyAndAllOrNone(t *testing.T) {
	t.Run("stale", func(t *testing.T) {
		service, ledger, dataDir := newArtifactLedgerTestService(t)
		guardPath := filepath.Join(dataDir, ".tt", "workspace.json")
		targetPath := filepath.Join(dataDir, "notes", "guarded.md")
		observed := []byte("catalog before")
		concurrent := []byte("catalog concurrent")
		writeArtifactBytes(t, guardPath, observed)
		writeArtifactBytes(t, guardPath, concurrent)

		_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
			Artifacts: []ArtifactChange{{Path: targetPath, Data: []byte("note"), ExpectedRevision: strptr("")}},
			Preconditions: []ArtifactPrecondition{{
				Path:             guardPath,
				ExpectedRevision: RawArtifactRevision(observed),
			}},
			UndoAction: "guarded note",
		})
		if !errors.Is(err, ErrStaleArtifact) {
			t.Fatalf("CommitArtifactTransaction err = %v, want ErrStaleArtifact", err)
		}
		requireAbsent(t, targetPath)
		requireAbsent(t, filepath.Dir(targetPath))
		assertArtifactBytes(t, guardPath, concurrent)
		if _, ok, selectErr := ledger.SelectNewestUndo(GlobalScope()); selectErr != nil || ok {
			t.Fatalf("stale precondition recorded history: ok=%v err=%v", ok, selectErr)
		}
		assertJournalClean(t, dataDir)
	})

	t.Run("successful guard is excluded from history", func(t *testing.T) {
		service, ledger, dataDir := newArtifactLedgerTestService(t)
		guardPath := filepath.Join(dataDir, ".tt", "workspace.json")
		targetPath := filepath.Join(dataDir, "notes", "guarded.md")
		guardBytes := []byte("catalog stable")
		writeArtifactBytes(t, guardPath, guardBytes)

		_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
			Artifacts: []ArtifactChange{{Path: targetPath, Data: []byte("note"), ExpectedRevision: strptr("")}},
			Preconditions: []ArtifactPrecondition{{
				Path:             guardPath,
				ExpectedRevision: RawArtifactRevision(guardBytes),
			}},
			UndoAction: "guarded note",
		})
		if err != nil {
			t.Fatalf("CommitArtifactTransaction: %v", err)
		}
		entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
		if err != nil || !ok {
			t.Fatalf("SelectNewestUndo ok=%v err=%v", ok, err)
		}
		if len(entry.Artifacts) != 1 || entry.Artifacts[0].Path != targetPath {
			t.Fatalf("ledger artifacts = %+v, want only %s", entry.Artifacts, targetPath)
		}
		assertArtifactBytes(t, guardPath, guardBytes)
		if _, err := ledger.Undo(GlobalScope()); err != nil {
			t.Fatalf("Undo: %v", err)
		}
		requireAbsent(t, targetPath)
		assertArtifactBytes(t, guardPath, guardBytes)
	})

	t.Run("outside approved root", func(t *testing.T) {
		service, ledger, dataDir := newArtifactLedgerTestService(t)
		targetPath := filepath.Join(dataDir, "notes", "guarded.md")
		outsideGuard := filepath.Join(t.TempDir(), "workspace.json")
		_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
			Artifacts:     []ArtifactChange{{Path: targetPath, Data: []byte("note")}},
			Preconditions: []ArtifactPrecondition{{Path: outsideGuard}},
		})
		if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
			t.Fatalf("CommitArtifactTransaction err = %v, want ErrTargetOutsideRoot", err)
		}
		requireAbsent(t, targetPath)
		requireAbsent(t, outsideGuard)
		requireAbsent(t, outsideGuard+".lock")
		if _, ok, selectErr := ledger.SelectNewestUndo(GlobalScope()); selectErr != nil || ok {
			t.Fatalf("invalid precondition recorded history: ok=%v err=%v", ok, selectErr)
		}
	})
}

// TestCommitArtifactTransaction_RecoveryPrecedesCapture proves artifact-only
// commits reuse the workspace crash-recovery gate. A prepared older transaction
// is rolled forward before the new transaction captures its own undo images, and
// undoing the new entry does not reverse the recovered transaction.
func TestCommitArtifactTransaction_RecoveryPrecedesCapture(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	recoveredPath := filepath.Join(dataDir, "notes", "recovered.md")
	requestPath := filepath.Join(dataDir, "notes", "request.md")
	journal := transaction.NewJournal(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
	if _, err := journal.Prepare([]transaction.Op{{
		Kind: transaction.OpWrite, Target: recoveredPath, Data: []byte("recovered"), Mode: FilePermissions,
	}}); err != nil {
		t.Fatalf("prepare interrupted transaction: %v", err)
	}

	if _, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: requestPath, Data: []byte("request"), ExpectedRevision: strptr("")}},
		UndoAction: "new request",
	}); err != nil {
		t.Fatalf("CommitArtifactTransaction: %v", err)
	}
	assertArtifactBytes(t, recoveredPath, []byte("recovered"))
	assertArtifactBytes(t, requestPath, []byte("request"))

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo(global): %v", err)
	}
	assertArtifactBytes(t, recoveredPath, []byte("recovered"))
	requireAbsent(t, requestPath)
	assertJournalClean(t, dataDir)
}

// TestCommitArtifactTransaction_PreparedJournalRecoversArtifactsAndLedger
// simulates a process stop after the combined manifest becomes durable but
// before Commit starts. Recovery must land the user bytes, retained blobs, and
// ledger JSON as one roll-forward unit, leaving the recovered mutation undoable.
func TestCommitArtifactTransaction_PreparedJournalRecoversArtifactsAndLedger(t *testing.T) {
	_, ledger, dataDir := newArtifactLedgerTestService(t)
	path := filepath.Join(dataDir, "notes", "recover-with-ledger.md")
	before := []byte{'b', 'e', 'f', 'o', 'r', 'e', 0}
	after := []byte{'a', 'f', 't', 'e', 'r', 0xff}
	writeArtifactBytes(t, path, before)
	txnID := "artifact-ledger-recovery"

	plan, planErr := ledger.prepareRecord(LedgerRecord{
		TransactionID: txnID,
		Action:        "recover note",
		ProjectIDs:    []string{"project-a"},
		Artifacts: []LedgerArtifact{{
			Path: path, BeforeExists: true, Before: before, AfterExists: true, After: after,
		}},
	})
	if planErr != nil {
		t.Fatalf("prepareRecord: %v", planErr)
	}
	ops := []transaction.Op{{Kind: transaction.OpWrite, Target: path, Data: after, Mode: FilePermissions}}
	ops = append(ops, plan.ops...)
	journal := transaction.NewJournal(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir})
	if _, err := journal.PrepareWithID(txnID, ops); err != nil {
		t.Fatalf("PrepareWithID: %v", err)
	}
	assertArtifactBytes(t, path, before)
	if _, ok, err := ledger.SelectNewestUndo(GlobalScope()); err != nil || ok {
		t.Fatalf("prepared ledger visible before recovery: ok=%v err=%v", ok, err)
	}

	report, err := transaction.NewJournal(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}).Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.RolledForward) != 1 || report.RolledForward[0] != txnID {
		t.Fatalf("RolledForward = %v, want [%s]", report.RolledForward, txnID)
	}
	assertArtifactBytes(t, path, after)
	entry, ok, err := ledger.SelectNewestUndo(ProjectScope("project-a"))
	if err != nil || !ok || entry.TransactionID != txnID {
		t.Fatalf("recovered ledger entry = %#v ok=%v err=%v", entry, ok, err)
	}
	if _, err := ledger.Undo(ProjectScope("project-a")); err != nil {
		t.Fatalf("Undo recovered transaction: %v", err)
	}
	assertArtifactBytes(t, path, before)
	assertJournalClean(t, dataDir)
}

// TestCommitArtifactTransaction_LedgerPreparationFailureIsAllOrNone injects a
// malformed ledger before planning. The transaction must fail before staging or
// applying the requested artifact, and the malformed ledger bytes stay intact.
func TestCommitArtifactTransaction_LedgerPreparationFailureIsAllOrNone(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	path := filepath.Join(dataDir, "notes", "must-not-land.md")
	malformed := []byte("{not-json")
	writeArtifactBytes(t, ledger.ledgerPath(), malformed)

	_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("nope"), ExpectedRevision: strptr("")}},
		UndoAction: "must fail",
	})
	if err == nil {
		t.Fatal("CommitArtifactTransaction succeeded with a malformed ledger")
	}
	requireAbsent(t, path)
	assertArtifactBytes(t, ledger.ledgerPath(), malformed)
	assertJournalClean(t, dataDir)
}

// TestCommitArtifactTransaction_LedgerOutsideJournalRootIsAllOrNone proves the
// journal validates internal ledger targets together with user artifacts before
// staging. A misconfigured out-of-root ledger cannot allow the user write alone.
func TestCommitArtifactTransaction_LedgerOutsideJournalRootIsAllOrNone(t *testing.T) {
	dataDir := t.TempDir()
	outsideDir := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(outsideDir, "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	path := filepath.Join(dataDir, "notes", "must-not-land.md")

	_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("nope"), ExpectedRevision: strptr("")}},
		UndoAction: "must fail",
	})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitArtifactTransaction err = %v, want ErrTargetOutsideRoot", err)
	}
	requireAbsent(t, path)
	requireAbsent(t, ledger.ledgerPath())
	assertJournalClean(t, dataDir)
}

// TestCommitArtifactTransaction_RedoInvalidationAndScope covers the two ledger
// policies artifact-only callers rely on: a global-only entry is not selectable
// from a project history, and a new overlapping transaction invalidates redo.
func TestCommitArtifactTransaction_RedoInvalidationAndScope(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	path := filepath.Join(dataDir, "notes", "global.md")

	if _, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("v1"), ExpectedRevision: strptr("")}},
		UndoAction: "global note",
	}); err != nil {
		t.Fatalf("first CommitArtifactTransaction: %v", err)
	}
	if _, ok, err := ledger.SelectNewestUndo(ProjectScope("project-a")); err != nil || ok {
		t.Fatalf("global-only entry selected by project: ok=%v err=%v", ok, err)
	}
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo(global): %v", err)
	}
	if _, ok, err := ledger.SelectNewestRedo(GlobalScope()); err != nil || !ok {
		t.Fatalf("SelectNewestRedo(global) ok=%v err=%v", ok, err)
	}

	if _, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("v2"), ExpectedRevision: strptr("")}},
		ProjectIDs: []string{"project-a"},
		UndoAction: "replace global note",
	}); err != nil {
		t.Fatalf("second CommitArtifactTransaction: %v", err)
	}
	if _, ok, err := ledger.SelectNewestRedo(GlobalScope()); err != nil || ok {
		t.Fatalf("overlapping redo survived: ok=%v err=%v", ok, err)
	}
	if _, ok, err := ledger.SelectNewestUndo(ProjectScope("project-a")); err != nil || !ok {
		t.Fatalf("project entry not selectable: ok=%v err=%v", ok, err)
	}
	assertArtifactBytes(t, path, []byte("v2"))
}

// TestCommitArtifacts_LedgerBackedServiceStillSkipsHistory locks in the wrapper
// compatibility contract: config-only saves use the shared implementation but
// do not create user-visible workspace undo entries.
func TestCommitArtifacts_LedgerBackedServiceStillSkipsHistory(t *testing.T) {
	service, ledger, dataDir := newArtifactLedgerTestService(t)
	path := filepath.Join(dataDir, ".tt", "planning.json")
	if _, err := service.CommitArtifacts([]ArtifactChange{{Path: path, Data: []byte("{}")}}); err != nil {
		t.Fatalf("CommitArtifacts: %v", err)
	}
	assertArtifactBytes(t, path, []byte("{}"))
	if _, ok, err := ledger.SelectNewestUndo(GlobalScope()); err != nil || ok {
		t.Fatalf("CommitArtifacts recorded history: ok=%v err=%v", ok, err)
	}
}

func writeArtifactBytes(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertArtifactBytes(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s bytes = %v, want %v", path, got, want)
	}
}

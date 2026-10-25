package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/transaction"
)

// TestLedgerRestoreAtomicPreflightRejectsMissingBlob proves a grouped restore
// validates every retained image before staging the first target. One missing
// blob leaves all user files and both ledger stacks untouched.
func TestLedgerRestoreAtomicPreflightRejectsMissingBlob(t *testing.T) {
	ledger, root := newTestLedger(t)
	first := filepath.Join(root, "projects", "alpha", "tasks.md")
	second := filepath.Join(root, "projects", "beta", "tasks.md")
	writeFile(t, first, "alpha after")
	writeFile(t, second, "beta after")
	recordLedgerRestoreGroup(t, ledger, "missing-blob", "grouped edit", []LedgerArtifact{
		{Path: first, BeforeExists: true, Before: []byte("alpha before"), AfterExists: true, After: []byte("alpha after")},
		{Path: second, BeforeExists: true, Before: []byte("beta before"), AfterExists: true, After: []byte("beta after")},
	})

	if err := os.Remove(blobPathFor(ledger, "beta before")); err != nil {
		t.Fatalf("remove retained blob: %v", err)
	}
	if _, err := ledger.Undo(GlobalScope()); err == nil {
		t.Fatal("Undo succeeded with a missing retained blob")
	}
	assertLedgerRestoreContent(t, first, "alpha after")
	assertLedgerRestoreContent(t, second, "beta after")
	assertLedgerRestoreStacks(t, ledger, 1, 0)
	assertJournalClean(t, root)
}

// TestLedgerRestoreAtomicRevisionDrift rejects an undo when any grouped target
// no longer matches the entry's after-image. Preflight detects the drift before
// the earlier artifact can be restored, and the unrestorable entry is retired.
func TestLedgerRestoreAtomicRevisionDrift(t *testing.T) {
	ledger, root := newTestLedger(t)
	first := filepath.Join(root, "projects", "alpha", "tasks.md")
	second := filepath.Join(root, "projects", "beta", "tasks.md")
	writeFile(t, first, "alpha after")
	writeFile(t, second, "beta after")
	recordLedgerRestoreGroup(t, ledger, "revision-drift", "grouped edit", []LedgerArtifact{
		{Path: first, BeforeExists: true, Before: []byte("alpha before"), AfterExists: true, After: []byte("alpha after")},
		{Path: second, BeforeExists: true, Before: []byte("beta before"), AfterExists: true, After: []byte("beta after")},
	})
	writeFile(t, second, "concurrent edit")

	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("Undo error = %v, want ErrStaleArtifact", err)
	}
	assertLedgerRestoreContent(t, first, "alpha after")
	assertLedgerRestoreContent(t, second, "concurrent edit")
	// No ledger entry explains the drift, so the entry is retired: it moves to
	// neither stack, and no artifact was written on the way.
	assertLedgerRestoreStacks(t, ledger, 0, 0)
	assertJournalClean(t, root)
}

// TestLedgerRestoreCrashRecoveryConvergesAtEveryOperation injects a process stop
// after each operation prefix of the production restore manifest. Recovery must
// converge edited and created artifacts plus ledger stack movement together.
func TestLedgerRestoreCrashRecoveryConvergesAtEveryOperation(t *testing.T) {
	prototype := prepareLedgerRestoreCrashFixture(t)
	opCount := len(prototype.manifest.Ops)
	if opCount != 3 {
		t.Fatalf("restore manifest ops = %d, want two artifacts plus ledger.json", opCount)
	}

	for prefix := 0; prefix <= opCount; prefix++ {
		t.Run(fmt.Sprintf("after-op-%02d", prefix), func(t *testing.T) {
			fixture := prepareLedgerRestoreCrashFixture(t)
			if prefix > 0 {
				fixture.manifest.State = transaction.StateCommitting
				writeLedgerRestoreManifest(t, fixture)
				applyLedgerRestoreOpPrefix(t, fixture.manifest, prefix)
			}

			report, err := transaction.NewJournal(fixture.transactionsDir, []string{fixture.root}, "").Recover()
			if err != nil {
				t.Fatalf("Recover: %v", err)
			}
			if report.Blocked() {
				t.Fatalf("Recover quarantined restore: %+v", report.Quarantined)
			}
			assertLedgerRestoreContent(t, fixture.editedPath, "edited before")
			if _, err := os.Stat(fixture.createdPath); !os.IsNotExist(err) {
				t.Fatalf("created artifact survived undo recovery: %v", err)
			}
			assertLedgerRestoreStacks(t, fixture.ledger, 0, 1)
			assertJournalClean(t, fixture.root)
		})
	}
}

// TestUndoConcurrentCommitSerializesLedgerRestore races a revision-checked user
// commit with undo. The shared workspace lock permits only the two serializable
// outcomes: commit-then-undo, or undo followed by a stale commit rejection.
func TestUndoConcurrentCommitSerializesLedgerRestore(t *testing.T) {
	root := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(root, ".docket", "undo"), []string{root})
	service := NewCommitServiceWithLedger(filepath.Join(root, ".docket", "transactions"), []string{root}, ledger)
	path := filepath.Join(root, "notes", "race.md")
	writeFile(t, path, "before")
	beforeRevision := RawArtifactRevision([]byte("before"))
	if _, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("after"), ExpectedRevision: &beforeRevision}},
		UndoAction: "first commit",
	}); err != nil {
		t.Fatalf("seed commit: %v", err)
	}

	afterRevision := RawArtifactRevision([]byte("after"))
	start := make(chan struct{})
	commitDone := make(chan error, 1)
	type restoreResult struct {
		entry  LedgerEntry
		action string
		err    error
	}
	undoDone := make(chan restoreResult, 1)
	go func() {
		<-start
		_, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
			Artifacts:  []ArtifactChange{{Path: path, Data: []byte("concurrent"), ExpectedRevision: &afterRevision}},
			UndoAction: "second commit",
		})
		commitDone <- err
	}()
	go func() {
		<-start
		entry, action, err := ledger.UndoWithEntry(GlobalScope())
		undoDone <- restoreResult{entry: entry, action: action, err: err}
	}()
	close(start)
	commitErr := <-commitDone
	undo := <-undoDone
	if undo.err != nil {
		t.Fatalf("concurrent Undo: %v", undo.err)
	}

	if commitErr == nil {
		if undo.action != "second commit" {
			t.Fatalf("commit-first undo action = %q, want second commit", undo.action)
		}
		assertLedgerRestoreContent(t, path, "after")
		assertLedgerRestoreStacks(t, ledger, 1, 1)
		return
	}
	if !errors.Is(commitErr, ErrStaleArtifact) {
		t.Fatalf("concurrent commit error = %v, want nil or ErrStaleArtifact", commitErr)
	}
	if undo.action != "first commit" {
		t.Fatalf("undo-first action = %q, want first commit", undo.action)
	}
	assertLedgerRestoreContent(t, path, "before")
	assertLedgerRestoreStacks(t, ledger, 0, 1)
}

// TestUndoRedoConcurrentCallersSerializeStackMovement starts global undo and
// redo together after a forward record invalidated the old redo branch. The
// result must match one of the two legal serial orders, with the reachable entry
// neither duplicated nor lost.
func TestUndoRedoConcurrentCallersSerializeStackMovement(t *testing.T) {
	ledger, root := newTestLedger(t)
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.md")
	writeFile(t, first, "one after")
	recordLedgerRestoreGroup(t, ledger, "txn-one", "first", []LedgerArtifact{{
		Path: first, BeforeExists: true, Before: []byte("one before"), AfterExists: true, After: []byte("one after"),
	}})
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("seed Undo: %v", err)
	}
	writeFile(t, second, "two after")
	recordLedgerRestoreGroup(t, ledger, "txn-two", "second", []LedgerArtifact{{
		Path: second, BeforeExists: true, Before: []byte("two before"), AfterExists: true, After: []byte("two after"),
	}})
	assertLedgerRestoreStacks(t, ledger, 1, 0)

	type concurrentRestore struct {
		action string
		err    error
	}
	start := make(chan struct{})
	undoDone := make(chan concurrentRestore, 1)
	redoDone := make(chan concurrentRestore, 1)
	go func() {
		<-start
		action, err := ledger.Undo(GlobalScope())
		undoDone <- concurrentRestore{action: action, err: err}
	}()
	go func() {
		<-start
		action, err := ledger.Redo(GlobalScope())
		redoDone <- concurrentRestore{action: action, err: err}
	}()
	close(start)
	undo := <-undoDone
	redo := <-redoDone
	if undo.err != nil || undo.action != "second" {
		t.Fatalf("concurrent undo action = %q, want second", undo.action)
	}
	switch {
	case redo.err == nil:
		if redo.action != "second" {
			t.Fatalf("concurrent redo action = %q, want second", redo.action)
		}
		assertLedgerRestoreContent(t, second, "two after")
		assertLedgerRestoreStacks(t, ledger, 1, 0)
	case redo.action == "" && redo.err != nil:
		assertLedgerRestoreContent(t, second, "two before")
		assertLedgerRestoreStacks(t, ledger, 0, 1)
	default:
		t.Fatalf("concurrent redo = action %q err %v, want txn-two or unavailable", redo.action, redo.err)
	}
	assertLedgerRestoreContent(t, first, "one before")
	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	seen := map[string]bool{}
	for _, entry := range append(append([]LedgerEntry(nil), lf.Entries...), lf.RedoEntries...) {
		if seen[entry.TransactionID] {
			t.Fatalf("ledger entry %q duplicated across stacks", entry.TransactionID)
		}
		seen[entry.TransactionID] = true
	}
	if seen["txn-one"] || !seen["txn-two"] {
		t.Fatalf("ledger entries after concurrent restores = %v", seen)
	}
}

type ledgerRestoreCrashFixture struct {
	root            string
	transactionsDir string
	ledger          *WorkspaceLedger
	manifest        *transaction.Manifest
	editedPath      string
	createdPath     string
}

func prepareLedgerRestoreCrashFixture(t *testing.T) ledgerRestoreCrashFixture {
	t.Helper()
	root := t.TempDir()
	transactionsDir := filepath.Join(root, ".docket", "transactions")
	ledger := NewWorkspaceLedger(filepath.Join(root, ".docket", "undo"), []string{root})
	editedPath := filepath.Join(root, "projects", "alpha", "tasks.md")
	createdPath := filepath.Join(root, "notes", "created.md")
	writeFile(t, editedPath, "edited after")
	writeFile(t, createdPath, "created after")
	recordLedgerRestoreGroup(t, ledger, "restore-crash", "grouped restore", []LedgerArtifact{
		{Path: editedPath, BeforeExists: true, Before: []byte("edited before"), AfterExists: true, After: []byte("edited after")},
		{Path: createdPath, BeforeExists: false, AfterExists: true, After: []byte("created after")},
	})
	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	entry, idx, ok := selectNewestOwned(lf.Entries, GlobalScope(), "")
	if !ok {
		t.Fatal("restore crash fixture has no undo entry")
	}
	journal := transaction.NewJournal(transactionsDir, []string{root}, "")
	manifest, err := ledger.prepareRestoreTransaction(journal, lf, entry, idx, false)
	if err != nil {
		t.Fatalf("prepare restore transaction: %v", err)
	}
	if manifest.Ops[len(manifest.Ops)-1].Target != ledger.ledgerPath() {
		t.Fatalf("last restore op = %q, want ledger.json", manifest.Ops[len(manifest.Ops)-1].Target)
	}
	return ledgerRestoreCrashFixture{
		root: root, transactionsDir: transactionsDir, ledger: ledger, manifest: manifest,
		editedPath: editedPath, createdPath: createdPath,
	}
}

func writeLedgerRestoreManifest(t *testing.T, fixture ledgerRestoreCrashFixture) {
	t.Helper()
	data, err := json.MarshalIndent(fixture.manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal restore manifest: %v", err)
	}
	path := filepath.Join(fixture.transactionsDir, fixture.manifest.ID+".json")
	if err := os.WriteFile(path, data, FilePermissions); err != nil {
		t.Fatalf("write restore manifest: %v", err)
	}
}

func applyLedgerRestoreOpPrefix(t *testing.T, manifest *transaction.Manifest, prefix int) {
	t.Helper()
	for i := 0; i < prefix; i++ {
		op := manifest.Ops[i]
		switch op.Kind {
		case transaction.OpWrite:
			if err := os.Rename(op.Staged, op.Target); err != nil {
				t.Fatalf("apply restore write %d: %v", i, err)
			}
		case transaction.OpDelete:
			if err := os.Remove(op.Target); err != nil && !os.IsNotExist(err) {
				t.Fatalf("apply restore delete %d: %v", i, err)
			}
		default:
			t.Fatalf("restore op %d has unsupported kind %q", i, op.Kind)
		}
	}
}

func recordLedgerRestoreGroup(t *testing.T, ledger *WorkspaceLedger, transactionID, action string, artifacts []LedgerArtifact) {
	t.Helper()
	if err := ledger.Record(LedgerRecord{TransactionID: transactionID, Action: action, Artifacts: artifacts}); err != nil {
		t.Fatalf("Record(%s): %v", transactionID, err)
	}
}

func assertLedgerRestoreContent(t *testing.T, path, want string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(data) != want {
		t.Fatalf("%s content = %q, want %q", path, data, want)
	}
}

func assertLedgerRestoreStacks(t *testing.T, ledger *WorkspaceLedger, undo, redo int) {
	t.Helper()
	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load ledger: %v", err)
	}
	if len(lf.Entries) != undo || len(lf.RedoEntries) != redo {
		t.Fatalf("ledger stack sizes = undo %d redo %d, want undo %d redo %d", len(lf.Entries), len(lf.RedoEntries), undo, redo)
	}
}

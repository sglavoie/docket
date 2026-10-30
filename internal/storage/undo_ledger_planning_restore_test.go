package storage

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/sglavoie/docket/internal/transaction"
)

// The configuration package tests the real planning codec. This injected
// domain callback checks storage's locking/preflight/journal integration without
// making storage depend on config (which already depends on storage).
func installTestPlanningRestore(t *testing.T, ledger *WorkspaceLedger, path string) {
	t.Helper()
	ledger.WithPlanningRestore(path, func(expected, desired, current []byte) ([]byte, bool) {
		if !bytes.Equal(current, append(bytes.Clone(expected), []byte(" + peer")...)) {
			return nil, false
		}
		return append(bytes.Clone(desired), []byte(" + peer")...), true
	})
}

func TestPlanningRestoreCrashRecoveryConverges(t *testing.T) {
	for prefix := 0; prefix <= 3; prefix++ {
		t.Run(fmt.Sprintf("after_op_%d", prefix), func(t *testing.T) {
			ledger, root := newTestLedger(t)
			project := filepath.Join(root, "projects", "alpha", "tasks.md")
			planning := filepath.Join(root, ".docket", "planning.json")
			before, after := taskRestoreDocuments()
			recordLedgerRestoreGroup(t, ledger, "planning-and-task", "plan task", []LedgerArtifact{
				{Path: project, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)), AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))},
				{Path: planning, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
			})
			current := after.clone()
			current.Tasks[1].Title = "Incoming B"
			writeFile(t, project, mustWriteCurrentDocument(t, current))
			writeFile(t, planning, "after + peer")
			installTestPlanningRestore(t, ledger, planning)
			lf, err := ledger.load()
			if err != nil {
				t.Fatal(err)
			}
			entry, index, ok := selectNewestOwned(lf.Entries, GlobalScope(), "")
			if !ok {
				t.Fatal("no undo entry")
			}
			manifest, err := ledger.prepareRestoreTransaction(transaction.NewJournal(ledger.transactionsDir, ledger.roots, ""), lf, entry, index, false, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(manifest.Ops) != 3 {
				t.Fatalf("restore has %d operations, want task, planning and ledger", len(manifest.Ops))
			}
			fixture := ledgerRestoreCrashFixture{root: root, ledger: ledger, transactionsDir: ledger.transactionsDir, manifest: manifest}
			if prefix > 0 {
				manifest.State = transaction.StateCommitting
			}
			writeLedgerRestoreManifest(t, fixture)
			applyLedgerRestoreOpPrefix(t, manifest, prefix)
			if _, err := NewCommitService(ledger.transactionsDir, ledger.roots).RecoverWorkspace(); err != nil {
				t.Fatal(err)
			}
			current.Tasks[0] = before.Tasks[0]
			assertLedgerRestoreContent(t, project, mustWriteCurrentDocument(t, current))
			assertLedgerRestoreContent(t, planning, "before + peer")
			assertLedgerRestoreStacks(t, ledger, 0, 1)
			assertJournalClean(t, root)
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatal(err)
			}
			current.Tasks[0] = after.Tasks[0]
			assertLedgerRestoreContent(t, project, mustWriteCurrentDocument(t, current))
			assertLedgerRestoreContent(t, planning, "after + peer")
			assertLedgerRestoreStacks(t, ledger, 1, 0)
		})
	}
}

func TestPlanningRestorePreflightsOtherArtifacts(t *testing.T) {
	ledger, root := newTestLedger(t)
	planning := filepath.Join(root, ".docket", "planning.json")
	project := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	installTestPlanningRestore(t, ledger, planning)
	recordLedgerRestoreGroup(t, ledger, "planning-and-task", "edit", []LedgerArtifact{
		{Path: planning, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
		{Path: project, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)), AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))},
	})
	writeFile(t, planning, "after + peer")
	current := after.clone()
	current.Tasks[0].Title = "Competing A"
	content := mustWriteCurrentDocument(t, current)
	writeFile(t, project, content)
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("Undo = %v, want stale artifact", err)
	}
	assertLedgerRestoreContent(t, planning, "after + peer")
	assertLedgerRestoreContent(t, project, content)
	assertJournalClean(t, root)
}

func TestPlanningRestoreCannotPassNewerLocalPlanning(t *testing.T) {
	ledger, root := newTestLedger(t)
	planning := filepath.Join(root, ".docket", "planning.json")
	project := filepath.Join(root, "projects", "alpha", "tasks.md")
	before, after := taskRestoreDocuments()
	installTestPlanningRestore(t, ledger, planning)
	recordLedgerRestoreGroup(t, ledger, "older", "older", []LedgerArtifact{
		{Path: planning, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
		{Path: project, BeforeExists: true, Before: []byte(mustWriteCurrentDocument(t, before)), AfterExists: true, After: []byte(mustWriteCurrentDocument(t, after))},
	})
	recordLedgerRestoreGroup(t, ledger, "newer", "newer", []LedgerArtifact{
		{Path: planning, BeforeExists: true, Before: []byte("after"), AfterExists: true, After: []byte("after + peer")},
	})
	writeFile(t, planning, "after + peer")
	writeFile(t, project, mustWriteCurrentDocument(t, after))
	if _, err := ledger.Undo(LedgerScope{ArtifactRoot: project}); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("scoped undo = %v, want stale artifact", err)
	}
	assertLedgerRestoreContent(t, planning, "after + peer")
	assertLedgerRestoreContent(t, project, mustWriteCurrentDocument(t, after))
	assertLedgerRestoreStacks(t, ledger, 2, 0)
}

func TestPlanningRestoreAppliesOnlyToRegisteredPath(t *testing.T) {
	ledger, root := newTestLedger(t)
	planning := filepath.Join(root, ".docket", "planning.json")
	note := filepath.Join(root, "projects", "alpha", "notes", "planning.json")
	installTestPlanningRestore(t, ledger, planning)
	recordLedgerRestoreGroup(t, ledger, "note", "edit note", []LedgerArtifact{
		{Path: note, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
	})
	writeFile(t, note, "after + peer")
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("note undo = %v, want stale artifact", err)
	}
	assertLedgerRestoreContent(t, note, "after + peer")
}

func TestPlanningRestoreGuardDoesNotRecreateMissingProject(t *testing.T) {
	ledger, root := newTestLedger(t)
	planning := filepath.Join(root, ".docket", "planning.json")
	missing := filepath.Join(root, "projects", "gone")
	installTestPlanningRestore(t, ledger, planning)
	ledger.WithPlanningRestoreGuard(func(_, _ []byte) ([]string, func([]transaction.Op) error, error) {
		return []string{filepath.Join(missing, "tasks.md")}, func(_ []transaction.Op) error {
			t.Fatal("validator ran after its project disappeared")
			return nil
		}, nil
	})
	recordLedgerRestoreGroup(t, ledger, "planning", "plan task", []LedgerArtifact{
		{Path: planning, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after")},
	})
	writeFile(t, planning, "after + peer")
	if _, err := ledger.Undo(GlobalScope()); !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("missing-project undo = %v, want stale artifact", err)
	}
	requireAbsent(t, missing)
	assertLedgerRestoreContent(t, planning, "after + peer")
	assertLedgerRestoreStacks(t, ledger, 0, 0)
	assertJournalClean(t, root)
}

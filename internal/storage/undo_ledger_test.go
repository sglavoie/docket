package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

const testWorkspaceLedgerDepth = 1000

// newTestLedger builds a WorkspaceLedger rooted under a fresh temp workspace,
// returning the ledger and the workspace root so tests can seed files inside it.
func newTestLedger(t *testing.T, maxDepth int) (*WorkspaceLedger, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".tt", "undo")
	return NewWorkspaceLedger(dir, maxDepth, []string{root}), root
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func requireAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s to be absent, stat err=%v", path, err)
	}
}

// TestWorkspaceLedger_DisabledRecordCreatesNoState verifies both planning and
// standalone recording are successful no-ops without ledger or journal state.
func TestWorkspaceLedger_DisabledRecordCreatesNoState(t *testing.T) {
	ledger, root := newTestLedger(t, 0)
	record := LedgerRecord{
		TransactionID: "disabled",
		Artifacts: []LedgerArtifact{{
			Path: filepath.Join(root, "notes", "disabled.md"), AfterExists: true, After: []byte("after"),
		}},
	}

	plan, err := ledger.prepareRecord(record)
	if err != nil {
		t.Fatalf("prepareRecord: %v", err)
	}
	if plan != nil {
		t.Fatalf("prepareRecord plan = %#v, want nil while disabled", plan)
	}
	if err := ledger.Record(record); err != nil {
		t.Fatalf("Record: %v", err)
	}
	requireAbsent(t, ledger.dir)
	requireAbsent(t, ledger.transactionsDir)
}

// TestCommitService_DisabledLedgerCommitsUserArtifacts proves a configured but
// disabled ledger does not prevent the user transaction from committing.
func TestCommitService_DisabledLedgerCommitsUserArtifacts(t *testing.T) {
	dataDir := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), 0, []string{dataDir})
	service := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)
	path := filepath.Join(dataDir, "notes", "global.md")

	txnID, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: path, Data: []byte("committed")}},
		UndoAction: "edit global note",
	})
	if err != nil {
		t.Fatalf("CommitArtifactTransaction: %v", err)
	}
	if txnID == "" {
		t.Fatal("CommitArtifactTransaction returned an empty transaction id")
	}
	if got := readFile(t, path); got != "committed" {
		t.Fatalf("committed artifact = %q, want committed", got)
	}
	if _, ok, err := ledger.SelectNewestUndo(GlobalScope()); err != nil || ok {
		t.Fatalf("SelectNewestUndo = ok %v err %v, want no history", ok, err)
	}
	requireAbsent(t, ledger.dir)
	assertJournalClean(t, dataDir)
}

// TestWorkspaceLedger_DisablingPreservesExistingHistory covers both reopening
// at depth zero and changing a live ledger to zero. Neither path may hide or
// mutate the existing undo and redo stacks, and later records remain disabled.
func TestWorkspaceLedger_DisablingPreservesExistingHistory(t *testing.T) {
	for _, mode := range []string{"reopen", "set"} {
		t.Run(mode, func(t *testing.T) {
			ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
			first := filepath.Join(root, "first.md")
			second := filepath.Join(root, "second.md")
			later := filepath.Join(root, "later.md")
			writeFile(t, first, "first after")
			writeFile(t, second, "second after")
			mustRecord(t, ledger, "txn-first", "p", first, "first before", "first after")
			mustRecord(t, ledger, "txn-second", "p", second, "second before", "second after")
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatalf("seed Undo: %v", err)
			}

			switch mode {
			case "reopen":
				ledger = NewWorkspaceLedger(ledger.dir, 0, []string{root})
			case "set":
				ledger.SetMaxDepth(0)
			}
			writeFile(t, later, "later after")
			mustRecord(t, ledger, "txn-disabled", "p", later, "later before", "later after")

			undoCount, err := ledger.UndoCount(GlobalScope())
			if err != nil {
				t.Fatalf("UndoCount: %v", err)
			}
			redoCount, err := ledger.RedoCount(GlobalScope())
			if err != nil {
				t.Fatalf("RedoCount: %v", err)
			}
			if undoCount != 1 || redoCount != 1 {
				t.Fatalf("history counts = undo %d redo %d, want 1 and 1", undoCount, redoCount)
			}
			if _, err := ledger.Undo(GlobalScope()); err != nil {
				t.Fatalf("Undo existing entry: %v", err)
			}
			if got := readFile(t, first); got != "first before" {
				t.Fatalf("first after undo = %q, want first before", got)
			}
			if _, err := ledger.Redo(GlobalScope()); err != nil {
				t.Fatalf("Redo existing entry: %v", err)
			}
			if got := readFile(t, second); got != "second after" {
				t.Fatalf("second after redo = %q, want second after", got)
			}
			if got := readFile(t, later); got != "later after" {
				t.Fatalf("unrecorded mutation changed during restore: got %q", got)
			}
		})
	}
}

// TestWorkspaceLedger_SetMaxDepthPrunesOnNextRecord verifies reducing a
// positive depth leaves existing history intact until the next recorded commit.
func TestWorkspaceLedger_SetMaxDepthPrunesOnNextRecord(t *testing.T) {
	ledger, root := newTestLedger(t, 3)
	for i := 0; i < 3; i++ {
		path := filepath.Join(root, fmt.Sprintf("entry-%d.md", i))
		writeFile(t, path, "after")
		mustRecord(t, ledger, fmt.Sprintf("txn-%d", i), "p", path, fmt.Sprintf("before-%d", i), "after")
	}

	ledger.SetMaxDepth(1)
	if count, err := ledger.UndoCount(GlobalScope()); err != nil || count != 3 {
		t.Fatalf("UndoCount before next record = %d err %v, want 3", count, err)
	}
	last := filepath.Join(root, "entry-last.md")
	writeFile(t, last, "after")
	mustRecord(t, ledger, "txn-last", "p", last, "before-last", "after")

	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo: ok=%v err=%v", ok, err)
	}
	if entry.TransactionID != "txn-last" {
		t.Fatalf("remaining transaction = %q, want txn-last", entry.TransactionID)
	}
	if count, err := ledger.UndoCount(GlobalScope()); err != nil || count != 1 {
		t.Fatalf("UndoCount after next record = %d err %v, want 1", count, err)
	}
	requireAbsent(t, blobPathFor(ledger, "before-0"))
}

// TestWorkspaceLedger_SetMaxDepthConcurrentWithRecord exercises live retention
// updates against record planning. The race detector verifies the shared depth
// has no unsynchronized reads or writes.
func TestWorkspaceLedger_SetMaxDepthConcurrentWithRecord(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	path := filepath.Join(root, "race.md")
	writeFile(t, path, "after")
	mustRecord(t, ledger, "txn-seed", "p", path, "before", "after")

	const iterations = 32
	start := make(chan struct{})
	errs := make(chan error, iterations)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			if i%2 == 0 {
				ledger.SetMaxDepth(0)
			} else {
				ledger.SetMaxDepth(testWorkspaceLedgerDepth)
			}
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < iterations; i++ {
			err := ledger.Record(LedgerRecord{
				TransactionID: fmt.Sprintf("txn-race-%d", i),
				ProjectIDs:    []string{"p"},
				Artifacts: []LedgerArtifact{{
					Path: path, BeforeExists: true, Before: []byte("before"), AfterExists: true, After: []byte("after"),
				}},
			})
			if err != nil {
				errs <- err
			}
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Record: %v", err)
	}

	ledger.SetMaxDepth(2)
	mustRecord(t, ledger, "txn-final", "p", path, "before", "after")
	if count, err := ledger.UndoCount(GlobalScope()); err != nil || count != 2 {
		t.Fatalf("UndoCount after final record = %d err %v, want 2", count, err)
	}
}

// TestWorkspaceLedger_UndoRedoRestoresImages verifies a single entry's undo
// restores the before-image and redo restores the after-image.
func TestWorkspaceLedger_UndoRedoRestoresImages(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	f := filepath.Join(root, "projects", "alpha", "tasks.md")
	writeFile(t, f, "v2") // post-commit on-disk state

	if err := ledger.Record(LedgerRecord{
		TransactionID: "txn-1",
		Action:        "edit",
		Context:       "alpha",
		ProjectIDs:    []string{"alpha"},
		Artifacts: []LedgerArtifact{{
			Path: f, BeforeExists: true, Before: []byte("v1"), AfterExists: true, After: []byte("v2"),
		}},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	desc, err := ledger.Undo(GlobalScope())
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if desc != "edit: alpha" {
		t.Fatalf("undo description = %q, want %q", desc, "edit: alpha")
	}
	if got := readFile(t, f); got != "v1" {
		t.Fatalf("after undo = %q, want v1", got)
	}

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := readFile(t, f); got != "v2" {
		t.Fatalf("after redo = %q, want v2", got)
	}

	// A second undo must again be available (entry is back on the undo stack).
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("second Undo: %v", err)
	}
	if got := readFile(t, f); got != "v1" {
		t.Fatalf("after second undo = %q, want v1", got)
	}
}

// TestWorkspaceLedger_UndoCreateAndDeleteImages covers the absent-state ends: an
// undo that removes a created file and a redo that recreates it.
func TestWorkspaceLedger_UndoCreateAndDeleteImages(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	f := filepath.Join(root, "projects", "alpha", "new.md")
	writeFile(t, f, "created") // file was created by the commit

	if err := ledger.Record(LedgerRecord{
		TransactionID: "txn-1",
		ProjectIDs:    []string{"alpha"},
		Artifacts:     []LedgerArtifact{{Path: f, AfterExists: true, After: []byte("created")}},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	requireAbsent(t, f)

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := readFile(t, f); got != "created" {
		t.Fatalf("after redo = %q, want created", got)
	}
}

// TestWorkspaceLedger_ProjectScopeOutOfOrder verifies current-project undo selects
// the newest entry affecting that project even when a later entry for a different
// project sits above it on the stack.
func TestWorkspaceLedger_ProjectScopeOutOfOrder(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	f1 := filepath.Join(root, "projects", "p1", "tasks.md")
	f2 := filepath.Join(root, "projects", "p2", "tasks.md")
	writeFile(t, f1, "b")
	writeFile(t, f2, "y")

	mustRecord(t, ledger, "txn-1", "p1", f1, "a", "b")
	mustRecord(t, ledger, "txn-2", "p2", f2, "x", "y")

	// Newest-for-p1 is the older entry (txn-1); undoing it must not touch p2.
	if _, err := ledger.Undo(ProjectScope("p1")); err != nil {
		t.Fatalf("Undo p1: %v", err)
	}
	if got := readFile(t, f1); got != "a" {
		t.Fatalf("f1 after undo = %q, want a", got)
	}
	if got := readFile(t, f2); got != "y" {
		t.Fatalf("f2 must be untouched, got %q, want y", got)
	}

	// Global undo now reverses the newest remaining entry (txn-2, p2).
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo global: %v", err)
	}
	if got := readFile(t, f2); got != "x" {
		t.Fatalf("f2 after global undo = %q, want x", got)
	}

	// Redo p1 replays txn-1.
	if _, err := ledger.Redo(ProjectScope("p1")); err != nil {
		t.Fatalf("Redo p1: %v", err)
	}
	if got := readFile(t, f1); got != "b" {
		t.Fatalf("f1 after redo = %q, want b", got)
	}
}

func TestLedgerScope_ArtifactRootMatchesAllArtifactTransitions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "notes")
	sibling := root + "-other"
	scope := ArtifactRootScope(filepath.Join(root, "."))
	tests := []struct {
		name      string
		artifacts []ArtifactImage
		want      bool
	}{
		{name: "create", artifacts: []ArtifactImage{{Path: filepath.Join(root, "created.md"), AfterExists: true}}, want: true},
		{name: "update", artifacts: []ArtifactImage{{Path: filepath.Join(root, "updated.md"), BeforeExists: true, AfterExists: true}}, want: true},
		{name: "delete", artifacts: []ArtifactImage{{Path: filepath.Join(root, "deleted.md"), BeforeExists: true}}, want: true},
		{name: "rename", artifacts: []ArtifactImage{
			{Path: filepath.Join(root, "old.md"), BeforeExists: true},
			{Path: filepath.Join(root, "new.md"), AfterExists: true},
		}, want: true},
		{name: "move out", artifacts: []ArtifactImage{
			{Path: filepath.Join(root, "moved.md"), BeforeExists: true},
			{Path: filepath.Join(sibling, "moved.md"), AfterExists: true},
		}, want: true},
		{name: "move in", artifacts: []ArtifactImage{
			{Path: filepath.Join(sibling, "incoming.md"), BeforeExists: true},
			{Path: filepath.Join(root, "nested", "..", "incoming.md"), AfterExists: true},
		}, want: true},
		{name: "similarly prefixed sibling", artifacts: []ArtifactImage{{Path: filepath.Join(sibling, "unrelated.md"), AfterExists: true}}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := LedgerEntry{ProjectIDs: []string{"p1"}, Artifacts: tt.artifacts}
			if got := entryMatchesScope(entry, scope); got != tt.want {
				t.Fatalf("entryMatchesScope() = %v, want %v", got, tt.want)
			}
			combined := scope
			combined.ProjectID = "p1"
			if got := entryMatchesScope(entry, combined); got != tt.want {
				t.Fatalf("entryMatchesScope() with matching project = %v, want %v", got, tt.want)
			}
			combined.ProjectID = "p2"
			if entryMatchesScope(entry, combined) {
				t.Fatal("project and artifact-root restrictions did not combine with AND semantics")
			}
		})
	}
}

// TestWorkspaceLedger_ArtifactRootScopeOutOfOrder verifies selection, undo and
// redo counts all skip a newer unrelated mutation using the same root matcher.
func TestWorkspaceLedger_ArtifactRootScopeOutOfOrder(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	notesRoot := filepath.Join(root, "notes")
	notePath := filepath.Join(notesRoot, "note.md")
	taskPath := filepath.Join(root, "projects", "p1", "tasks.md")
	writeFile(t, notePath, "note after")
	writeFile(t, taskPath, "task after")

	if err := ledger.Record(LedgerRecord{
		TransactionID: "note-txn",
		Action:        "update note",
		Artifacts: []LedgerArtifact{{
			Path: notePath, BeforeExists: true, Before: []byte("note before"), AfterExists: true, After: []byte("note after"),
		}},
	}); err != nil {
		t.Fatalf("Record note: %v", err)
	}
	mustRecord(t, ledger, "task-txn", "p1", taskPath, "task before", "task after")

	scope := ArtifactRootScope(notesRoot)
	if count, err := ledger.UndoCount(scope); err != nil || count != 1 {
		t.Fatalf("UndoCount(notes) = %d, err=%v, want 1", count, err)
	}
	entry, ok, err := ledger.SelectNewestUndo(scope)
	if err != nil || !ok || entry.TransactionID != "note-txn" {
		t.Fatalf("SelectNewestUndo(notes) = %#v, ok=%v, err=%v", entry, ok, err)
	}
	if _, undoErr := ledger.Undo(scope); undoErr != nil {
		t.Fatalf("Undo(notes): %v", undoErr)
	}
	if got := readFile(t, notePath); got != "note before" {
		t.Fatalf("note after undo = %q, want note before", got)
	}
	if got := readFile(t, taskPath); got != "task after" {
		t.Fatalf("unrelated task after note undo = %q, want task after", got)
	}
	if count, countErr := ledger.RedoCount(scope); countErr != nil || count != 1 {
		t.Fatalf("RedoCount(notes) = %d, err=%v, want 1", count, countErr)
	}
	redo, ok, err := ledger.SelectNewestRedo(scope)
	if err != nil || !ok || redo.TransactionID != "note-txn" {
		t.Fatalf("SelectNewestRedo(notes) = %#v, ok=%v, err=%v", redo, ok, err)
	}
	if _, redoErr := ledger.Redo(scope); redoErr != nil {
		t.Fatalf("Redo(notes): %v", redoErr)
	}
	if got := readFile(t, notePath); got != "note after" {
		t.Fatalf("note after redo = %q, want note after", got)
	}
}

// TestWorkspaceLedger_GroupedEntryRestoresAllProjects verifies a single entry
// spanning multiple projects reverses every project's file at once and is
// selectable by any member project.
func TestWorkspaceLedger_GroupedEntryRestoresAllProjects(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	f1 := filepath.Join(root, "projects", "p1", "tasks.md")
	f2 := filepath.Join(root, "projects", "p2", "tasks.md")
	writeFile(t, f1, "b")
	writeFile(t, f2, "y")

	if err := ledger.Record(LedgerRecord{
		TransactionID: "txn-1",
		ProjectIDs:    []string{"p1", "p2"},
		Artifacts: []LedgerArtifact{
			{Path: f1, BeforeExists: true, Before: []byte("a"), AfterExists: true, After: []byte("b")},
			{Path: f2, BeforeExists: true, Before: []byte("x"), AfterExists: true, After: []byte("y")},
		},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Selectable via either member project.
	for _, pid := range []string{"p1", "p2"} {
		if _, ok, err := ledger.SelectNewestUndo(ProjectScope(pid)); err != nil || !ok {
			t.Fatalf("SelectNewestUndo(%s) ok=%v err=%v", pid, ok, err)
		}
	}

	if _, err := ledger.Undo(ProjectScope("p2")); err != nil {
		t.Fatalf("Undo p2: %v", err)
	}
	if got := readFile(t, f1); got != "a" {
		t.Fatalf("f1 after grouped undo = %q, want a", got)
	}
	if got := readFile(t, f2); got != "x" {
		t.Fatalf("f2 after grouped undo = %q, want x", got)
	}
}

// TestWorkspaceLedger_RetentionPruning verifies the undo stack keeps only the
// newest maxDepth entries and garbage-collects the pruned entry's blobs.
func TestWorkspaceLedger_RetentionPruning(t *testing.T) {
	ledger, root := newTestLedger(t, 2)
	files := []string{
		filepath.Join(root, "projects", "p", "a.md"),
		filepath.Join(root, "projects", "p", "b.md"),
		filepath.Join(root, "projects", "p", "c.md"),
	}
	for i, f := range files {
		writeFile(t, f, "after")
		mustRecord(t, ledger, "txn", "p", f, "before"+string(rune('0'+i)), "after")
	}

	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(lf.Entries) != 2 {
		t.Fatalf("undo stack depth = %d, want 2", len(lf.Entries))
	}
	// The oldest entry (a.md) was pruned; undoing twice must not reach it.
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("undo 1: %v", err)
	}
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("undo 2: %v", err)
	}
	if got := readFile(t, files[0]); got != "after" {
		t.Fatalf("pruned entry must be unreachable: a.md = %q, want after", got)
	}

	// The pruned entry's unique before-blob must be gone from the blob store.
	prunedBlob := blobPathFor(ledger, "before0")
	requireAbsent(t, prunedBlob)
	// A surviving entry's before-blob must remain.
	if _, err := os.Stat(blobPathFor(ledger, "before2")); err != nil {
		t.Fatalf("surviving blob missing: %v", err)
	}
}

// TestWorkspaceLedger_RedoInvalidationOnOverlap verifies a new mutation that
// touches a file with a pending redo entry discards that redo, while a
// non-overlapping mutation leaves the redo replayable.
func TestWorkspaceLedger_RedoInvalidationOnOverlap(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	f1 := filepath.Join(root, "projects", "p", "f1.md")
	f2 := filepath.Join(root, "projects", "p", "f2.md")

	writeFile(t, f1, "b")
	mustRecord(t, ledger, "txn-1", "p", f1, "a", "b")
	if _, err := ledger.Undo(GlobalScope()); err != nil { // f1 -> a, e1 on redo stack
		t.Fatalf("undo: %v", err)
	}

	// Non-overlapping mutation on f2 leaves the redo of e1 intact.
	writeFile(t, f2, "y")
	mustRecord(t, ledger, "txn-2", "p", f2, "x", "y")
	if _, ok, _ := ledger.SelectNewestRedo(GlobalScope()); !ok {
		t.Fatal("non-overlapping mutation must preserve redo")
	}

	// Overlapping mutation on f1 invalidates the redo of e1.
	writeFile(t, f1, "c")
	mustRecord(t, ledger, "txn-3", "p", f1, "a", "c")
	if _, ok, _ := ledger.SelectNewestRedo(GlobalScope()); ok {
		t.Fatal("overlapping mutation must invalidate redo")
	}
	if _, err := ledger.Redo(GlobalScope()); err == nil {
		t.Fatal("expected redo to be unavailable after invalidation")
	}
}

// TestWorkspaceLedger_RejectsPathOutsideRoots proves a tampered ledger cannot
// escape the approved workspace roots during restore.
func TestWorkspaceLedger_RejectsPathOutsideRoots(t *testing.T) {
	ledger, root := newTestLedger(t, testWorkspaceLedgerDepth)
	outside := filepath.Join(t.TempDir(), "escape.md")
	writeFile(t, outside, "after")
	_ = root

	if err := ledger.Record(LedgerRecord{
		TransactionID: "txn-1",
		Artifacts:     []LedgerArtifact{{Path: outside, BeforeExists: true, Before: []byte("a"), AfterExists: true, After: []byte("after")}},
	}); err != nil {
		t.Fatalf("Record: %v", err)
	}
	if _, err := ledger.Undo(GlobalScope()); err == nil {
		t.Fatal("expected undo to reject a path outside approved roots")
	}
	if got := readFile(t, outside); got != "after" {
		t.Fatalf("outside file must be untouched, got %q", got)
	}
}

// TestCommitService_LedgerRecordsAndUndoesAttachmentDelete exercises the whole
// wired path: a commit that deletes a task's attachments records a ledger entry,
// and undoing that entry restores both the project markdown and the attachment
// bytes captured before the delete.
func TestCommitService_LedgerRecordsAndUndoesAttachmentDelete(t *testing.T) {
	dataDir := t.TempDir()
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), testWorkspaceLedgerDepth, []string{dataDir})
	svc := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger)

	project := newCommitTestProject(t, dataDir, "alpha", "attach me")
	task := project.Tasks[0]
	attachPath := seedTaskAttachment(t, project, task, "note.txt", []byte("attachment bytes"))

	// Commit 1: establish tasks.md on disk (records an entry).
	if _, err := svc.Commit(ChangeSet{Project: project, UndoAction: "create"}); err != nil {
		t.Fatalf("commit 1: %v", err)
	}

	// Commit 2: delete the attachment through the transactional side effect.
	if _, err := svc.Commit(ChangeSet{
		Project:       project,
		UndoAction:    "delete attachment",
		AttachmentOps: []AttachmentOp{{Kind: AttachmentDelete, Task: task, SourceProject: project}},
	}); err != nil {
		t.Fatalf("commit 2: %v", err)
	}
	requireAbsent(t, attachPath)

	// Undo the delete: the attachment bytes captured before the commit come back.
	desc, err := ledger.Undo(GlobalScope())
	if err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if desc != "delete attachment" {
		t.Fatalf("undo description = %q, want %q", desc, "delete attachment")
	}
	if got := readFile(t, attachPath); got != "attachment bytes" {
		t.Fatalf("restored attachment = %q, want %q", got, "attachment bytes")
	}

	// The entry is now selectable for redo (delete again).
	if _, ok, err := ledger.SelectNewestRedo(GlobalScope()); err != nil || !ok {
		t.Fatalf("SelectNewestRedo ok=%v err=%v", ok, err)
	}
}

// TestCommitService_NoLedgerSkipsRecording confirms a service without a ledger
// commits normally and writes no ledger file, preserving pre-ledger behavior.
func TestCommitService_NoLedgerSkipsRecording(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "no history")
	if _, err := svc.Commit(ChangeSet{Project: project}); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	requireAbsent(t, filepath.Join(dataDir, ".tt", "undo", "ledger.json"))
}

// mustRecord records a single-artifact before->after edit and fails on error.
func mustRecord(t *testing.T, ledger *WorkspaceLedger, txn, projectID, path, before, after string) {
	t.Helper()
	if err := ledger.Record(LedgerRecord{
		TransactionID: txn,
		ProjectIDs:    []string{projectID},
		Artifacts: []LedgerArtifact{{
			Path: path, BeforeExists: true, Before: []byte(before), AfterExists: true, After: []byte(after),
		}},
	}); err != nil {
		t.Fatalf("Record(%s): %v", txn, err)
	}
}

// blobPathFor returns the content-addressed blob path for the given content.
func blobPathFor(ledger *WorkspaceLedger, content string) string {
	sum := sha256.Sum256([]byte(content))
	return ledger.blobPath(hex.EncodeToString(sum[:]))
}

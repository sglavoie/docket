package storage

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// newTestLedger builds a WorkspaceLedger rooted under a fresh temp workspace,
// returning the ledger and the workspace root so tests can seed files inside it.
func newTestLedger(t *testing.T) (*WorkspaceLedger, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, ".tt", "undo")
	return NewWorkspaceLedger(dir, []string{root}), root
}

func TestWorkspaceLedgerValidateChecksRetainedBlobsWithoutMutatingHistory(t *testing.T) {
	ledger, root := newTestLedger(t)
	target := filepath.Join(root, "projects", "alpha", "tasks.md")
	mustRecord(t, ledger, "txn-validate", "alpha", target, "before", "after")

	status, err := ledger.Validate()
	if err != nil {
		t.Fatalf("Validate healthy ledger: %v", err)
	}
	if status.UndoEntries != 1 || status.RedoEntries != 0 {
		t.Fatalf("validation status = %+v", status)
	}
	again, err := ledger.Validate()
	if err != nil || again != status {
		t.Fatalf("repeat validation = (%+v, %v), want (%+v, nil)", again, err, status)
	}

	if err := os.Remove(blobPathFor(ledger, "before")); err != nil {
		t.Fatalf("remove retained blob: %v", err)
	}
	if _, err := ledger.Validate(); err == nil {
		t.Fatal("Validate accepted a ledger with a missing retained blob")
	}
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

// TestWorkspaceLedger_UndoRedoRestoresImages verifies a single entry's undo
// restores the before-image and redo restores the after-image.
func TestWorkspaceLedger_UndoRedoRestoresImages(t *testing.T) {
	ledger, root := newTestLedger(t)
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

// TestWorkspaceLedger_RedoReplaysInTheOrderUndone verifies that redo walks back
// up the history it was undone down: two edits to one file, both undone, redo
// oldest first. Replaying the newer one first would find the file at neither of
// its images and refuse, leaving the redo stack stuck.
func TestWorkspaceLedger_RedoReplaysInTheOrderUndone(t *testing.T) {
	ledger, root := newTestLedger(t)
	f := filepath.Join(root, "projects", "alpha", "tasks.md")
	for i, step := range []struct{ before, after string }{{"v1", "v2"}, {"v2", "v3"}} {
		writeFile(t, f, step.after)
		if err := ledger.Record(LedgerRecord{
			TransactionID: fmt.Sprintf("txn-%d", i+1),
			Action:        "edit " + step.after,
			ProjectIDs:    []string{"alpha"},
			Artifacts: []LedgerArtifact{{
				Path: f, BeforeExists: true, Before: []byte(step.before), AfterExists: true, After: []byte(step.after),
			}},
		}); err != nil {
			t.Fatalf("Record %s: %v", step.after, err)
		}
	}

	for _, want := range []string{"v2", "v1"} {
		if _, err := ledger.Undo(GlobalScope()); err != nil {
			t.Fatalf("Undo to %s: %v", want, err)
		}
		if got := readFile(t, f); got != want {
			t.Fatalf("after undo = %q, want %s", got, want)
		}
	}

	next, ok, err := ledger.SelectNewestRedo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestRedo: ok=%v err=%v", ok, err)
	}
	if next.TransactionID != "txn-1" {
		t.Fatalf("next redo = %q, want txn-1", next.TransactionID)
	}
	for _, want := range []string{"v2", "v3"} {
		if _, err := ledger.Redo(GlobalScope()); err != nil {
			t.Fatalf("Redo to %s: %v", want, err)
		}
		if got := readFile(t, f); got != want {
			t.Fatalf("after redo = %q, want %s", got, want)
		}
	}
}

// TestWorkspaceLedger_UndoCreateAndDeleteImages covers the absent-state ends: an
// undo that removes a created file and a redo that recreates it.
func TestWorkspaceLedger_UndoCreateAndDeleteImages(t *testing.T) {
	ledger, root := newTestLedger(t)
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
	ledger, root := newTestLedger(t)
	f1 := filepath.Join(root, "projects", "p1", "tasks.md")
	f2 := filepath.Join(root, "projects", "p2", "tasks.md")
	writeFile(t, f1, "b")
	writeFile(t, f2, "y")

	mustRecord(t, ledger, "txn-1", "p1", f1, "a", "b")
	mustRecord(t, ledger, "txn-2", "p2", f2, "x", "y")

	// Newest-for-p1 is the older entry (txn-1); undoing it must not touch p2.
	if _, err := ledger.Undo(LedgerScope{ProjectID: "p1"}); err != nil {
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
	if _, err := ledger.Redo(LedgerScope{ProjectID: "p1"}); err != nil {
		t.Fatalf("Redo p1: %v", err)
	}
	if got := readFile(t, f1); got != "b" {
		t.Fatalf("f1 after redo = %q, want b", got)
	}
}

func TestLedgerScope_ArtifactRootMatchesAllArtifactTransitions(t *testing.T) {
	root := filepath.Join(t.TempDir(), "notes")
	sibling := root + "-other"
	scope := LedgerScope{ArtifactRoot: filepath.Join(root, ".")}
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
	ledger, root := newTestLedger(t)
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

	scope := LedgerScope{ArtifactRoot: notesRoot}
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

func TestWorkspaceLedgerEmptyHistoryIdentityAcrossScopes(t *testing.T) {
	ledger, root := newTestLedger(t)
	for _, scope := range []LedgerScope{
		GlobalScope(), {ProjectID: "project"}, {ArtifactRoot: root}, {ProjectID: "project", ArtifactRoot: root},
	} {
		if _, err := ledger.Undo(scope); !errors.Is(err, ErrNoUndoHistory) {
			t.Fatalf("Undo(%+v) = %v, want empty undo history", scope, err)
		}
		if _, err := ledger.Redo(scope); !errors.Is(err, ErrNoRedoHistory) {
			t.Fatalf("Redo(%+v) = %v, want empty redo history", scope, err)
		}
	}
	writeFile(t, ledger.ledgerPath(), "invalid JSON")
	if _, err := ledger.Undo(GlobalScope()); err == nil || errors.Is(err, ErrNoUndoHistory) {
		t.Fatalf("corrupt ledger should be a real failure, got %v", err)
	}
}

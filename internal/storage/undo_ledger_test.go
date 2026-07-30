package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
	ledger, root := newTestLedger(t)
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

func TestWorkspaceLedger_GroupedArtifactKindsRestoreTogether(t *testing.T) {
	ledger, root := newTestLedger(t)
	artifacts := []struct {
		name   string
		path   string
		before string
		after  string
	}{
		{
			name:   "tasks",
			path:   filepath.Join(root, "projects", "alpha", "tasks.md"),
			before: "tasks before",
			after:  "tasks after",
		},
		{
			name:   "projects",
			path:   filepath.Join(root, ".tt", "workspace.json"),
			before: `{"projects":["before"]}`,
			after:  `{"projects":["after"]}`,
		},
		{
			name:   "notes",
			path:   filepath.Join(root, "projects", "alpha", "notes", "design.md"),
			before: "note before",
			after:  "note after",
		},
		{
			name:   "attachments",
			path:   filepath.Join(root, "projects", "alpha", "attachments", "task-1", "evidence.txt"),
			before: "attachment before",
			after:  "attachment after",
		},
	}

	record := LedgerRecord{
		TransactionID: "mixed-artifacts",
		Action:        "grouped mutation",
		ProjectIDs:    []string{"alpha"},
		Artifacts:     make([]LedgerArtifact, 0, len(artifacts)),
	}
	for _, artifact := range artifacts {
		writeFile(t, artifact.path, artifact.after)
		record.Artifacts = append(record.Artifacts, LedgerArtifact{
			Path:         artifact.path,
			BeforeExists: true,
			Before:       []byte(artifact.before),
			AfterExists:  true,
			After:        []byte(artifact.after),
		})
	}
	if err := ledger.Record(record); err != nil {
		t.Fatalf("Record: %v", err)
	}

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	for _, artifact := range artifacts {
		if got := readFile(t, artifact.path); got != artifact.before {
			t.Fatalf("%s after undo = %q, want %q", artifact.name, got, artifact.before)
		}
	}

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	for _, artifact := range artifacts {
		if got := readFile(t, artifact.path); got != artifact.after {
			t.Fatalf("%s after redo = %q, want %q", artifact.name, got, artifact.after)
		}
	}
}

// TestWorkspaceLedger_RetentionPruning verifies the undo stack keeps exactly
// the newest WorkspaceUndoLimit entries and garbage-collects the evicted entry's
// blobs deterministically.
func TestWorkspaceLedger_RetentionPruning(t *testing.T) {
	ledger, root := newTestLedger(t)
	files := make([]string, WorkspaceUndoLimit+1)
	for i := range files {
		f := filepath.Join(root, "projects", "p", fmt.Sprintf("%02d.md", i))
		files[i] = f
		writeFile(t, f, "after")
		mustRecord(t, ledger, fmt.Sprintf("txn-%02d", i), "p", f, fmt.Sprintf("before-%02d", i), "after")
	}

	lf, err := ledger.load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(lf.Entries) != WorkspaceUndoLimit {
		t.Fatalf("undo stack depth = %d, want %d", len(lf.Entries), WorkspaceUndoLimit)
	}
	// The oldest entry was pruned; undoing the entire retained stack must not
	// reach it.
	for i := 0; i < WorkspaceUndoLimit; i++ {
		if _, err := ledger.Undo(GlobalScope()); err != nil {
			t.Fatalf("undo %d: %v", i+1, err)
		}
	}
	if got := readFile(t, files[0]); got != "after" {
		t.Fatalf("pruned entry must be unreachable: a.md = %q, want after", got)
	}

	// The pruned entry's unique before-blob must be gone from the blob store.
	prunedBlob := blobPathFor(ledger, "before-00")
	requireAbsent(t, prunedBlob)
	// A surviving entry's before-blob must remain.
	if _, err := os.Stat(blobPathFor(ledger, fmt.Sprintf("before-%02d", WorkspaceUndoLimit))); err != nil {
		t.Fatalf("surviving blob missing: %v", err)
	}
}

func TestWorkspaceLedger_LoadPrunesOversizedExistingHistory(t *testing.T) {
	ledger, _ := newTestLedger(t)
	oversized := ledgerFile{
		SchemaVersion: UndoLedgerSchemaVersion,
		NextSeq:       WorkspaceUndoLimit + 2,
	}
	for seq := 1; seq <= WorkspaceUndoLimit+1; seq++ {
		entry := LedgerEntry{Seq: seq, TransactionID: fmt.Sprintf("txn-%02d", seq)}
		if seq <= 26 {
			oversized.Entries = append(oversized.Entries, entry)
		} else {
			oversized.RedoEntries = append(oversized.RedoEntries, entry)
		}
	}
	data, err := json.Marshal(oversized)
	if err != nil {
		t.Fatalf("marshal oversized ledger: %v", err)
	}
	writeFile(t, ledger.ledgerPath(), string(data))

	loaded, err := ledger.load()
	if err != nil {
		t.Fatalf("load oversized ledger: %v", err)
	}
	if got := len(loaded.Entries) + len(loaded.RedoEntries); got != WorkspaceUndoLimit {
		t.Fatalf("loaded history size = %d, want %d", got, WorkspaceUndoLimit)
	}
	for _, stack := range [][]LedgerEntry{loaded.Entries, loaded.RedoEntries} {
		for _, entry := range stack {
			if entry.Seq == 1 {
				t.Fatal("oldest oversized entry remained reachable")
			}
		}
	}

	if _, undoErr := ledger.Undo(GlobalScope()); undoErr != nil {
		t.Fatalf("Undo oversized ledger: %v", undoErr)
	}
	persistedData, err := os.ReadFile(ledger.ledgerPath())
	if err != nil {
		t.Fatalf("read persisted ledger: %v", err)
	}
	var persisted ledgerFile
	if err := json.Unmarshal(persistedData, &persisted); err != nil {
		t.Fatalf("unmarshal persisted ledger: %v", err)
	}
	if got := len(persisted.Entries) + len(persisted.RedoEntries); got != WorkspaceUndoLimit {
		t.Fatalf("persisted history size = %d, want %d", got, WorkspaceUndoLimit)
	}
}

// TestWorkspaceLedger_ForwardMutationClearsRedo verifies the linear history
// contract: every successful forward mutation discards all pending redo entries,
// even when the new transaction touches unrelated artifacts.
func TestWorkspaceLedger_ForwardMutationClearsRedo(t *testing.T) {
	ledger, root := newTestLedger(t)
	f1 := filepath.Join(root, "projects", "p", "f1.md")
	f2 := filepath.Join(root, "projects", "p", "f2.md")

	writeFile(t, f1, "b")
	mustRecord(t, ledger, "txn-1", "p", f1, "a", "b")
	if _, err := ledger.Undo(GlobalScope()); err != nil { // f1 -> a, e1 on redo stack
		t.Fatalf("undo: %v", err)
	}

	// A non-overlapping mutation still starts a new branch and clears redo.
	writeFile(t, f2, "y")
	mustRecord(t, ledger, "txn-2", "p", f2, "x", "y")
	if _, ok, _ := ledger.SelectNewestRedo(GlobalScope()); ok {
		t.Fatal("forward mutation must invalidate every redo entry")
	}
	if _, err := ledger.Redo(GlobalScope()); err == nil {
		t.Fatal("expected redo to be unavailable after invalidation")
	}
	requireAbsent(t, blobPathFor(ledger, "a"))
	requireAbsent(t, blobPathFor(ledger, "b"))
}

// TestWorkspaceLedger_RejectsPathOutsideRoots proves a tampered ledger cannot
// escape the approved workspace roots during restore.
func TestWorkspaceLedger_RejectsPathOutsideRoots(t *testing.T) {
	ledger, root := newTestLedger(t)
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
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
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

func TestWorkspaceLedger_LeavesLegacyProjectUndoDirectoryUntouched(t *testing.T) {
	ledger, root := newTestLedger(t)
	artifactPath := filepath.Join(root, "projects", "alpha", "tasks.md")
	legacySentinel := filepath.Join(root, "projects", "alpha", ".undo", "sentinel.json")
	writeFile(t, artifactPath, "after")
	writeFile(t, legacySentinel, "legacy snapshot")

	mustRecord(t, ledger, "txn-1", "alpha", artifactPath, "before", "after")
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	if got := readFile(t, legacySentinel); got != "legacy snapshot" {
		t.Fatalf("legacy .undo sentinel = %q, want untouched snapshot", got)
	}
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

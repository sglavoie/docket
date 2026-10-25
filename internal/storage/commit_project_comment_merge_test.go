package storage

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// This file exercises project-comment merging at the real commit boundary rather
// than on the pure function. The merge decides what the document should be; only
// the commit decides whether anything reaches disk, and the two claims a user
// depends on — that concurrent comments both survive, and that a conflicting one
// costs nothing — are only true if the boundary honors them together.

func commentAnchor() *models.Task { return mergeTestTask("t1", "the anchor") }

// loadCommentDoc reads the canonical document the last commit left on disk.
func loadCommentDoc(t *testing.T, project *models.Project) *ProjectDocument {
	t.Helper()
	doc, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	return doc
}

// TestCommitMerge_ConcurrentCommentAdditionsBothLand is the case the feature
// exists for. Two machines comment on one project without seeing each other; the
// stale writer's commit merges by identity and both comments are on disk
// afterwards, with no conflict recorded and nothing for anyone to resolve.
func TestCommitMerge_ConcurrentCommentAdditionsBothLand(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Comments", "comments")

	base := env.commitDoc(t, project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)},
		commentAnchor()), "", nil)

	// The remote machine comments first and lands.
	env.commitDoc(t, project, commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDC, "added on the remote machine", 2*time.Hour),
	}, commentAnchor()), base.Revision, base)

	// The local machine, still holding the pre-race base, comments too.
	local := projectFromDocument(project, commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "added on the local machine", time.Hour),
	}, commentAnchor()))
	if _, err := env.svc.Commit(ChangeSet{
		Project: local, ExpectedRevision: base.Revision, Base: base, UndoAction: "Add Comment",
	}); err != nil {
		t.Fatalf("Commit a concurrent comment: %v", err)
	}

	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("independent comments recorded %d conflict(s)", len(records))
	}
	merged := loadCommentDoc(t, project)
	assertCommentIDs(t, merged.Comments, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC)
	if got := commentTextByID(t, merged.Comments, mergeCommentIDC); got != "added on the remote machine" {
		t.Fatalf("the remote comment was rewritten by the merge: %q", got)
	}
}

// TestCommitMerge_DivergentCommentEditsRecordAConflictAndWriteNothing is the
// all-or-nothing claim at the boundary. Two rewrites of one comment cannot be
// reconciled, so the transaction is refused whole: the clean task edit riding
// with it is not applied either, the document is byte-for-byte the winner's, and
// the bundle names the comment rather than a task.
func TestCommitMerge_DivergentCommentEditsRecordAConflictAndWriteNothing(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Comments", "comments")

	base := env.commitDoc(t, project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "the original body", 0)},
		commentAnchor()), "", nil)

	remote := env.commitDoc(t, project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "rewritten remotely", 0)},
		commentAnchor()), base.Revision, base)

	// The local machine rewrites the same comment and retitles a task beside it.
	local := projectFromDocument(project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "rewritten locally", 0)},
		mergeTestTask("t1", "the anchor, retitled locally")))
	_, err := env.svc.Commit(ChangeSet{
		Project: local, ExpectedRevision: base.Revision, Base: base,
		UndoAction: "Edit Comment", UndoContext: "rewritten locally",
	})
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("Commit err = %v, want ErrMergeConflict", err)
	}
	var conflict *CommitConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *CommitConflictError, got %T", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].ConflictID == "" {
		t.Fatalf("expected one recorded conflict carrying an id, got %+v", conflict.Conflicts)
	}

	records, listErr := env.store.List()
	if listErr != nil || len(records) != 1 {
		t.Fatalf("expected exactly one durable conflict record, got %d (err %v)", len(records), listErr)
	}
	recorded, ok := findCommentConflict(records[0].Conflicts, mergeCommentIDA, projectCommentTextField)
	if !ok {
		t.Fatalf("the bundle does not identify the diverging comment: %+v", records[0].Conflicts)
	}
	if recorded.TaskTitle != "" {
		t.Fatalf("the bundle presented a comment conflict as a task conflict: %+v", recorded)
	}
	if recorded.Local == recorded.Current {
		t.Fatalf("the bundle cannot tell the two sides apart: %+v", recorded)
	}
	if records[0].Action != "Edit Comment" {
		t.Fatalf("recorded conflict action = %q, want the mutation's own", records[0].Action)
	}

	// Nothing partial: neither the refused comment nor the clean task edit that
	// rode with it is on disk.
	after := loadCommentDoc(t, project)
	if after.Revision != remote.Revision {
		t.Fatalf("on-disk revision moved on a refused commit: got %q want %q", after.Revision, remote.Revision)
	}
	if got := commentTextByID(t, after.Comments, mergeCommentIDA); got != "rewritten remotely" {
		t.Fatalf("the refused comment was written anyway: %q", got)
	}
	if firstTaskTitle(after) != "the anchor" {
		t.Fatalf("the companion task edit was applied by a refused commit: %q", firstTaskTitle(after))
	}

	// The retry-only path: adopting remote writes nothing, clears the bundle, and
	// leaves the remote comment intact for the retry to build on.
	resolveSvc := NewCommitService(filepath.Join(env.dataDir, ".tt", "transactions"), []string{env.dataDir})
	res, err := ResolveConflict(env.store, resolveSvc, conflict.Conflicts[0].ConflictID, ResolveUseRemote, "")
	if err != nil {
		t.Fatalf("ResolveConflict(remote): %v", err)
	}
	if res.Committed || !res.RetryRequired {
		t.Fatalf("expected a non-writing acknowledgement requiring retry, got %+v", res)
	}
	if records, _ = env.store.List(); len(records) != 0 {
		t.Fatalf("adopting remote left %d conflict(s) recorded", len(records))
	}
	adopted := loadCommentDoc(t, project)
	if got := commentTextByID(t, adopted.Comments, mergeCommentIDA); got != "rewritten remotely" {
		t.Fatalf("adopting remote changed the accepted comment: %q", got)
	}

	// The retry reads what the winner left and re-applies the local intent on top.
	retried := projectFromDocument(project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "rewritten locally after the retry", 0)},
		mergeTestTask("t1", "the anchor, retitled locally")))
	if _, err = env.svc.Commit(ChangeSet{
		Project: retried, ExpectedRevision: adopted.Revision, Base: adopted, UndoAction: "Edit Comment",
	}); err != nil {
		t.Fatalf("retrying the refused comment edit: %v", err)
	}
	settled := loadCommentDoc(t, project)
	if got := commentTextByID(t, settled.Comments, mergeCommentIDA); got != "rewritten locally after the retry" {
		t.Fatalf("the retry did not land: %q", got)
	}
	if firstTaskTitle(settled) != "the anchor, retitled locally" {
		t.Fatalf("the retry lost its companion task edit: %q", firstTaskTitle(settled))
	}
}

// newLedgeredCommentEnv builds a commit environment whose transactions are
// recorded in a workspace ledger. The merge-only tests deliberately commit
// without one; an undo test cannot, and both undo tests below have to be looking
// at the same ledger wiring or they would be proving different things.
func newLedgeredCommentEnv(t *testing.T) (mergeCommitEnv, *WorkspaceLedger) {
	t.Helper()
	dataDir := t.TempDir()
	store := NewConflictStore(filepath.Join(dataDir, ".tt", "conflicts"))
	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	svc := NewCommitServiceWithLedger(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}, ledger).
		WithConflictStore(store)
	return mergeCommitEnv{svc: svc, store: store, dataDir: dataDir}, ledger
}

// TestCommitMerge_UndoingALocalCommentKeepsTheRemoteAddition is the sharper half
// of the ledger contract, and the one a project comment is the first mutation to
// reach: the undone commit is itself a comment addition, so the entry being taken
// back and the comment that must survive it are the same kind of thing.
//
// Undo restores the bytes the commit replaced. Those bytes are the merged
// document's predecessor — the remote machine's — not the local machine's own
// pre-merge draft, which never existed on disk and does not name the remote
// comment at all. Restoring that draft would take a comment this machine never
// saw off the disk, and no surface would ever report it: the local user would see
// their own comment disappear, which is what they asked for, and the other
// machine's along with it, which they did not.
func TestCommitMerge_UndoingALocalCommentKeepsTheRemoteAddition(t *testing.T) {
	env, ledger := newLedgeredCommentEnv(t)
	project := env.project("p1", "Comments", "comments")

	base := env.commitDoc(t, project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)},
		commentAnchor()), "", nil)

	// The remote machine comments first and lands.
	env.commitDoc(t, project, commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDC, "added on the remote machine", 2*time.Hour),
	}, commentAnchor()), base.Revision, base)

	// The local machine, still holding the pre-race base, adds its own.
	local := projectFromDocument(project, commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDB, "added on the local machine", time.Hour),
	}, commentAnchor()))
	if _, err := env.svc.Commit(ChangeSet{
		Project: local, ExpectedRevision: base.Revision, Base: base,
		UndoAction: "Add Project Comment", UndoContext: project.Name,
	}); err != nil {
		t.Fatalf("Commit a concurrent project comment: %v", err)
	}
	assertCommentIDs(t, loadCommentDoc(t, project).Comments, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC)

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	undone := loadCommentDoc(t, project)
	assertCommentIDs(t, undone.Comments, mergeCommentIDA, mergeCommentIDC)
	if got := commentTextByID(t, undone.Comments, mergeCommentIDC); got != "added on the remote machine" {
		t.Fatalf("undoing the local comment rewrote the remote one: %q", got)
	}

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	assertCommentIDs(t, loadCommentDoc(t, project).Comments, mergeCommentIDA, mergeCommentIDB, mergeCommentIDC)
}

// TestCommitMerge_UndoAndRedoPreserveAnAcceptedRemoteComment covers the ledger
// side of a merged commit. Undo restores the bytes the commit replaced, which
// are the remote machine's — so a comment the merge accepted from remote must
// still be there afterwards, and redo must not lose it either. A merged commit
// whose undo restored the local writer's pre-merge document would delete a
// comment that machine never saw.
func TestCommitMerge_UndoAndRedoPreserveAnAcceptedRemoteComment(t *testing.T) {
	env, ledger := newLedgeredCommentEnv(t)
	svc := env.svc
	project := env.project("p1", "Comments", "comments")

	base := env.commitDoc(t, project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)},
		commentAnchor()), "", nil)

	// The remote machine adds a comment and lands.
	env.commitDoc(t, project, commentTestDoc([]*models.ProjectComment{
		mergeComment(mergeCommentIDA, "the first note", 0),
		mergeComment(mergeCommentIDC, "added on the remote machine", 2*time.Hour),
	}, commentAnchor()), base.Revision, base)

	// The local machine, stale, edits a task. The merge accepts the remote comment.
	local := projectFromDocument(project, commentTestDoc(
		[]*models.ProjectComment{mergeComment(mergeCommentIDA, "the first note", 0)},
		mergeTestTask("t1", "the anchor, retitled locally")))
	if _, err := svc.Commit(ChangeSet{
		Project: local, ExpectedRevision: base.Revision, Base: base,
		UndoAction: "Edit Task", UndoContext: "the anchor",
	}); err != nil {
		t.Fatalf("Commit the merged task edit: %v", err)
	}
	merged := loadCommentDoc(t, project)
	assertCommentIDs(t, merged.Comments, mergeCommentIDA, mergeCommentIDC)
	if firstTaskTitle(merged) != "the anchor, retitled locally" {
		t.Fatalf("the local task edit did not land: %q", firstTaskTitle(merged))
	}

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	undone := loadCommentDoc(t, project)
	assertCommentIDs(t, undone.Comments, mergeCommentIDA, mergeCommentIDC)
	if firstTaskTitle(undone) != "the anchor" {
		t.Fatalf("undo did not take the task edit back: %q", firstTaskTitle(undone))
	}

	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	redone := loadCommentDoc(t, project)
	assertCommentIDs(t, redone.Comments, mergeCommentIDA, mergeCommentIDC)
	if firstTaskTitle(redone) != "the anchor, retitled locally" {
		t.Fatalf("redo did not restore the task edit: %q", firstTaskTitle(redone))
	}
}

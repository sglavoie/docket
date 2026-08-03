package storage

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

// conflictTestEnv is a fully-wired resolution environment: a commit service, a
// conflict store, and a project whose tasks.md lives under the same data dir.
type conflictTestEnv struct {
	store   *ConflictStore
	svc     *CommitService
	project *models.Project
	dataDir string
}

func newConflictTestEnv(t *testing.T) conflictTestEnv {
	t.Helper()
	dataDir := t.TempDir()
	return conflictTestEnv{
		store:   NewConflictStore(filepath.Join(dataDir, ".tt", "conflicts")),
		svc:     NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}),
		project: &models.Project{ID: "p1", Name: "Alpha", Slug: "alpha", Path: filepath.Join(dataDir, "projects", "alpha")},
		dataDir: dataDir,
	}
}

// setDisk writes a document to the project on disk and returns the canonical
// document as it parses back — the form all on-disk revisions are computed from.
// Using the canonical form keeps recorded revisions aligned with what
// LoadProjectDocument later observes, so "remote advanced" reflects a real edit.
func (e conflictTestEnv) setDisk(t *testing.T, doc *ProjectDocument) *ProjectDocument {
	t.Helper()
	if _, err := e.svc.Commit(ChangeSet{Project: projectFromDocument(e.project, doc)}); err != nil {
		t.Fatalf("setDisk commit: %v", err)
	}
	return e.onDiskDoc(t)
}

func (e conflictTestEnv) onDiskDoc(t *testing.T) *ProjectDocument {
	t.Helper()
	doc, err := LoadProjectDocument(e.project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	return doc
}

// cloneEdit returns a copy of doc with its first task mutated and its revision
// recomputed, leaving the source untouched.
func cloneEdit(doc *ProjectDocument, edit func(*models.Task)) *ProjectDocument {
	copied := *doc
	copied.Tasks = cloneDocTasks(doc.Tasks)
	edit(copied.Tasks[0])
	copied.Revision = copied.ComputeRevision()
	return &copied
}

// TestMergeOrRecordConflict_CleanMergeReturnsMerged verifies a non-conflicting
// merge returns the merged document and records nothing.
func TestMergeOrRecordConflict_CleanMergeReturnsMerged(t *testing.T) {
	env := newConflictTestEnv(t)

	base := env.setDisk(t, mergeTestDoc("Alpha", mergeTestTask("t1", "title")))
	local := cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" })
	current := cloneEdit(base, func(tk *models.Task) { tk.Priority = models.PriorityMin })

	result, id, err := MergeOrRecordConflict(env.store, env.project, base, local, current, "edit", "")
	if err != nil {
		t.Fatalf("MergeOrRecordConflict: %v", err)
	}
	if id != "" {
		t.Fatalf("expected no conflict id for a clean merge, got %q", id)
	}
	if result.Merged == nil || result.HasConflicts() {
		t.Fatal("expected a clean merged document")
	}
	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("clean merge should record no conflict, found %d", len(records))
	}
}

// TestMergeOrRecordConflict_ConflictRecordsBundle verifies a conflicting merge
// persists a bundle carrying the structured conflicts.
func TestMergeOrRecordConflict_ConflictRecordsBundle(t *testing.T) {
	env := newConflictTestEnv(t)

	base := env.setDisk(t, mergeTestDoc("Alpha", mergeTestTask("t1", "title")))
	local := cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" })
	current := cloneEdit(base, func(tk *models.Task) { tk.Title = "remote title" })

	result, id, err := MergeOrRecordConflict(env.store, env.project, base, local, current, "edit", "")
	if err != nil {
		t.Fatalf("MergeOrRecordConflict: %v", err)
	}
	if id == "" || result.Merged != nil {
		t.Fatalf("expected a recorded conflict with nil merged doc; id=%q merged=%v", id, result.Merged)
	}
	bundle, err := env.store.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(bundle.Record.Conflicts) == 0 || bundle.Record.Conflicts[0].Field != "title" {
		t.Fatalf("recorded conflicts = %+v", bundle.Record.Conflicts)
	}
}

// recordTitleConflict sets up a project on disk holding `remote` content and
// records a title conflict between local and remote against a common base. It
// returns the conflict id and the three canonical documents.
func recordTitleConflict(t *testing.T, env conflictTestEnv) (id string, base, remote *ProjectDocument) {
	t.Helper()
	base = env.setDisk(t, mergeTestDoc("Alpha", mergeTestTask("t1", "base title")))
	local := cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" })
	remoteEdit := cloneEdit(base, func(tk *models.Task) { tk.Title = "remote title" })
	remote = env.setDisk(t, remoteEdit) // the workspace now holds the remote content

	var err error
	id, err = RecordMergeConflict(env.store, env.project, base, local, remote, []MergeConflict{{Kind: MergeConflictFieldEdit, Field: "title"}}, "edit", "")
	if err != nil {
		t.Fatalf("record: %v", err)
	}
	return id, base, remote
}

// TestResolveConflict_UseLocalCommits verifies resolving with local commits the
// local document and clears the conflict when the remote has not advanced.
func TestResolveConflict_UseLocalCommits(t *testing.T) {
	env := newConflictTestEnv(t)
	id, _, _ := recordTitleConflict(t, env)

	res, err := ResolveConflict(env.store, env.svc, id, ResolveUseLocal, "")
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if !res.Committed || res.Remerged {
		t.Fatalf("unexpected resolve result: %+v", res)
	}
	if got := env.onDiskDoc(t); got.Tasks[0].Title != "local title" {
		t.Fatalf("on-disk title = %q, want local title", got.Tasks[0].Title)
	}
	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("resolved conflict should be deleted, %d remain", len(records))
	}
}

// TestResolveConflict_UseRemoteWritesNothing verifies adopting remote clears the
// conflict without writing over the on-disk content.
func TestResolveConflict_UseRemoteWritesNothing(t *testing.T) {
	env := newConflictTestEnv(t)
	id, _, remote := recordTitleConflict(t, env)

	res, err := ResolveConflict(env.store, env.svc, id, ResolveUseRemote, "")
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if res.Committed {
		t.Fatal("adopting remote should not commit")
	}
	if got := env.onDiskDoc(t); got.Tasks[0].Title != "remote title" || got.Revision != remote.Revision {
		t.Fatalf("on-disk changed unexpectedly: %q rev=%q", got.Tasks[0].Title, got.Revision)
	}
	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("resolved conflict should be deleted, %d remain", len(records))
	}
}

// TestResolveConflict_UseFileValidatesAndCommits verifies a valid supplied merged
// file is committed.
func TestResolveConflict_UseFileValidatesAndCommits(t *testing.T) {
	env := newConflictTestEnv(t)
	id, base, _ := recordTitleConflict(t, env)

	mergedDoc := cloneEdit(base, func(tk *models.Task) {
		tk.Title = "hand merged"
		tk.ApplyWorkflowState(models.StatePaused)
	})
	mergedPath := filepath.Join(t.TempDir(), "merged.md")
	if err := os.WriteFile(mergedPath, []byte(mustWriteCurrentDocument(t, mergedDoc)), 0o600); err != nil {
		t.Fatal(err)
	}

	res, err := ResolveConflict(env.store, env.svc, id, ResolveUseFile, mergedPath)
	if err != nil {
		t.Fatalf("ResolveConflict(file): %v", err)
	}
	if !res.Committed {
		t.Fatal("expected the merged file to be committed")
	}
	if got := env.onDiskDoc(t); got.Tasks[0].Title != "hand merged" || got.Tasks[0].State() != models.StatePaused {
		t.Fatalf("on-disk task = title %q state %q, want hand merged/paused", got.Tasks[0].Title, got.Tasks[0].State())
	}
}

// TestResolveConflict_UseFileRejectsInvalid verifies a merged file that breaks a
// state invariant is rejected and the conflict is left intact.
func TestResolveConflict_UseFileRejectsInvalid(t *testing.T) {
	env := newConflictTestEnv(t)
	id, base, _ := recordTitleConflict(t, env)

	// A terminal (done) task with no completion timestamp violates the state
	// invariant enforced by auditMergedDocument. ApplyWorkflowState sets the
	// authoritative WorkflowState (which the v2 writer serializes and the audit
	// reads) without touching CompletedAt.
	badContent := strings.Replace(mustWriteCurrentDocument(t, base), "state: backlog", "state: done", 1)
	badPath := filepath.Join(t.TempDir(), "bad.md")
	if err := os.WriteFile(badPath, []byte(badContent), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := ResolveConflict(env.store, env.svc, id, ResolveUseFile, badPath); err == nil {
		t.Fatal("expected validation failure for an invariant-violating merged file")
	}
	if records, _ := env.store.List(); len(records) != 1 {
		t.Fatalf("rejected resolution must leave the conflict intact, %d remain", len(records))
	}
}

// TestResolveConflict_RemoteAdvancedRemergeClean verifies that when the remote
// advanced since the conflict was recorded, a local resolution is re-merged
// against the newer remote and commits when the re-merge is clean.
func TestResolveConflict_RemoteAdvancedRemergeClean(t *testing.T) {
	env := newConflictTestEnv(t)
	id, base, _ := recordTitleConflict(t, env)

	// A second remote edit during resolution touches a DIFFERENT field
	// (priority) than local (title), so re-merging is clean.
	env.setDisk(t, cloneEdit(base, func(tk *models.Task) { tk.Priority = models.PriorityMin }))

	res, err := ResolveConflict(env.store, env.svc, id, ResolveUseLocal, "")
	if err != nil {
		t.Fatalf("ResolveConflict: %v", err)
	}
	if !res.Committed || !res.Remerged {
		t.Fatalf("expected a clean re-merge commit, got %+v", res)
	}
	got := env.onDiskDoc(t)
	if got.Tasks[0].Title != "local title" {
		t.Fatalf("title = %q, want local title (local edit preserved)", got.Tasks[0].Title)
	}
	if got.Tasks[0].Priority != models.PriorityMin {
		t.Fatalf("priority = %d, want the newer remote's %d (folded in)", got.Tasks[0].Priority, models.PriorityMin)
	}
}

// TestResolveConflict_RemoteAdvancedRemergeConflicts verifies that when the
// re-merge against the newer remote itself conflicts, the chosen resolution is
// not committed, ErrRemoteReconflict is returned, and a fresh conflict capturing
// the newer remote replaces the stale one.
func TestResolveConflict_RemoteAdvancedRemergeConflicts(t *testing.T) {
	env := newConflictTestEnv(t)
	id, base, remote := recordTitleConflict(t, env)

	// A second remote edit re-touches the SAME field (title) differently, so
	// re-merging local's title against it cannot auto-resolve.
	env.setDisk(t, cloneEdit(base, func(tk *models.Task) { tk.Title = "newer remote title" }))

	res, err := ResolveConflict(env.store, env.svc, id, ResolveUseLocal, "")
	if !errors.Is(err, ErrRemoteReconflict) {
		t.Fatalf("err = %v, want ErrRemoteReconflict", err)
	}
	if res.Committed || res.NewConflictID == "" {
		t.Fatalf("expected an unresolved re-conflict result, got %+v", res)
	}
	if got := env.onDiskDoc(t); got.Tasks[0].Title != "newer remote title" {
		t.Fatalf("on-disk title changed to %q; resolution must not have written", got.Tasks[0].Title)
	}
	records, _ := env.store.List()
	if len(records) != 1 || records[0].ID != res.NewConflictID {
		t.Fatalf("expected only the fresh conflict %s, got %+v", res.NewConflictID, records)
	}
	if records[0].RemoteRevision == remote.Revision {
		t.Fatal("fresh conflict should capture the newer remote revision, not the stale one")
	}
}

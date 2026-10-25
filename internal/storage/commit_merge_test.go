package storage

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
)

// mergeCommitEnv is a commit boundary wired with a durable conflict store, so a
// stale-revision commit three-way merges (clean) or records a conflict (true
// divergence) instead of aborting with a bare stale-revision error.
type mergeCommitEnv struct {
	svc     *CommitService
	store   *ConflictStore
	dataDir string
}

func newMergeCommitEnv(t *testing.T) mergeCommitEnv {
	t.Helper()
	dataDir := t.TempDir()
	store := NewConflictStore(filepath.Join(dataDir, ".docket", "conflicts"))
	svc := NewCommitService(filepath.Join(dataDir, ".docket", "transactions"), []string{dataDir}).
		WithConflictStore(store)
	return mergeCommitEnv{svc: svc, store: store, dataDir: dataDir}
}

func (e mergeCommitEnv) project(id, name, slug string) *models.Project {
	return &models.Project{ID: id, Name: name, Slug: slug, Path: filepath.Join(e.dataDir, "projects", slug)}
}

// commitDoc writes doc as project's tasks.md through the (in-sync) commit path and
// returns the canonical on-disk document — the base every later revision is
// measured from.
func (e mergeCommitEnv) commitDoc(t *testing.T, project *models.Project, doc *ProjectDocument, expected string, base *ProjectDocument) *ProjectDocument {
	t.Helper()
	if _, err := e.svc.Commit(ChangeSet{
		Project:          projectFromDocument(project, doc),
		ExpectedRevision: expected,
		Base:             base,
	}); err != nil {
		t.Fatalf("commitDoc: %v", err)
	}
	loaded, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	return loaded
}

func firstTaskTitle(doc *ProjectDocument) string {
	if len(doc.Tasks) == 0 {
		return ""
	}
	return doc.Tasks[0].Title
}

// TestCommitMerge_IndependentEditsMergeAutomatically verifies that a stale commit
// whose local edit touches a different field than the concurrent on-disk edit is
// merged automatically through the production commit path and returns a
// reload-stable revision.
func TestCommitMerge_IndependentEditsMergeAutomatically(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")

	base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("t1", "title")), "", nil)

	// A concurrent writer changes the priority and commits it to disk first.
	current := cloneEdit(base, func(tk *models.Task) { tk.Priority = models.PriorityMin })
	onDisk := env.commitDoc(t, project, current, base.Revision, base)

	// Our writer, still holding the older base, changes the title. Its expected
	// revision is now stale, so the commit boundary three-way merges.
	local := projectFromDocument(project, cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" }))
	result, err := env.svc.Commit(ChangeSet{Project: local, ExpectedRevision: base.Revision, Base: base})
	if err != nil {
		t.Fatalf("Commit merged edit: %v", err)
	}

	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("clean merge must record no conflict, found %d", len(records))
	}

	merged, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload merged: %v", err)
	}
	if got := firstTaskTitle(merged); got != "local title" {
		t.Fatalf("merged title = %q, want the local edit %q", got, "local title")
	}
	if got := merged.Tasks[0].Priority; got != models.PriorityMin {
		t.Fatalf("merged priority = %d, want the concurrent edit %d", got, models.PriorityMin)
	}
	if merged.Revision == onDisk.Revision {
		t.Fatal("merge should have advanced the on-disk revision")
	}
	if result.Revision != merged.Revision {
		t.Fatalf("commit result revision %q is not reload-stable (disk %q)", result.Revision, merged.Revision)
	}
}

func TestCommitMerge_CleanStaleMergePreservesIndependentFieldsAndLatestTimestamp(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")
	baseTime := time.Date(2026, 7, 14, 9, 0, 0, 0, time.Local)
	remoteTime := baseTime.Add(time.Hour)
	localTime := baseTime.Add(2 * time.Hour)

	initial := mergeTestDoc("Alpha", mergeTestTask("t1", "title"))
	initial.Tasks[0].CreatedAt = baseTime
	initial.Tasks[0].UpdatedAt = baseTime
	base := env.commitDoc(t, project, initial, "", nil)

	// The remote writer lands first with a priority change.
	remote := cloneEdit(base, func(task *models.Task) {
		task.Priority = models.PriorityMin
		task.UpdatedAt = remoteTime
	})
	env.commitDoc(t, project, remote, base.Revision, base)

	// The local writer retains the older base, edits an independent field, and
	// carries a newer lifecycle timestamp.
	local := projectFromDocument(project, cloneEdit(base, func(task *models.Task) {
		task.Title = "local title"
		task.UpdatedAt = localTime
	}))
	if _, err := env.svc.Commit(ChangeSet{Project: local, ExpectedRevision: base.Revision, Base: base}); err != nil {
		t.Fatalf("Commit clean stale merge: %v", err)
	}

	merged, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("LoadProjectDocument: %v", err)
	}
	if merged.Tasks[0].Title != "local title" || merged.Tasks[0].Priority != models.PriorityMin {
		t.Fatalf("merged business fields = title %q priority %d", merged.Tasks[0].Title, merged.Tasks[0].Priority)
	}
	if !merged.Tasks[0].UpdatedAt.Equal(localTime) {
		t.Fatalf("merged UpdatedAt = %v, want latest %v", merged.Tasks[0].UpdatedAt, localTime)
	}

	if records, listErr := env.store.List(); listErr != nil || len(records) != 0 {
		t.Fatalf("clean merge recorded conflicts: records=%d err=%v", len(records), listErr)
	}
}

// TestCommitMerge_SameFieldConflictIsRecordedAndAbortsWrite verifies that a stale
// commit conflicting on the same field is durably recorded and leaves the on-disk
// project exactly as the concurrent writer left it (no partial write).
func TestCommitMerge_SameFieldConflictIsRecordedAndAbortsWrite(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")

	base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("t1", "title")), "", nil)

	// Concurrent writer renames the same task and commits it to disk.
	remote := env.commitDoc(t, project, cloneEdit(base, func(tk *models.Task) { tk.Title = "remote title" }), base.Revision, base)

	// Our writer renames the same task from the older base: a true same-field conflict.
	local := projectFromDocument(project, cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" }))
	_, err := env.svc.Commit(ChangeSet{
		Project:          local,
		ExpectedRevision: base.Revision,
		Base:             base,
		UndoAction:       "edit title",
	})
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("Commit err = %v, want ErrMergeConflict", err)
	}
	var conflict *CommitConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *CommitConflictError, got %T", err)
	}
	if len(conflict.Conflicts) != 1 || conflict.Conflicts[0].ConflictID == "" {
		t.Fatalf("expected one recorded conflict with an id, got %+v", conflict.Conflicts)
	}
	if conflict.Conflicts[0].ProjectID != project.ID {
		t.Fatalf("conflict project = %q, want %q", conflict.Conflicts[0].ProjectID, project.ID)
	}

	// Durability: the bundle survives and carries the structured conflicts and the action.
	records, err := env.store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("expected exactly one durable conflict record, got %d (err %v)", len(records), err)
	}
	if records[0].Action != "edit title" || len(records[0].Conflicts) == 0 {
		t.Fatalf("recorded conflict missing action/details: %+v", records[0])
	}

	// No partial write: the on-disk state is still exactly the concurrent writer's.
	after, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload after conflict: %v", err)
	}
	if after.Revision != remote.Revision {
		t.Fatalf("on-disk revision changed on an aborted conflicting commit: got %q want %q", after.Revision, remote.Revision)
	}
	if got := firstTaskTitle(after); got != "remote title" {
		t.Fatalf("on-disk title = %q, want untouched %q", got, "remote title")
	}
}

// TestCommitMerge_RecordedConflictRequiresRetry verifies that a conflict recorded
// by the commit boundary cannot later commit only its selected project document.
// The original ChangeSet was aborted, so the operator must adopt current remote
// state to clear the record and retry the complete operation through the app.
func TestCommitMerge_RecordedConflictResolvesThroughSharedService(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")

	base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("t1", "title")), "", nil)
	env.commitDoc(t, project, cloneEdit(base, func(tk *models.Task) { tk.Title = "remote title" }), base.Revision, base)

	local := projectFromDocument(project, cloneEdit(base, func(tk *models.Task) { tk.Title = "local title" }))
	_, err := env.svc.Commit(ChangeSet{Project: local, ExpectedRevision: base.Revision, Base: base})
	var conflict *CommitConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *CommitConflictError, got %v", err)
	}
	id := conflict.Conflicts[0].ConflictID

	// A local resolution must fail closed: the conflict bundle does not persist the
	// complete original ChangeSet, even when this particular fixture has only one
	// project document.
	resolveSvc := NewCommitService(filepath.Join(env.dataDir, ".docket", "transactions"), []string{env.dataDir})
	res, err := ResolveConflict(env.store, resolveSvc, id, ResolveUseLocal, "")
	if !errors.Is(err, ErrConflictRetryRequired) {
		t.Fatalf("ResolveConflict err = %v, want ErrConflictRetryRequired", err)
	}
	if res == nil || res.Committed || !res.RetryRequired {
		t.Fatalf("expected non-applying retry-required result, got %+v", res)
	}

	unchanged, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload after rejected resolution: %v", err)
	}
	if got := firstTaskTitle(unchanged); got != "remote title" {
		t.Fatalf("rejected resolution changed title to %q, want remote title", got)
	}
	if records, _ := env.store.List(); len(records) != 1 {
		t.Fatalf("rejected resolution should retain the conflict, found %d", len(records))
	}

	// Adopting remote is the explicit acknowledgement path: it writes nothing,
	// clears the record, and tells the caller to retry the original operation.
	res, err = ResolveConflict(env.store, resolveSvc, id, ResolveUseRemote, "")
	if err != nil {
		t.Fatalf("ResolveConflict(remote): %v", err)
	}
	if res.Committed || !res.RetryRequired {
		t.Fatalf("expected remote acknowledgement requiring retry, got %+v", res)
	}
	if records, _ := env.store.List(); len(records) != 0 {
		t.Fatalf("remote acknowledgement should clear the conflict, found %d", len(records))
	}
}

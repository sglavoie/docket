package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/models"
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
	store := NewConflictStore(filepath.Join(dataDir, ".tt", "conflicts"))
	svc := NewCommitService(filepath.Join(dataDir, ".tt", "transactions"), []string{dataDir}).
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

func TestCommitMerge_CleanStaleMergeRetainsActivityAndRecordsLocalDeltaOnce(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")
	baseTime := time.Date(2026, 7, 14, 9, 0, 0, 0, time.Local)
	remoteTime := baseTime.Add(time.Hour)
	localTime := baseTime.Add(2 * time.Hour)

	initial := mergeTestDoc("Alpha", mergeTestTask("t1", "title"))
	initial.Tasks[0].CreatedAt = baseTime
	initial.Tasks[0].UpdatedAt = baseTime
	base := env.commitDoc(t, project, initial, "", nil)

	// The remote writer lands first and records its priority activity through the
	// ordinary in-sync path.
	remote := cloneEdit(base, func(task *models.Task) {
		task.Priority = models.PriorityMin
		task.UpdatedAt = remoteTime
	})
	current := env.commitDoc(t, project, remote, base.Revision, base)
	if len(current.Tasks[0].Activity) != 1 || current.Tasks[0].Activity[0].Field != "priority" {
		t.Fatalf("remote activity = %+v, want one priority entry", current.Tasks[0].Activity)
	}

	// The local writer retains the older base, edits an independent field, and
	// carries a different derived timestamp. The stale commit must preserve the
	// remote entry and append only the accepted local title delta.
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

	activityCounts := make(map[string]int)
	for _, entry := range merged.Tasks[0].Activity {
		activityCounts[entry.Field]++
	}
	if activityCounts["priority"] != 1 || activityCounts["title"] != 1 || len(merged.Tasks[0].Activity) != 2 {
		t.Fatalf("merged activity = %+v, want remote priority and local title exactly once", merged.Tasks[0].Activity)
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
	resolveSvc := NewCommitService(filepath.Join(env.dataDir, ".tt", "transactions"), []string{env.dataDir})
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

// TestCommitMerge_ComplexConflictNeverPartiallyAppliesIntent covers the complete
// F002 regression shape: a multi-project transaction carries a clean companion
// edit, a configuration artifact, and attachment bytes while another project
// conflicts. Neither the aborted commit nor a later local resolution may apply
// any subset. The conflict is explicitly retry-only.
func TestCommitMerge_ComplexConflictNeverPartiallyAppliesIntent(t *testing.T) {
	env := newMergeCommitEnv(t)
	projA := env.project("pa", "Alpha", "alpha")
	projB := env.project("pb", "Beta", "beta")

	baseA := env.commitDoc(t, projA, mergeTestDoc("Alpha", mergeTestTask("a1", "base alpha")), "", nil)
	baseB := env.commitDoc(t, projB, mergeTestDoc("Beta", mergeTestTask("b1", "base beta")), "", nil)
	remoteA := env.commitDoc(t, projA, cloneEdit(baseA, func(task *models.Task) {
		task.Title = "remote alpha"
	}), baseA.Revision, baseA)

	localA := projectFromDocument(projA, cloneEdit(baseA, func(task *models.Task) {
		task.Title = "local alpha"
	}))
	localBDoc := cloneEdit(baseB, func(task *models.Task) {
		task.Title = "local beta"
		task.Attachments = append(task.Attachments, models.Attachment{Filename: "evidence.txt"})
	})
	localB := projectFromDocument(projB, localBDoc)

	artifactPath := filepath.Join(env.dataDir, ".tt", "planning.json")
	sourcePath := filepath.Join(env.dataDir, "evidence.txt")
	if err := os.WriteFile(sourcePath, []byte("attachment bytes"), FilePermissions); err != nil {
		t.Fatalf("write attachment source: %v", err)
	}
	attachmentPath := GetAttachmentPath(projB, localB.Tasks[0], "evidence.txt")

	_, err := env.svc.CommitMulti([]ChangeSet{
		{
			Project:          localA,
			ExpectedRevision: baseA.Revision,
			Base:             baseA,
			UndoAction:       "complex edit",
		},
		{
			Project:          localB,
			ExpectedRevision: baseB.Revision,
			Base:             baseB,
			Artifacts: []ArtifactChange{{
				Path: artifactPath,
				Data: []byte(`{"focus_queue":["b1"]}`),
			}},
			AttachmentOps: []AttachmentOp{{
				Kind:        AttachmentCopy,
				Task:        localB.Tasks[0],
				DestProject: projB,
				SourcePath:  sourcePath,
				Filename:    "evidence.txt",
			}},
		},
	})
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("CommitMulti err = %v, want ErrMergeConflict", err)
	}
	var conflict *CommitConflictError
	if !errors.As(err, &conflict) || len(conflict.Conflicts) != 1 {
		t.Fatalf("expected one recorded conflict, got %v", err)
	}
	id := conflict.Conflicts[0].ConflictID

	bundle, err := env.store.Load(id)
	if err != nil {
		t.Fatalf("Load conflict: %v", err)
	}
	if !bundle.Record.OperationRetryRequired() {
		t.Fatal("commit-origin conflict must be marked as requiring an operation retry")
	}

	assertComplexIntentAbsent := func(stage string) {
		t.Helper()
		afterA, loadErr := LoadProjectDocument(projA)
		if loadErr != nil {
			t.Fatalf("%s: load Alpha: %v", stage, loadErr)
		}
		if afterA.Revision != remoteA.Revision || firstTaskTitle(afterA) != "remote alpha" {
			t.Fatalf("%s: conflicting project changed: title=%q revision=%q", stage, firstTaskTitle(afterA), afterA.Revision)
		}
		afterB, loadErr := LoadProjectDocument(projB)
		if loadErr != nil {
			t.Fatalf("%s: load Beta: %v", stage, loadErr)
		}
		if afterB.Revision != baseB.Revision || firstTaskTitle(afterB) != "base beta" {
			t.Fatalf("%s: companion project changed: title=%q revision=%q", stage, firstTaskTitle(afterB), afterB.Revision)
		}
		if _, statErr := os.Stat(artifactPath); !os.IsNotExist(statErr) {
			t.Fatalf("%s: configuration artifact was applied: %v", stage, statErr)
		}
		if _, statErr := os.Stat(attachmentPath); !os.IsNotExist(statErr) {
			t.Fatalf("%s: attachment bytes were applied: %v", stage, statErr)
		}
	}
	assertComplexIntentAbsent("after aborted commit")

	resolveSvc := NewCommitService(filepath.Join(env.dataDir, ".tt", "transactions"), []string{env.dataDir})
	res, resolveErr := ResolveConflict(env.store, resolveSvc, id, ResolveUseLocal, "")
	if !errors.Is(resolveErr, ErrConflictRetryRequired) {
		t.Fatalf("ResolveConflict err = %v, want ErrConflictRetryRequired", resolveErr)
	}
	if res == nil || res.Committed || !res.RetryRequired {
		t.Fatalf("expected non-applying retry-required result, got %+v", res)
	}
	assertComplexIntentAbsent("after rejected local resolution")

	res, resolveErr = ResolveConflict(env.store, resolveSvc, id, ResolveUseRemote, "")
	if resolveErr != nil {
		t.Fatalf("ResolveConflict(remote): %v", resolveErr)
	}
	if res.Committed || !res.RetryRequired {
		t.Fatalf("expected remote acknowledgement requiring retry, got %+v", res)
	}
	assertComplexIntentAbsent("after remote acknowledgement")
	if records, listErr := env.store.List(); listErr != nil || len(records) != 0 {
		t.Fatalf("remote acknowledgement should clear retry-only conflict: records=%d err=%v", len(records), listErr)
	}
}

// TestCommitMerge_MultiProjectConflictWritesNothing verifies that when a
// multi-project commit has one cleanly-mergeable project and one conflicting
// project, the whole set aborts: the clean merge is discarded, neither project's
// tasks.md changes, and only the conflicting project records a bundle.
func TestCommitMerge_MultiProjectConflictWritesNothing(t *testing.T) {
	env := newMergeCommitEnv(t)
	projA := env.project("pa", "Alpha", "alpha")
	projB := env.project("pb", "Beta", "beta")

	baseA := env.commitDoc(t, projA, mergeTestDoc("Alpha", mergeTestTask("a1", "a title")), "", nil)
	baseB := env.commitDoc(t, projB, mergeTestDoc("Beta", mergeTestTask("b1", "b title")), "", nil)

	// Advance both projects on disk so both incoming commits are stale.
	diskA := env.commitDoc(t, projA, cloneEdit(baseA, func(tk *models.Task) { tk.Priority = models.PriorityMin }), baseA.Revision, baseA)
	diskB := env.commitDoc(t, projB, cloneEdit(baseB, func(tk *models.Task) { tk.Title = "remote b" }), baseB.Revision, baseB)

	csA := ChangeSet{ // stale but cleanly mergeable (title vs. priority)
		Project:          projectFromDocument(projA, cloneEdit(baseA, func(tk *models.Task) { tk.Title = "local a" })),
		ExpectedRevision: baseA.Revision,
		Base:             baseA,
	}
	csB := ChangeSet{ // stale and conflicting (same-field title)
		Project:          projectFromDocument(projB, cloneEdit(baseB, func(tk *models.Task) { tk.Title = "local b" })),
		ExpectedRevision: baseB.Revision,
		Base:             baseB,
	}

	_, err := env.svc.CommitMulti([]ChangeSet{csA, csB})
	if !errors.Is(err, ErrMergeConflict) {
		t.Fatalf("CommitMulti err = %v, want ErrMergeConflict", err)
	}

	// Project A's clean merge was discarded: its on-disk state is untouched.
	afterA, err := LoadProjectDocument(projA)
	if err != nil {
		t.Fatalf("reload A: %v", err)
	}
	if afterA.Revision != diskA.Revision {
		t.Fatalf("project A changed on an aborted multi-commit: got %q want %q", afterA.Revision, diskA.Revision)
	}
	if got := firstTaskTitle(afterA); got != "a title" {
		t.Fatalf("project A title = %q, want untouched %q", got, "a title")
	}

	// Project B is untouched too.
	afterB, err := LoadProjectDocument(projB)
	if err != nil {
		t.Fatalf("reload B: %v", err)
	}
	if afterB.Revision != diskB.Revision {
		t.Fatalf("project B changed on an aborted multi-commit: got %q want %q", afterB.Revision, diskB.Revision)
	}

	// Only the conflicting project recorded a bundle.
	records, err := env.store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("expected exactly one recorded conflict, got %d (err %v)", len(records), err)
	}
	if records[0].ProjectID != projB.ID {
		t.Fatalf("recorded conflict project = %q, want %q", records[0].ProjectID, projB.ID)
	}
}

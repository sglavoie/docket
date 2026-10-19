package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/sglavoie/tt/internal/models"
)

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

// mergeTestDocAdding returns doc with one extra task appended, which is what a
// creation is at this layer: the loaded document plus a task minted against it.
// cloneEdit only reaches Tasks[0], so a creation race needs its own spelling.
func mergeTestDocAdding(doc *ProjectDocument, id, title string, shortID int) *ProjectDocument {
	copied := *doc
	copied.Tasks = append(cloneDocTasks(doc.Tasks), mergeTestTask(id, title))
	copied.Tasks[len(copied.Tasks)-1].ShortID = shortID
	copied.Revision = copied.ComputeRevision()
	return &copied
}

// TestCommitMerge_ConcurrentCreationsRecordAConflictAndSucceedOnRetry covers the
// one shape a textual merge resolves cleanly and the canonical writer then
// refuses: two machines creating a task at once. Each mints the next free short
// id against a document that has not seen the other's task, so both new tasks are
// non-overlapping additions the field-wise merge keeps — and both answer to the
// same number, which is the reference every `#N` spelling resolves through.
//
// The claim is that this is classified rather than merely refused. Before the
// merge audit knew about short ids the commit boundary failed while rendering the
// merged document: nothing was written and no update was lost, but the loser got
// a bare error naming no conflict bundle, so there was nothing for `tt conflict`
// to show and nothing to resolve. It is now the ordinary retry-only conflict, and
// the retry — which reads the document the winner left — is what actually
// completes the losing machine's creation.
func TestCommitMerge_ConcurrentCreationsRecordAConflictAndSucceedOnRetry(t *testing.T) {
	env := newMergeCommitEnv(t)
	project := env.project("p1", "Alpha", "alpha")

	base := env.commitDoc(t, project, mergeTestDoc("Alpha", mergeTestTask("t1", "the anchor")), "", nil)

	// The remote machine creates first and lands. #2 is the next free short id in
	// the document both machines read.
	remote := env.commitDoc(t, project, mergeTestDocAdding(base, "t2", "remote creation", 2), base.Revision, base)

	// The local machine, still holding the pre-race base, mints the same #2.
	local := projectFromDocument(project, mergeTestDocAdding(base, "t3", "local creation", 2))
	_, err := env.svc.Commit(ChangeSet{
		Project:          local,
		ExpectedRevision: base.Revision,
		Base:             base,
		UndoAction:       "Add Task",
		UndoContext:      "local creation",
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

	// The bundle names the invariant rather than an edited field, which is what
	// separates "two machines edited one title" from "the merged whole is invalid".
	records, err := env.store.List()
	if err != nil || len(records) != 1 {
		t.Fatalf("expected exactly one durable conflict record, got %d (err %v)", len(records), err)
	}
	if records[0].Action != "Add Task" {
		t.Fatalf("recorded conflict action = %q, want the mutation's own %q", records[0].Action, "Add Task")
	}
	classified := false
	for _, recorded := range records[0].Conflicts {
		if recorded.Kind == MergeConflictInvariant && recorded.Field == "short_id" {
			classified = true
		}
	}
	if !classified {
		t.Fatalf("recorded conflict does not name the short id invariant: %+v", records[0].Conflicts)
	}

	// No partial task data: the document is byte-for-byte the winner's, and the
	// losing machine's task is nowhere in it.
	after, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload after the refused creation: %v", err)
	}
	if after.Revision != remote.Revision {
		t.Fatalf("on-disk revision moved on a refused creation: got %q want %q", after.Revision, remote.Revision)
	}
	for _, task := range after.Tasks {
		if task.Title == "local creation" {
			t.Fatal("the refused creation wrote its task anyway")
		}
	}

	// The retry is the point of a retry-only bundle. It reads what the winner left
	// and mints against that, so the short id it takes is free.
	retried := projectFromDocument(project, mergeTestDocAdding(after, "t3", "local creation", 3))
	if _, err = env.svc.Commit(ChangeSet{
		Project:          retried,
		ExpectedRevision: after.Revision,
		Base:             after,
		UndoAction:       "Add Task",
		UndoContext:      "local creation",
	}); err != nil {
		t.Fatalf("retrying the refused creation: %v", err)
	}

	settled, err := LoadProjectDocument(project)
	if err != nil {
		t.Fatalf("reload after the retry: %v", err)
	}
	shortIDs := map[int]string{}
	titles := map[string]bool{}
	for _, task := range settled.Tasks {
		if previous, clash := shortIDs[task.ShortID]; clash {
			t.Fatalf("tasks %q and %q both answer to #%d after the retry", previous, task.Title, task.ShortID)
		}
		shortIDs[task.ShortID] = task.Title
		titles[task.Title] = true
	}
	if !titles["remote creation"] || !titles["local creation"] {
		t.Fatalf("both creations should survive the retry, got %v", titles)
	}
}

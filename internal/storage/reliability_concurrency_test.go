package storage

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

// Session 22 reliability gate: prove the commit boundary is safe under true
// concurrency (goroutines, exercised under `go test -race`), that revision
// threading turns concurrent read-modify-write into detected conflicts rather
// than silent lost updates, that an out-of-band external edit is detected, and
// that a normal commit drives crash recovery of an interrupted transaction. The
// existing commit_test.go cases cover the same guarantees only with sequential
// commits; these add the concurrent and integration coverage the milestone gate
// requires.

// reliabilityProject builds a canonical <data>/projects/<slug> project holding
// the given tasks, ready to commit.
func reliabilityProject(t *testing.T, dataDir, slug string, tasks ...*models.Task) *models.Project {
	t.Helper()
	p, err := models.NewProject(uuid.New().String(), slug, "")
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	p.Slug = slug
	p.Path = fmt.Sprintf("%s/projects/%s", dataDir, slug)
	p.Tasks = tasks
	p.NextShortID = len(tasks) + 1
	p.BuildTaskTree()
	return p
}

func reliabilityTask(t *testing.T, shortID int, title string) *models.Task {
	t.Helper()
	task, err := models.NewGenericTask(uuid.New().String(), shortID, title)
	if err != nil {
		t.Fatalf("NewTask: %v", err)
	}
	return task
}

// TestReliability_ConcurrentCommitsNeverCorruptSharedFile runs many goroutines
// committing to the same project's tasks.md at once with no expected revision
// (last-write-wins). The journal's per-project file lock serializes every atomic
// replace, so the file must always parse and always hold exactly one writer's
// complete task set — never a torn or interleaved write. Run under -race this
// also proves the commit boundary has no data race for concurrent callers.
func TestReliability_ConcurrentCommitsNeverCorruptSharedFile(t *testing.T) {
	svc, dataDir := newCommitTestService(t)

	// Seed the project so its directory and an initial tasks.md exist.
	seed := newCommitTestProject(t, dataDir, "conc", "seed")
	if _, err := svc.Commit(ChangeSet{Project: seed}); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}

	const workers = 16
	var wg sync.WaitGroup
	commitErrs := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Each worker writes its own single-task view of the file. Whichever
			// commit lands last wins the whole file; none may corrupt it.
			p := reliabilityProject(t, dataDir, "conc", reliabilityTask(t, 2, fmt.Sprintf("worker-%d", i)))
			_, commitErrs[i] = svc.Commit(ChangeSet{Project: p})
		}(i)
	}
	wg.Wait()

	for i, err := range commitErrs {
		if err != nil {
			t.Fatalf("worker %d commit failed (writes must serialize, not error): %v", i, err)
		}
	}

	// The final file is a valid, fully-formed document from exactly one winner.
	content, err := os.ReadFile(tasksFilePath(seed.Path))
	if err != nil {
		t.Fatalf("read committed tasks file: %v", err)
	}
	doc, err := ParseProjectDocument(string(content))
	if err != nil {
		t.Fatalf("committed file must always parse (no torn write): %v", err)
	}
	if len(doc.Tasks) != 1 {
		t.Fatalf("expected exactly one winner's task set, got %d tasks", len(doc.Tasks))
	}
	if !strings.HasPrefix(doc.Tasks[0].Title, "worker-") {
		t.Fatalf("winning task title = %q, want a worker-* title", doc.Tasks[0].Title)
	}
}

// TestReliability_ConcurrentRevisionCheckedCommitsPreventLostUpdates proves the
// milestone's core "no silent lost update" guarantee at the commit boundary:
// when every concurrent writer threads the revision it loaded, exactly one
// commit succeeds and every other is rejected with a RevisionConflictError
// (unwrapping to ErrStaleRevision) instead of silently clobbering the winner.
func TestReliability_ConcurrentRevisionCheckedCommitsPreventLostUpdates(t *testing.T) {
	svc, dataDir := newCommitTestService(t)

	base := newCommitTestProject(t, dataDir, "rev", "base")
	if _, err := svc.Commit(ChangeSet{Project: base}); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}
	baseRev := onDiskRevision(t, base)

	const workers = 16
	var wg sync.WaitGroup
	results := make([]error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// Every worker loaded the same base revision and now tries to append
			// its own task on top of it.
			p := reliabilityProject(t, dataDir, "rev",
				reliabilityTask(t, 1, "base"),
				reliabilityTask(t, 2, fmt.Sprintf("worker-%d", i)),
			)
			_, results[i] = svc.Commit(ChangeSet{Project: p, ExpectedRevision: baseRev})
		}(i)
	}
	wg.Wait()

	successes := 0
	for i, err := range results {
		if err == nil {
			successes++
			continue
		}
		if !errors.Is(err, ErrStaleRevision) {
			t.Errorf("worker %d: got %v, want an ErrStaleRevision conflict", i, err)
		}
		var rce *RevisionConflictError
		if !errors.As(err, &rce) {
			t.Errorf("worker %d: got %T, want *RevisionConflictError", i, err)
		}
	}
	if successes != 1 {
		t.Fatalf("exactly one revision-checked commit must win; got %d successes", successes)
	}

	// The winner landed a valid two-task document; nothing was lost or corrupted.
	doc, err := ParseProjectDocument(mustReadFileString(t, tasksFilePath(base.Path)))
	if err != nil {
		t.Fatalf("committed file must parse: %v", err)
	}
	if len(doc.Tasks) != 2 {
		t.Fatalf("winner's document should have 2 tasks, got %d", len(doc.Tasks))
	}
}

// TestReliability_ExternalFileEditDetectedByRevisionCheck models a user (or a
// cloud sync client) editing tasks.md out of band, outside the transaction
// journal, between the moment a caller loaded the project and the moment it
// commits. A revision-checked commit must detect the external change and abort
// with a RevisionConflictError rather than clobbering the external edit.
func TestReliability_ExternalFileEditDetectedByRevisionCheck(t *testing.T) {
	svc, dataDir := newCommitTestService(t)

	p := newCommitTestProject(t, dataDir, "ext", "original")
	if _, err := svc.Commit(ChangeSet{Project: p}); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}
	loadedRev := onDiskRevision(t, p)

	// An external editor rewrites tasks.md directly on disk, bypassing the journal.
	edited := reliabilityProject(t, dataDir, "ext",
		reliabilityTask(t, 1, "original"),
		reliabilityTask(t, 2, "added by external editor"),
	)
	externalContent := mustWriteCurrentProject(t, edited)
	if err := os.WriteFile(tasksFilePath(p.Path), []byte(externalContent), FilePermissions); err != nil {
		t.Fatalf("simulate external edit: %v", err)
	}
	externalRev := onDiskRevision(t, p)
	if externalRev == loadedRev {
		t.Fatal("external edit should change the on-disk revision")
	}

	// Committing our stale-based change must be refused, not applied over the edit.
	p.Tasks[0].Title = "my conflicting local change"
	_, err := svc.Commit(ChangeSet{Project: p, ExpectedRevision: loadedRev})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("got %v, want ErrStaleRevision for an external edit", err)
	}
	var rce *RevisionConflictError
	if !errors.As(err, &rce) {
		t.Fatalf("got %T, want *RevisionConflictError", err)
	}
	if rce.Actual != externalRev {
		t.Fatalf("conflict Actual = %q, want the external revision %q", rce.Actual, externalRev)
	}

	// The external edit survives untouched.
	doc, err := ParseProjectDocument(mustReadFileString(t, tasksFilePath(p.Path)))
	if err != nil {
		t.Fatalf("parse after refused commit: %v", err)
	}
	if len(doc.Tasks) != 2 {
		t.Fatalf("external edit must survive; got %d tasks", len(doc.Tasks))
	}
}

// TestReliability_CommitDrivesCrashRecoveryRollForward proves the commit
// boundary itself recovers an interrupted transaction: a prepared-but-unapplied
// transaction left by a crash is rolled forward by the recovery pass that every
// Commit runs before staging, so a later unrelated commit both completes the
// stranded work and lands its own change.
func TestReliability_CommitDrivesCrashRecoveryRollForward(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	transactionsDir := dataDir + "/.docket/transactions"

	// Simulate a crash after a transaction staged a write to project "beta" and
	// durably wrote its manifest, but before it applied. The project directory
	// exists (it was created before the crash).
	betaPath := dataDir + "/projects/beta"
	if err := os.MkdirAll(betaPath, DirPermissions); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	betaTasks := tasksFilePath(betaPath)
	stranded := reliabilityProject(t, dataDir, "beta", reliabilityTask(t, 1, "recovered work"))
	strandedContent := mustWriteCurrentProject(t, stranded)
	journal := transaction.NewJournal(transactionsDir, []string{dataDir}, "")
	if _, err := journal.Prepare([]transaction.Op{{
		Kind:   transaction.OpWrite,
		Target: betaTasks,
		Data:   []byte(strandedContent),
		Mode:   FilePermissions,
	}}); err != nil {
		t.Fatalf("prepare stranded transaction: %v", err)
	}
	// Deliberately do NOT commit the prepared transaction: this is the crash.

	// A normal, unrelated commit runs recovery first and rolls the stranded
	// transaction forward before staging its own change.
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha task")
	if _, err := svc.Commit(ChangeSet{Project: alpha}); err != nil {
		t.Fatalf("commit after crash must succeed (recovery rolls forward): %v", err)
	}

	// The stranded write completed.
	betaDoc, err := ParseProjectDocument(mustReadFileString(t, betaTasks))
	if err != nil {
		t.Fatalf("recovered beta file must parse: %v", err)
	}
	if len(betaDoc.Tasks) != 1 || betaDoc.Tasks[0].Title != "recovered work" {
		t.Fatalf("beta not rolled forward: %+v", betaDoc.Tasks)
	}

	// The new commit landed too.
	alphaDoc, err := ParseProjectDocument(mustReadFileString(t, tasksFilePath(alpha.Path)))
	if err != nil {
		t.Fatalf("alpha file must parse: %v", err)
	}
	if len(alphaDoc.Tasks) != 1 || alphaDoc.Tasks[0].Title != "alpha task" {
		t.Fatalf("alpha not committed: %+v", alphaDoc.Tasks)
	}
}

func mustReadFileString(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

// strptr returns a pointer to s, for building ExpectedRevision contracts inline.
func strptr(s string) *string { return &s }

// TestArtifactContainmentRejectsBeforeFilesystemSideEffects exercises every
// artifact-bearing commit entry point. An out-of-root artifact must be rejected
// before the workspace lock, project directory, artifact parent, adjacent lock,
// staging file, or manifest can be created.
func TestArtifactContainmentRejectsBeforeFilesystemSideEffects(t *testing.T) {
	tests := []struct {
		name   string
		commit func(*testing.T, *CommitService, string, ArtifactChange) error
	}{
		{
			name: "project commit",
			commit: func(t *testing.T, svc *CommitService, root string, artifact ArtifactChange) error {
				t.Helper()
				project := newCommitTestProject(t, root, "alpha", "task")
				_, err := svc.Commit(ChangeSet{Project: project, Artifacts: []ArtifactChange{artifact}})
				return err
			},
		},
		{
			name: "lifecycle commit",
			commit: func(_ *testing.T, svc *CommitService, _ string, artifact ArtifactChange) error {
				return svc.CommitOpsWithArtifacts(nil, []ArtifactChange{artifact})
			},
		},
		{
			name: "artifact-only commit",
			commit: func(_ *testing.T, svc *CommitService, _ string, artifact ArtifactChange) error {
				_, err := svc.CommitArtifacts([]ArtifactChange{artifact})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "workspace")
			transactionsDir := filepath.Join(root, ".docket", "transactions")
			svc := NewCommitService(transactionsDir, []string{root})
			outsideBase := t.TempDir()
			outsideParent := filepath.Join(outsideBase, "must-not-exist", "nested")
			artifactPath := filepath.Join(outsideParent, "planning.json")

			err := tc.commit(t, svc, root, ArtifactChange{Path: artifactPath, Data: []byte("nope")})
			if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
				t.Fatalf("commit err = %v, want ErrTargetOutsideRoot", err)
			}
			for _, path := range []string{root, transactionsDir, outsideParent, artifactPath, artifactPath + ".lock"} {
				if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
					t.Fatalf("rejected artifact created %q: stat err = %v", path, statErr)
				}
			}
		})
	}
}

// TestArtifactContainmentRejectsSymlinkEscapeBeforeLockCreation proves the
// read-only preflight resolves existing symlinks before MkdirAll or WithFileLock
// can follow an escaping parent and create external directories or lock files.
func TestArtifactContainmentRejectsSymlinkEscapeBeforeLockCreation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}
	root := t.TempDir()
	outside := t.TempDir()
	escape := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	transactionsDir := filepath.Join(root, ".docket", "transactions")
	artifactPath := filepath.Join(escape, "must-not-exist", "planning.json")
	svc := NewCommitService(transactionsDir, []string{root})
	_, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("nope")}})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitArtifacts err = %v, want ErrTargetOutsideRoot", err)
	}
	for _, path := range []string{
		transactionsDir,
		filepath.Join(outside, "must-not-exist"),
		filepath.Join(outside, "must-not-exist", "planning.json.lock"),
	} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("symlink-escaping artifact created %q: stat err = %v", path, statErr)
		}
	}
}

// TestArtifactLockContainmentRejectsEscapingAdjacentLock covers the edge case
// where the artifact target itself equals an approved root but its adjacent
// "<artifact>.lock" path is a sibling outside that root.
func TestArtifactLockContainmentRejectsEscapingAdjacentLock(t *testing.T) {
	root := t.TempDir()
	transactionsDir := filepath.Join(root, ".docket", "transactions")
	lockPath := root + ".lock"
	svc := NewCommitService(transactionsDir, []string{root})

	_, err := svc.CommitArtifacts([]ArtifactChange{{Path: root, Data: []byte("nope")}})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitArtifacts err = %v, want ErrTargetOutsideRoot", err)
	}
	for _, path := range []string{transactionsDir, lockPath} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("rejected artifact lock created %q: stat err = %v", path, statErr)
		}
	}
}

// TestArtifactLockContainmentRejectsSymlinkEscape proves the adjacent lock path
// receives its own symlink-aware containment check even when the artifact target
// itself is an ordinary path inside the workspace.
func TestArtifactLockContainmentRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}
	root := t.TempDir()
	outside := t.TempDir()
	artifactPath := filepath.Join(root, "planning.json")
	lockPath := artifactPath + ".lock"
	externalLock := filepath.Join(outside, "external.lock")
	if err := os.WriteFile(externalLock, []byte("outside sentinel"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalLock, lockPath); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	transactionsDir := filepath.Join(root, ".docket", "transactions")
	svc := NewCommitService(transactionsDir, []string{root})

	_, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("nope")}})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitArtifacts err = %v, want ErrTargetOutsideRoot", err)
	}
	if _, statErr := os.Lstat(transactionsDir); !os.IsNotExist(statErr) {
		t.Fatalf("rejected lock created transactions directory: stat err = %v", statErr)
	}
	if _, statErr := os.Lstat(artifactPath); !os.IsNotExist(statErr) {
		t.Fatalf("rejected lock allowed artifact write: stat err = %v", statErr)
	}
	if got, readErr := os.ReadFile(externalLock); readErr != nil || string(got) != "outside sentinel" {
		t.Fatalf("external lock target changed: data = %q err = %v", got, readErr)
	}
}

// TestArtifactChange_ExpectedRevisionMatchCommits confirms that a project commit
// carrying an artifact whose ExpectedRevision matches the current on-disk raw hash
// is applied normally.
func TestArtifactChange_ExpectedRevisionMatchCommits(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "task")
	artifactPath := filepath.Join(dataDir, ".docket", "planning.json")

	// Seed the artifact so it has a known revision.
	seed := []byte(`{"queue":["a"]}`)
	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: seed}}); err != nil {
		t.Fatalf("seed CommitArtifacts: %v", err)
	}
	rev := RawArtifactRevision(seed)

	next := []byte(`{"queue":["a","b"]}`)
	if _, err := svc.Commit(ChangeSet{
		Project:   project,
		Artifacts: []ArtifactChange{{Path: artifactPath, Data: next, ExpectedRevision: strptr(rev)}},
	}); err != nil {
		t.Fatalf("Commit with matching artifact revision: %v", err)
	}
	got, err := os.ReadFile(artifactPath)
	if err != nil || string(got) != string(next) {
		t.Fatalf("artifact = %q err = %v, want %q", got, err, next)
	}
}

// TestArtifactChange_StaleExpectedRevisionAbortsAll proves a non-empty
// ExpectedRevision that no longer matches the on-disk raw hash aborts the whole
// commit with an *ArtifactConflictError before the project or the artifact is
// written — a concurrent config writer's change is never silently overwritten.
func TestArtifactChange_StaleExpectedRevisionAbortsAll(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	project := newCommitTestProject(t, dataDir, "alpha", "task")
	artifactPath := filepath.Join(dataDir, ".docket", "planning.json")

	current := []byte(`{"queue":["current"]}`)
	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: current}}); err != nil {
		t.Fatalf("seed CommitArtifacts: %v", err)
	}

	_, err := svc.Commit(ChangeSet{
		Project:   project,
		Artifacts: []ArtifactChange{{Path: artifactPath, Data: []byte("clobber"), ExpectedRevision: strptr(RawArtifactRevision([]byte("stale")))}},
	})
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("expected ErrStaleArtifact, got %v", err)
	}
	var conflict *ArtifactConflictError
	if !errors.As(err, &conflict) {
		t.Fatalf("expected *ArtifactConflictError, got %T", err)
	}
	if conflict.Path != artifactPath {
		t.Fatalf("conflict path = %q, want %q", conflict.Path, artifactPath)
	}
	// Neither the artifact nor the project was written.
	if got, _ := os.ReadFile(artifactPath); string(got) != string(current) {
		t.Fatalf("artifact overwritten despite conflict: %q", got)
	}
	if commitProjectExists(project) {
		t.Fatalf("project written despite artifact conflict")
	}
	assertJournalClean(t, dataDir)
}

// TestArtifactChange_ExpectedAbsenceContract exercises the empty-string
// ExpectedRevision: it requires the artifact to be absent. A create against an
// absent file succeeds; a create against an existing file aborts as a conflict.
func TestArtifactChange_ExpectedAbsenceContract(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	artifactPath := filepath.Join(dataDir, ".docket", "settings.json")

	// Absent → require-absent create succeeds.
	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("first"), ExpectedRevision: strptr("")}}); err != nil {
		t.Fatalf("require-absent create against absent file: %v", err)
	}

	// Present → require-absent create conflicts.
	_, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("second"), ExpectedRevision: strptr("")}})
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("expected ErrStaleArtifact for require-absent against existing file, got %v", err)
	}
	if got, _ := os.ReadFile(artifactPath); string(got) != "first" {
		t.Fatalf("artifact overwritten despite absence conflict: %q", got)
	}
}

// TestArtifactChange_NilExpectedRevisionIsUnchecked confirms a nil ExpectedRevision
// opts out of the concurrency check entirely (last-writer-wins), preserving the
// pre-coordinator behavior for callers that do not track a load revision.
func TestArtifactChange_NilExpectedRevisionIsUnchecked(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	artifactPath := filepath.Join(dataDir, ".docket", "automation.json")

	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("v1")}}); err != nil {
		t.Fatalf("first unchecked write: %v", err)
	}
	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: artifactPath, Data: []byte("v2")}}); err != nil {
		t.Fatalf("second unchecked write should clobber freely: %v", err)
	}
	if got, _ := os.ReadFile(artifactPath); string(got) != "v2" {
		t.Fatalf("unchecked write did not clobber: %q", got)
	}
}

// TestCommitArtifacts_WritesAndDeletesAtomically confirms the config-only commit
// service applies a mix of writes and a delete in one transaction and reclaims the
// journal.
func TestCommitArtifacts_WritesAndDeletesAtomically(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	catalog := filepath.Join(dataDir, ".docket", "workspace.json")
	planning := filepath.Join(dataDir, ".docket", "planning.json")
	doomed := filepath.Join(dataDir, ".docket", "obsolete.json")

	if err := os.MkdirAll(filepath.Dir(doomed), DirPermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(doomed, []byte("remove me"), 0o600); err != nil {
		t.Fatal(err)
	}

	txnID, err := svc.CommitArtifacts([]ArtifactChange{
		{Path: catalog, Data: []byte(`{"projects":[]}`)},
		{Path: planning, Data: []byte(`{"queue":[]}`)},
		{Path: doomed, Delete: true},
	})
	if err != nil {
		t.Fatalf("CommitArtifacts: %v", err)
	}
	if txnID == "" {
		t.Fatalf("expected a transaction id")
	}
	if got, _ := os.ReadFile(catalog); string(got) != `{"projects":[]}` {
		t.Fatalf("catalog = %q", got)
	}
	if got, _ := os.ReadFile(planning); string(got) != `{"queue":[]}` {
		t.Fatalf("planning = %q", got)
	}
	if _, err := os.Stat(doomed); !os.IsNotExist(err) {
		t.Fatalf("doomed artifact not deleted: stat err = %v", err)
	}
	assertJournalClean(t, dataDir)
}

// TestCommitArtifacts_RecoversAfterInterruption proves journal-backed all-or-
// nothing recovery for a config-only commit: a transaction prepared but never
// committed (a crash between Prepare and Commit) is rolled forward by a later
// process's recovery so every artifact lands together.
func TestCommitArtifacts_RecoversAfterInterruption(t *testing.T) {
	_, dataDir := newCommitTestService(t)
	txnDir := filepath.Join(dataDir, ".docket", "transactions")
	catalog := filepath.Join(dataDir, ".docket", "workspace.json")
	planning := filepath.Join(dataDir, ".docket", "planning.json")

	journal := transaction.NewJournal(txnDir, []string{dataDir}, "")
	if _, err := journal.Prepare([]transaction.Op{
		{Kind: transaction.OpWrite, Target: catalog, Data: []byte(`{"projects":["p"]}`), Mode: FilePermissions},
		{Kind: transaction.OpWrite, Target: planning, Data: []byte(`{"queue":["q"]}`), Mode: FilePermissions},
	}); err != nil {
		t.Fatalf("Prepare: %v", err)
	}

	// Simulate a crash: manifest prepared, never committed. A fresh journal recovers.
	report, err := transaction.NewJournal(txnDir, []string{dataDir}, "").Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want exactly one", report.RolledForward)
	}
	if got, _ := os.ReadFile(catalog); string(got) != `{"projects":["p"]}` {
		t.Fatalf("catalog after recovery = %q", got)
	}
	if got, _ := os.ReadFile(planning); string(got) != `{"queue":["q"]}` {
		t.Fatalf("planning after recovery = %q", got)
	}
	assertJournalClean(t, dataDir)
}

// TestConcurrentUpdatesToDifferentProjects confirms two committers mutating
// different projects both succeed without loss: the workspace commit lock
// serializes their pipelines while each writes its own project's tasks.md.
func TestConcurrentUpdatesToDifferentProjects(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha task")
	beta := newCommitTestProject(t, dataDir, "beta", "beta task")

	var wg sync.WaitGroup
	errs := make([]error, 2)
	commit := func(idx int, project *models.Project) {
		defer wg.Done()
		_, errs[idx] = svc.Commit(ChangeSet{Project: project})
	}
	wg.Add(2)
	go commit(0, alpha)
	go commit(1, beta)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent commit %d failed: %v", i, err)
		}
	}
	if !commitProjectExists(alpha) || !commitProjectExists(beta) {
		t.Fatalf("expected both projects committed: alpha=%v beta=%v", commitProjectExists(alpha), commitProjectExists(beta))
	}
	assertJournalClean(t, dataDir)
}

// TestConcurrentUpdatesToSameDomainNoLostUpdate proves that two committers racing
// to update the SAME shared config domain from the same baseline revision cannot
// silently overwrite each other: exactly one commit lands, the other is rejected
// with an *ArtifactConflictError, and the on-disk content is the winner's.
func TestConcurrentUpdatesToSameDomainNoLostUpdate(t *testing.T) {
	svc, dataDir := newCommitTestService(t)
	planning := filepath.Join(dataDir, ".docket", "planning.json")

	base := []byte(`{"queue":["base"]}`)
	if _, err := svc.CommitArtifacts([]ArtifactChange{{Path: planning, Data: base}}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	baseRev := RawArtifactRevision(base)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	payloads := [][]byte{[]byte(`{"queue":["a"]}`), []byte(`{"queue":["b"]}`)}
	commit := func(idx int) {
		defer wg.Done()
		_, errs[idx] = svc.CommitArtifacts([]ArtifactChange{{Path: planning, Data: payloads[idx], ExpectedRevision: strptr(baseRev)}})
	}
	wg.Add(2)
	go commit(0)
	go commit(1)
	wg.Wait()

	successes, conflicts, winner := 0, 0, -1
	for i, err := range errs {
		switch {
		case err == nil:
			successes++
			winner = i
		case errors.Is(err, ErrStaleArtifact):
			conflicts++
		default:
			t.Fatalf("commit %d got unexpected error: %v", i, err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("expected exactly one success and one conflict, got %d success / %d conflict", successes, conflicts)
	}
	got, _ := os.ReadFile(planning)
	if string(got) != string(payloads[winner]) {
		t.Fatalf("on-disk content = %q, want winner's %q", got, payloads[winner])
	}
	assertJournalClean(t, dataDir)
}

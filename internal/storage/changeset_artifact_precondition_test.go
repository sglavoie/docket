package storage

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/transaction"
)

func writeGuardedArtifact(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), DirPermissions); err != nil {
		t.Fatalf("create guarded artifact directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), FilePermissions); err != nil {
		t.Fatalf("write guarded artifact: %v", err)
	}
	return RawArtifactRevision([]byte(content))
}

func readGuardedArtifact(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read guarded artifact: %v", err)
	}
	return string(data)
}

// TestCommitMultiArtifactPreconditionExpiresBeforeMergeOrConflictRecording is the
// catalog-race contract a whole-workspace census depends on: the changeset's
// project revision is stale and would otherwise three-way merge, but the stale
// catalog guard expires the mutation first, so no merge happens and no conflict
// bundle is recorded for a read set that was never valid.
func TestCommitMultiArtifactPreconditionExpiresBeforeMergeOrConflictRecording(t *testing.T) {
	env := newMergeCommitEnv(t)
	alpha := env.project("alpha-id", "Alpha", "alpha")
	base := env.commitDoc(t, alpha, mergeTestDoc("Alpha", mergeTestTask("alpha-task", "alpha base")), "", nil)

	catalogPath := filepath.Join(env.dataDir, ".tt", "workspace.json")
	censusRevision := writeGuardedArtifact(t, catalogPath, `{"version":17,"projects":["alpha"]}`)

	remote := cloneEdit(base, func(task *models.Task) { task.Priority = models.PriorityMin })
	current := env.commitDoc(t, alpha, remote, base.Revision, base)
	writeGuardedArtifact(t, catalogPath, `{"version":17,"projects":["alpha","beta"]}`)

	localDoc := cloneEdit(base, func(task *models.Task) { task.Title = "alpha local" })
	_, err := env.svc.Commit(ChangeSet{
		Project:          projectFromDocument(alpha, localDoc),
		ExpectedRevision: base.Revision,
		Base:             base,
		ArtifactPreconditions: []ArtifactPrecondition{{
			Path:             catalogPath,
			ExpectedRevision: censusRevision,
		}},
	})
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("Commit error = %v, want ErrStaleArtifact", err)
	}
	var conflict *ArtifactConflictError
	if !errors.As(err, &conflict) || conflict.Path != catalogPath || conflict.Expected != censusRevision {
		t.Fatalf("artifact conflict = %#v, want catalog path and census revision", conflict)
	}
	if records, listErr := env.store.List(); listErr != nil || len(records) != 0 {
		t.Fatalf("failed artifact precondition recorded conflict bundles: records=%v err=%v", records, listErr)
	}
	alphaDisk, loadErr := LoadProjectDocument(alpha)
	if loadErr != nil {
		t.Fatalf("LoadProjectDocument alpha: %v", loadErr)
	}
	if got := firstTaskTitle(alphaDisk); got != "alpha base" {
		t.Fatalf("alpha title after failed artifact precondition = %q, want the untouched remote title", got)
	}
	if alphaDisk.Revision != current.Revision {
		t.Fatalf("alpha revision after failed artifact precondition = %q, want %q", alphaDisk.Revision, current.Revision)
	}
	assertJournalClean(t, env.dataDir)
}

// TestCommitMultiArtifactPreconditionIsReadOnlyAndExcludedFromUndo proves the
// guard never becomes a write: the guarded bytes survive the commit untouched,
// the undo ledger image names only the project the changeset actually wrote, and
// undoing that entry leaves the guarded artifact alone.
func TestCommitMultiArtifactPreconditionIsReadOnlyAndExcludedFromUndo(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	if _, err := NewCommitService(transactionsDir, []string{dataDir}).Commit(ChangeSet{Project: alpha}); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}
	alphaBase, err := LoadProjectDocument(alpha)
	if err != nil {
		t.Fatalf("LoadProjectDocument alpha: %v", err)
	}

	catalogPath := filepath.Join(dataDir, ".tt", "workspace.json")
	const catalogContent = `{"version":17,"projects":["alpha"]}`
	censusRevision := writeGuardedArtifact(t, catalogPath, catalogContent)

	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(transactionsDir, []string{dataDir}, ledger)
	localDoc := cloneEdit(alphaBase, func(task *models.Task) { task.Title = "alpha after" })
	if _, commitErr := service.Commit(ChangeSet{
		Project:          projectFromDocument(alpha, localDoc),
		ExpectedRevision: alphaBase.Revision,
		Base:             alphaBase,
		ArtifactPreconditions: []ArtifactPrecondition{{
			Path:             catalogPath,
			ExpectedRevision: censusRevision,
		}},
		UndoAction: "guarded census edit",
	}); commitErr != nil {
		t.Fatalf("Commit guarded edit: %v", commitErr)
	}
	if got := readGuardedArtifact(t, catalogPath); got != catalogContent {
		t.Fatalf("guarded artifact after commit = %q, want it unchanged", got)
	}

	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo ok=%v err=%v", ok, err)
	}
	if len(entry.Artifacts) != 1 || entry.Artifacts[0].Path != tasksFilePath(alpha.Path) {
		t.Fatalf("ledger artifacts = %+v, want only alpha tasks.md", entry.Artifacts)
	}
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := onDiskRevision(t, alpha); got != alphaBase.Revision {
		t.Fatalf("alpha revision after undo = %q, want %q", got, alphaBase.Revision)
	}
	if got := readGuardedArtifact(t, catalogPath); got != catalogContent {
		t.Fatalf("guarded artifact after undo = %q, want it unchanged", got)
	}
}

// TestCommitMultiRejectsArtifactPreconditionPathBeforeEffects holds the guard to
// the same containment rule as a write: an escaping path is refused before the
// workspace lock creates the transactions directory or any lock file.
func TestCommitMultiRejectsArtifactPreconditionPathBeforeEffects(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	project := newCommitTestProject(t, dataDir, "alpha", "alpha")
	outside := filepath.Join(t.TempDir(), "escaped-catalog.json")

	_, err := NewCommitService(transactionsDir, []string{dataDir}).Commit(ChangeSet{
		Project:               project,
		ArtifactPreconditions: []ArtifactPrecondition{{Path: outside, ExpectedRevision: "deadbeef"}},
	})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("Commit error = %v, want ErrTargetOutsideRoot", err)
	}
	for _, path := range []string{project.Path, transactionsDir, outside, outside + ".lock"} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("rejected artifact precondition left filesystem effect at %s: %v", path, statErr)
		}
	}
}

// TestCommitMultiDeduplicatesResolvedArtifactPreconditionLocks covers the lock
// set rather than the guard: a census guards the very catalog its own commit
// rewrites, and may reach it through a symlinked alias, so the guard and the
// write must resolve to one held lock instead of deadlocking on two.
func TestCommitMultiDeduplicatesResolvedArtifactPreconditionLocks(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	service := NewCommitService(transactionsDir, []string{dataDir})
	if _, err := service.Commit(ChangeSet{Project: alpha}); err != nil {
		t.Fatalf("seed Commit: %v", err)
	}
	alphaBase, err := LoadProjectDocument(alpha)
	if err != nil {
		t.Fatalf("LoadProjectDocument alpha: %v", err)
	}

	catalogPath := filepath.Join(dataDir, ".tt", "workspace.json")
	censusRevision := writeGuardedArtifact(t, catalogPath, `{"version":17,"projects":["alpha"]}`)
	alias := filepath.Join(dataDir, "tt-alias")
	if err := os.Symlink(filepath.Join(dataDir, ".tt"), alias); err != nil {
		t.Fatalf("symlink workspace alias: %v", err)
	}
	expected := censusRevision

	done := make(chan error, 1)
	go func() {
		localDoc := cloneEdit(alphaBase, func(task *models.Task) { task.Priority = models.PriorityMin })
		_, commitErr := service.Commit(ChangeSet{
			Project:          projectFromDocument(alpha, localDoc),
			ExpectedRevision: alphaBase.Revision,
			Base:             alphaBase,
			Artifacts: []ArtifactChange{{
				Path:             catalogPath,
				Data:             []byte(`{"version":17,"projects":["alpha","beta"]}`),
				ExpectedRevision: &expected,
			}},
			ArtifactPreconditions: []ArtifactPrecondition{
				{Path: catalogPath, ExpectedRevision: censusRevision},
				{Path: filepath.Join(alias, "workspace.json"), ExpectedRevision: censusRevision},
			},
		})
		done <- commitErr
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Commit with aliased artifact precondition locks: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Commit deadlocked while acquiring aliased artifact precondition locks")
	}
	if got := readGuardedArtifact(t, catalogPath); got != `{"version":17,"projects":["alpha","beta"]}` {
		t.Fatalf("catalog after guarded rewrite = %q, want the new bytes", got)
	}
}

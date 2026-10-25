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

func TestCommitMultiProjectPreconditionExpiresBeforeMergeOrConflictRecording(t *testing.T) {
	env := newMergeCommitEnv(t)
	alpha := env.project("alpha-id", "Alpha", "alpha")
	beta := env.project("beta-id", "Beta", "beta")
	alphaBase := env.commitDoc(t, alpha, mergeTestDoc("Alpha", mergeTestTask("alpha-task", "alpha base")), "", nil)
	betaBase := env.commitDoc(t, beta, mergeTestDoc("Beta", mergeTestTask("beta-task", "beta base")), "", nil)

	alphaRemote := cloneEdit(alphaBase, func(task *models.Task) { task.Title = "alpha remote" })
	env.commitDoc(t, alpha, alphaRemote, alphaBase.Revision, alphaBase)
	betaRemote := cloneEdit(betaBase, func(task *models.Task) { task.Priority = models.PriorityMin })
	betaCurrent := env.commitDoc(t, beta, betaRemote, betaBase.Revision, betaBase)

	alphaLocalDoc := cloneEdit(alphaBase, func(task *models.Task) { task.Title = "alpha local" })
	alphaLocal := projectFromDocument(alpha, alphaLocalDoc)
	_, err := env.svc.Commit(ChangeSet{
		Project:          alphaLocal,
		ExpectedRevision: alphaBase.Revision,
		Base:             alphaBase,
		ProjectPreconditions: []ProjectRevisionPrecondition{{
			ProjectID:        beta.ID,
			ProjectName:      beta.Name,
			Path:             beta.Path,
			ExpectedRevision: betaBase.Revision,
		}},
	})
	if !errors.Is(err, ErrStaleRevision) {
		t.Fatalf("Commit error = %v, want ErrStaleRevision", err)
	}
	var conflict *RevisionConflictError
	if !errors.As(err, &conflict) || conflict.ProjectID != beta.ID || conflict.Expected != betaBase.Revision || conflict.Actual != betaCurrent.Revision {
		t.Fatalf("revision conflict = %#v, want beta expected/actual revisions", conflict)
	}
	if records, listErr := env.store.List(); listErr != nil || len(records) != 0 {
		t.Fatalf("failed precondition recorded conflict bundles: records=%v err=%v", records, listErr)
	}
	alphaDisk, loadErr := LoadProjectDocument(alpha)
	if loadErr != nil {
		t.Fatalf("LoadProjectDocument alpha: %v", loadErr)
	}
	if got := firstTaskTitle(alphaDisk); got != "alpha remote" {
		t.Fatalf("alpha title after failed precondition = %q, want remote", got)
	}
	if got := onDiskRevision(t, beta); got != betaCurrent.Revision {
		t.Fatalf("beta revision after failed precondition = %q, want %q", got, betaCurrent.Revision)
	}
	assertJournalClean(t, env.dataDir)
}

func TestCommitMultiProjectPreconditionIsReadOnlyAndExcludedFromUndo(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	beta := newCommitTestProject(t, dataDir, "beta", "beta stable")
	seed := NewCommitService(transactionsDir, []string{dataDir})
	if _, err := seed.CommitMulti([]ChangeSet{{Project: alpha}, {Project: beta}}); err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}
	alphaBase, err := LoadProjectDocument(alpha)
	if err != nil {
		t.Fatalf("LoadProjectDocument alpha: %v", err)
	}
	betaBase, err := LoadProjectDocument(beta)
	if err != nil {
		t.Fatalf("LoadProjectDocument beta: %v", err)
	}

	ledger := NewWorkspaceLedger(filepath.Join(dataDir, ".tt", "undo"), []string{dataDir})
	service := NewCommitServiceWithLedger(transactionsDir, []string{dataDir}, ledger)
	alphaLocalDoc := cloneEdit(alphaBase, func(task *models.Task) { task.Title = "alpha after" })
	if _, commitErr := service.Commit(ChangeSet{
		Project:          projectFromDocument(alpha, alphaLocalDoc),
		ExpectedRevision: alphaBase.Revision,
		Base:             alphaBase,
		ProjectPreconditions: []ProjectRevisionPrecondition{{
			ProjectID:        beta.ID,
			ProjectName:      beta.Name,
			Path:             beta.Path,
			ExpectedRevision: betaBase.Revision,
		}},
		UndoAction: "guarded alpha edit",
	}); commitErr != nil {
		t.Fatalf("Commit guarded edit: %v", commitErr)
	}
	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo ok=%v err=%v", ok, err)
	}
	if len(entry.Artifacts) != 1 || entry.Artifacts[0].Path != tasksFilePath(alpha.Path) {
		t.Fatalf("ledger artifacts = %+v, want only alpha tasks.md", entry.Artifacts)
	}
	if len(entry.ProjectIDs) != 1 || entry.ProjectIDs[0] != alpha.ID {
		t.Fatalf("ledger project IDs = %v, want only alpha", entry.ProjectIDs)
	}
	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if got := onDiskRevision(t, alpha); got != alphaBase.Revision {
		t.Fatalf("alpha revision after undo = %q, want %q", got, alphaBase.Revision)
	}
	if got := onDiskRevision(t, beta); got != betaBase.Revision {
		t.Fatalf("beta revision after undo = %q, want %q", got, betaBase.Revision)
	}
}

func TestCommitMultiRejectsProjectPreconditionPathBeforeEffects(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	project := newCommitTestProject(t, dataDir, "alpha", "alpha")
	outside := filepath.Join(t.TempDir(), "escaped-project")

	_, err := NewCommitService(transactionsDir, []string{dataDir}).Commit(ChangeSet{
		Project: project,
		ProjectPreconditions: []ProjectRevisionPrecondition{{
			ProjectID: "outside",
			Path:      outside,
		}},
	})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("Commit error = %v, want ErrTargetOutsideRoot", err)
	}
	for _, path := range []string{project.Path, transactionsDir, outside, filepath.Join(outside, LockFileName)} {
		if _, statErr := os.Lstat(path); !os.IsNotExist(statErr) {
			t.Fatalf("rejected precondition left filesystem effect at %s: %v", path, statErr)
		}
	}
}

func TestCommitMultiDeduplicatesResolvedProjectPreconditionLocks(t *testing.T) {
	dataDir := t.TempDir()
	transactionsDir := filepath.Join(dataDir, ".tt", "transactions")
	alpha := newCommitTestProject(t, dataDir, "alpha", "alpha before")
	beta := newCommitTestProject(t, dataDir, "beta", "beta stable")
	service := NewCommitService(transactionsDir, []string{dataDir})
	if _, err := service.CommitMulti([]ChangeSet{{Project: alpha}, {Project: beta}}); err != nil {
		t.Fatalf("seed CommitMulti: %v", err)
	}
	alphaBase, _ := LoadProjectDocument(alpha)
	betaBase, _ := LoadProjectDocument(beta)
	alias := filepath.Join(dataDir, "projects", "beta-alias")
	if err := os.Symlink(beta.Path, alias); err != nil {
		t.Fatalf("symlink beta alias: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		alphaLocal := projectFromDocument(alpha, cloneEdit(alphaBase, func(task *models.Task) { task.Priority = models.PriorityMin }))
		done <- func() error {
			_, err := service.Commit(ChangeSet{
				Project:          alphaLocal,
				ExpectedRevision: alphaBase.Revision,
				Base:             alphaBase,
				ProjectPreconditions: []ProjectRevisionPrecondition{
					{ProjectID: beta.ID, ProjectName: beta.Name, Path: beta.Path, ExpectedRevision: betaBase.Revision},
					{ProjectID: beta.ID, ProjectName: beta.Name, Path: alias, ExpectedRevision: betaBase.Revision},
				},
			})
			return err
		}()
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Commit with aliased precondition locks: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Commit deadlocked while acquiring aliased project precondition locks")
	}
}

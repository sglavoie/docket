package storage

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/sglavoie/tt/internal/transaction"
)

func TestArtifactBackupsAcrossCommitEntryPoints(t *testing.T) {
	tests := []struct {
		name      string
		committer func(*testing.T, *CommitService, string) func(ArtifactChange) error
	}{
		{
			name: "project commit",
			committer: func(t *testing.T, service *CommitService, root string) func(ArtifactChange) error {
				project := newCommitTestProject(t, root, "backup-project", "task")
				return func(change ArtifactChange) error {
					_, err := service.Commit(ChangeSet{Project: project, Artifacts: []ArtifactChange{change}})
					return err
				}
			},
		},
		{
			name: "lifecycle commit",
			committer: func(_ *testing.T, service *CommitService, _ string) func(ArtifactChange) error {
				return func(change ArtifactChange) error {
					return service.CommitOpsWithArtifacts(nil, []ArtifactChange{change})
				}
			},
		},
		{
			name: "artifact-only commit",
			committer: func(_ *testing.T, service *CommitService, _ string) func(ArtifactChange) error {
				return func(change ArtifactChange) error {
					_, err := service.CommitArtifacts([]ArtifactChange{change})
					return err
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			service, root := newCommitTestService(t)
			primary := filepath.Join(root, ".tt", "planning.json")
			backup := primary + ".bak"
			commit := tc.committer(t, service, root)

			if err := commit(ArtifactChange{
				Path: primary, Data: []byte("v1"), BackupPath: backup, ExpectedRevision: strptr(""),
			}); err != nil {
				t.Fatalf("first commit: %v", err)
			}
			assertArtifactBytes(t, primary, []byte("v1"))
			requireAbsent(t, backup)

			if err := commit(ArtifactChange{
				Path: primary, Data: []byte("v2"), BackupPath: backup, ExpectedRevision: strptr(RawArtifactRevision([]byte("v1"))),
			}); err != nil {
				t.Fatalf("second commit: %v", err)
			}
			assertArtifactBytes(t, primary, []byte("v2"))
			assertArtifactBytes(t, backup, []byte("v1"))
			if info, err := os.Stat(backup); err != nil {
				t.Fatalf("stat backup: %v", err)
			} else if got := info.Mode().Perm(); got != FilePermissions {
				t.Fatalf("backup permissions = %o, want %o", got, FilePermissions)
			}

			if err := commit(ArtifactChange{
				Path: primary, Data: []byte("v3"), BackupPath: backup, ExpectedRevision: strptr(RawArtifactRevision([]byte("v2"))),
			}); err != nil {
				t.Fatalf("third commit: %v", err)
			}
			assertArtifactBytes(t, primary, []byte("v3"))
			assertArtifactBytes(t, backup, []byte("v2"))
		})
	}
}

func TestArtifactBackupDeletePreservesDeletedPrimary(t *testing.T) {
	service, root := newCommitTestService(t)
	primary := filepath.Join(root, "settings.json")
	backup := primary + ".bak"
	writeArtifactBytes(t, primary, []byte("current"))

	if _, err := service.CommitArtifacts([]ArtifactChange{{
		Path: primary, Delete: true, BackupPath: backup, ExpectedRevision: strptr(RawArtifactRevision([]byte("current"))),
	}}); err != nil {
		t.Fatalf("CommitArtifacts delete: %v", err)
	}
	requireAbsent(t, primary)
	assertArtifactBytes(t, backup, []byte("current"))
}

func TestArtifactBackupStaleRevisionLeavesPrimaryAndBackupUnchanged(t *testing.T) {
	service, root := newCommitTestService(t)
	primary := filepath.Join(root, "planning.json")
	backup := primary + ".bak"
	writeArtifactBytes(t, primary, []byte("current"))
	writeArtifactBytes(t, backup, []byte("previous"))

	_, err := service.CommitArtifacts([]ArtifactChange{{
		Path: primary, Data: []byte("clobber"), BackupPath: backup, ExpectedRevision: strptr(RawArtifactRevision([]byte("stale"))),
	}})
	if !errors.Is(err, ErrStaleArtifact) {
		t.Fatalf("CommitArtifacts err = %v, want ErrStaleArtifact", err)
	}
	assertArtifactBytes(t, primary, []byte("current"))
	assertArtifactBytes(t, backup, []byte("previous"))
}

func TestArtifactBackupPrepareFailureLeavesPrimaryAndBackupUnchanged(t *testing.T) {
	service, root := newCommitTestService(t)
	primary := filepath.Join(root, "planning.json")
	writeArtifactBytes(t, primary, []byte("current"))
	blocker := filepath.Join(root, "not-a-directory")
	writeArtifactBytes(t, blocker, []byte("blocker"))
	backup := filepath.Join(blocker, "planning.json.bak")

	_, err := service.CommitArtifacts([]ArtifactChange{{
		Path: primary, Data: []byte("next"), BackupPath: backup, ExpectedRevision: strptr(RawArtifactRevision([]byte("current"))),
	}})
	if err == nil {
		t.Fatal("CommitArtifacts unexpectedly succeeded")
	}
	assertArtifactBytes(t, primary, []byte("current"))
	assertArtifactBytes(t, blocker, []byte("blocker"))
	if _, readErr := os.ReadFile(backup); readErr == nil {
		t.Fatal("failed transaction created backup")
	}
	assertJournalClean(t, root)
}

func TestArtifactBackupValidationRejectsBeforeFilesystemSideEffects(t *testing.T) {
	tests := []struct {
		name      string
		committer func(*testing.T, *CommitService, string, ArtifactChange) error
	}{
		{
			name: "project commit",
			committer: func(t *testing.T, service *CommitService, root string, change ArtifactChange) error {
				project := newCommitTestProject(t, root, "invalid-backup", "task")
				_, err := service.Commit(ChangeSet{Project: project, Artifacts: []ArtifactChange{change}})
				return err
			},
		},
		{
			name: "lifecycle commit",
			committer: func(_ *testing.T, service *CommitService, _ string, change ArtifactChange) error {
				return service.CommitOpsWithArtifacts(nil, []ArtifactChange{change})
			},
		},
		{
			name: "artifact-only commit",
			committer: func(_ *testing.T, service *CommitService, _ string, change ArtifactChange) error {
				_, err := service.CommitArtifacts([]ArtifactChange{change})
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "workspace")
			primary := filepath.Join(root, "planning.json")
			writeArtifactBytes(t, primary, []byte("current"))
			outside := filepath.Join(t.TempDir(), "planning.json.bak")
			transactionsDir := filepath.Join(root, ".tt", "transactions")
			service := NewCommitService(transactionsDir, []string{root})

			err := tc.committer(t, service, root, ArtifactChange{Path: primary, Data: []byte("next"), BackupPath: outside})
			if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
				t.Fatalf("commit err = %v, want ErrTargetOutsideRoot", err)
			}
			assertArtifactBytes(t, primary, []byte("current"))
			requireAbsent(t, outside)
			requireAbsent(t, primary+".lock")
			requireAbsent(t, transactionsDir)
		})
	}
}

func TestArtifactBackupValidationRejectsSelfReferenceAndCollisions(t *testing.T) {
	tests := []struct {
		name      string
		artifacts func(string) []ArtifactChange
	}{
		{
			name: "self reference",
			artifacts: func(root string) []ArtifactChange {
				primary := filepath.Join(root, "planning.json")
				return []ArtifactChange{{Path: primary, Data: []byte("next"), BackupPath: primary}}
			},
		},
		{
			name: "primary target collision",
			artifacts: func(root string) []ArtifactChange {
				primary := filepath.Join(root, "planning.json")
				other := filepath.Join(root, "settings.json")
				return []ArtifactChange{{Path: primary, Data: []byte("next"), BackupPath: other}, {Path: other, Data: []byte("settings")}}
			},
		},
		{
			name: "backup target collision",
			artifacts: func(root string) []ArtifactChange {
				backup := filepath.Join(root, "domains.bak")
				return []ArtifactChange{
					{Path: filepath.Join(root, "planning.json"), Data: []byte("planning"), BackupPath: backup},
					{Path: filepath.Join(root, "settings.json"), Data: []byte("settings"), BackupPath: backup},
				}
			},
		},
		{
			name: "lock collision",
			artifacts: func(root string) []ArtifactChange {
				primary := filepath.Join(root, "planning.json")
				return []ArtifactChange{{Path: primary, Data: []byte("next"), BackupPath: primary + ".lock"}}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			service := NewCommitService(filepath.Join(root, ".tt", "transactions"), []string{root})
			artifacts := tc.artifacts(root)
			_, err := service.CommitArtifacts(artifacts)
			if err == nil {
				t.Fatal("CommitArtifacts unexpectedly succeeded")
			}
			for _, artifact := range artifacts {
				requireAbsent(t, artifact.Path)
				if artifact.BackupPath != artifact.Path {
					requireAbsent(t, artifact.BackupPath)
				}
			}
			requireAbsent(t, filepath.Join(root, ".tt", "transactions"))
		})
	}
}

func TestArtifactBackupValidationRejectsSymlinkEscape(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows CI")
	}
	root := t.TempDir()
	outside := t.TempDir()
	escape := filepath.Join(root, "escape")
	if err := os.Symlink(outside, escape); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	primary := filepath.Join(root, "planning.json")
	writeArtifactBytes(t, primary, []byte("current"))
	backup := filepath.Join(escape, "planning.json.bak")
	service := NewCommitService(filepath.Join(root, ".tt", "transactions"), []string{root})

	_, err := service.CommitArtifacts([]ArtifactChange{{Path: primary, Data: []byte("next"), BackupPath: backup}})
	if !errors.Is(err, transaction.ErrTargetOutsideRoot) {
		t.Fatalf("CommitArtifacts err = %v, want ErrTargetOutsideRoot", err)
	}
	assertArtifactBytes(t, primary, []byte("current"))
	requireAbsent(t, filepath.Join(outside, "planning.json.bak"))
}

func TestArtifactBackupPreparedManifestRecoversBackupAndPrimaryTogether(t *testing.T) {
	root := t.TempDir()
	primary := filepath.Join(root, "planning.json")
	backup := primary + ".bak"
	writeArtifactBytes(t, primary, []byte("v1"))
	ops, _, err := expandArtifactChanges([]ArtifactChange{{Path: primary, Data: []byte("v2"), BackupPath: backup}})
	if err != nil {
		t.Fatalf("expandArtifactChanges: %v", err)
	}
	if len(ops) != 2 || ops[0].Target != backup || ops[1].Target != primary {
		t.Fatalf("expanded ops = %#v, want backup then primary", ops)
	}
	if !bytes.Equal(ops[0].Data, []byte("v1")) || ops[0].Mode != FilePermissions {
		t.Fatalf("backup op = %#v, want v1 at 0600", ops[0])
	}

	journal := transaction.NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root}, "")
	if _, prepareErr := journal.Prepare(ops); prepareErr != nil {
		t.Fatalf("Prepare: %v", prepareErr)
	}
	assertArtifactBytes(t, primary, []byte("v1"))
	requireAbsent(t, backup)

	report, err := transaction.NewJournal(filepath.Join(root, ".tt", "transactions"), []string{root}, "").Recover()
	if err != nil {
		t.Fatalf("Recover: %v", err)
	}
	if len(report.RolledForward) != 1 {
		t.Fatalf("RolledForward = %v, want one transaction", report.RolledForward)
	}
	assertArtifactBytes(t, backup, []byte("v1"))
	assertArtifactBytes(t, primary, []byte("v2"))
}

func TestArtifactBackupIsExcludedFromUndoHistory(t *testing.T) {
	service, ledger, root := newArtifactLedgerTestService(t)
	primary := filepath.Join(root, "planning.json")
	backup := primary + ".bak"
	writeArtifactBytes(t, primary, []byte("v1"))

	if _, err := service.CommitArtifactTransaction(ArtifactCommitRequest{
		Artifacts:  []ArtifactChange{{Path: primary, Data: []byte("v2"), BackupPath: backup}},
		UndoAction: "update planning",
	}); err != nil {
		t.Fatalf("CommitArtifactTransaction: %v", err)
	}
	entry, ok, err := ledger.SelectNewestUndo(GlobalScope())
	if err != nil || !ok {
		t.Fatalf("SelectNewestUndo ok=%v err=%v", ok, err)
	}
	if len(entry.Artifacts) != 1 || entry.Artifacts[0].Path != primary {
		t.Fatalf("ledger artifacts = %#v, want only primary", entry.Artifacts)
	}
	assertArtifactBytes(t, backup, []byte("v1"))

	if _, err := ledger.Undo(GlobalScope()); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	assertArtifactBytes(t, primary, []byte("v1"))
	assertArtifactBytes(t, backup, []byte("v1"))
	if _, err := ledger.Redo(GlobalScope()); err != nil {
		t.Fatalf("Redo: %v", err)
	}
	assertArtifactBytes(t, primary, []byte("v2"))
	assertArtifactBytes(t, backup, []byte("v1"))
}

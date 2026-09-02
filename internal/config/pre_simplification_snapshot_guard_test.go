package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/sglavoie/tt/internal/storage"
)

func TestCreatePreSimplificationV5SnapshotIsByteDeterministic(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	rootA := filepath.Join(t.TempDir(), "a")
	rootB := filepath.Join(t.TempDir(), "b")
	seedDeterministicPreSimplificationV5Workspace(t, rootA)
	seedDeterministicPreSimplificationV5Workspace(t, rootB)

	first, err := createPreSimplificationV5Snapshot(rootA, &preSimplificationV5SnapshotHooks{now: preSimplificationV5TestTime})
	if err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	second, err := createPreSimplificationV5Snapshot(rootB, &preSimplificationV5SnapshotHooks{now: preSimplificationV5TestTime})
	if err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	firstArchive, err := os.ReadFile(first.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	secondArchive, err := os.ReadFile(second.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(firstArchive, secondArchive) {
		t.Fatal("identical source bytes and timestamp produced different tar.gz bytes")
	}
	firstAudit, _ := os.ReadFile(first.AuditPath)
	secondAudit, _ := os.ReadFile(second.AuditPath)
	if !bytes.Equal(firstAudit, secondAudit) {
		t.Fatal("identical source bytes and timestamp produced different audit bytes")
	}
	if first.ArchiveSHA256 != second.ArchiveSHA256 {
		t.Fatalf("deterministic archive hashes differ: %s != %s", first.ArchiveSHA256, second.ArchiveSHA256)
	}
}

func TestCreatePreSimplificationV5SnapshotRefusesCollisionWithoutPartialPair(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedDeterministicPreSimplificationV5Workspace(t, root)
	backupsDir := PreSimplificationV5BackupsDir(root)
	if err := os.MkdirAll(backupsDir, DirPermissions); err != nil {
		t.Fatal(err)
	}
	stem := "pre-simplification-v5-20260729T010203Z"
	archivePath := filepath.Join(backupsDir, stem+".tar.gz")
	auditPath := filepath.Join(backupsDir, stem+".audit.md")
	if err := os.WriteFile(archivePath, []byte("existing immutable snapshot"), FilePermissions); err != nil {
		t.Fatal(err)
	}

	_, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{now: preSimplificationV5TestTime})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("collision error = %v", err)
	}
	got, readErr := os.ReadFile(archivePath)
	if readErr != nil || string(got) != "existing immutable snapshot" {
		t.Fatalf("collision changed existing archive: bytes=%q err=%v", got, readErr)
	}
	if _, statErr := os.Stat(auditPath); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("collision published a partial audit pair: %v", statErr)
	}
}

func TestCreatePreSimplificationV5SnapshotDoesNotOverwriteDestinationRace(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedDeterministicPreSimplificationV5Workspace(t, root)

	_, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{
		now: preSimplificationV5TestTime,
		beforePublish: func(archivePath, _ string) {
			if mkdirErr := os.MkdirAll(filepath.Dir(archivePath), DirPermissions); mkdirErr != nil {
				t.Fatalf("create racing backup dir: %v", mkdirErr)
			}
			if writeErr := os.WriteFile(archivePath, []byte("external destination race"), FilePermissions); writeErr != nil {
				t.Fatalf("create destination race: %v", writeErr)
			}
		},
	})
	if err == nil || !errors.Is(err, storage.ErrStaleArtifact) {
		t.Fatalf("destination race error = %v, want stale artifact", err)
	}
	stem := filepath.Join(
		PreSimplificationV5BackupsDir(root),
		"pre-simplification-v5-20260729T010203Z",
	)
	got, readErr := os.ReadFile(stem + ".tar.gz")
	if readErr != nil || string(got) != "external destination race" {
		t.Fatalf("destination race was overwritten: bytes=%q err=%v", got, readErr)
	}
	if _, statErr := os.Stat(stem + ".audit.md"); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("destination race published a partial audit: %v", statErr)
	}
}

func TestCreatePreSimplificationV5SnapshotRejectsUnsafeSourceWithoutBackup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink semantics differ on Windows")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedDeterministicPreSimplificationV5Workspace(t, root)
	outside := filepath.Join(t.TempDir(), "outside.md")
	if err := os.WriteFile(outside, []byte("outside"), FilePermissions); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "projects", "alpha", "unsafe.md")); err != nil {
		t.Fatal(err)
	}

	_, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{now: preSimplificationV5TestTime})
	if err == nil || !strings.Contains(err.Error(), "refuses symlink source") {
		t.Fatalf("symlink error = %v", err)
	}
	if _, statErr := os.Stat(PreSimplificationV5BackupsDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("unsafe source created backup state: %v", statErr)
	}
	if _, statErr := os.Stat(TransactionsDir(root)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("preflight source failure created a transaction marker: %v", statErr)
	}
}

func TestCreatePreSimplificationV5SnapshotSourceDivergencePublishesNothing(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedDeterministicPreSimplificationV5Workspace(t, root)
	tasksPath := filepath.Join(root, "projects", "alpha", "tasks.md")

	_, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{
		now: preSimplificationV5TestTime,
		afterInventory: func() {
			if writeErr := os.WriteFile(tasksPath, []byte("direct filesystem race"), FilePermissions); writeErr != nil {
				t.Fatalf("mutate source: %v", writeErr)
			}
		},
	})
	if err == nil || !strings.Contains(err.Error(), "source changed while snapshotting") {
		t.Fatalf("source divergence error = %v", err)
	}
	assertNoPreSimplificationV5Pair(t, root)
}

func TestCreatePreSimplificationV5SnapshotHoldsWorkspaceCommitLock(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	root := filepath.Join(t.TempDir(), "workspace")
	seedDeterministicPreSimplificationV5Workspace(t, root)
	tasksPath := filepath.Join(root, "projects", "alpha", "tasks.md")

	inventoried := make(chan struct{})
	allowSnapshot := make(chan struct{})
	snapshotDone := make(chan struct {
		report *PreSimplificationV5Snapshot
		err    error
	}, 1)
	go func() {
		report, err := createPreSimplificationV5Snapshot(root, &preSimplificationV5SnapshotHooks{
			now: preSimplificationV5TestTime,
			afterInventory: func() {
				close(inventoried)
				<-allowSnapshot
			},
		})
		snapshotDone <- struct {
			report *PreSimplificationV5Snapshot
			err    error
		}{report: report, err: err}
	}()

	select {
	case <-inventoried:
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not reach locked inventory hook")
	}

	commitDone := make(chan error, 1)
	go func() {
		expected, readErr := os.ReadFile(tasksPath)
		if readErr != nil {
			commitDone <- readErr
			return
		}
		revision := storage.RawArtifactRevision(expected)
		_, commitErr := storage.NewCommitService(TransactionsDir(root), []string{root}).CommitArtifacts([]storage.ArtifactChange{{
			Path:             tasksPath,
			Data:             []byte("committed after snapshot"),
			Mode:             FilePermissions,
			ExpectedRevision: &revision,
		}})
		commitDone <- commitErr
	}()
	select {
	case err := <-commitDone:
		t.Fatalf("workspace commit completed while snapshot held the lock: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	close(allowSnapshot)
	var snapshotResult struct {
		report *PreSimplificationV5Snapshot
		err    error
	}
	select {
	case snapshotResult = <-snapshotDone:
		if snapshotResult.err != nil {
			t.Fatalf("snapshot: %v", snapshotResult.err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("snapshot did not complete")
	}
	select {
	case err := <-commitDone:
		if err != nil {
			t.Fatalf("blocked commit after snapshot: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("blocked workspace commit did not resume")
	}

	entries := readPreSimplificationV5Archive(t, snapshotResult.report.ArchivePath)
	if got := string(entries["workspace/projects/alpha/tasks.md"]); got != "Active task\n" {
		t.Fatalf("snapshot task bytes = %q, want pre-commit bytes", got)
	}
	if got, err := os.ReadFile(tasksPath); err != nil || string(got) != "committed after snapshot" {
		t.Fatalf("live task bytes = %q err=%v, want resumed commit", got, err)
	}
}

func TestPreSimplificationV5SnapshotMutationGuard(t *testing.T) {
	root := filepath.Join(t.TempDir(), "workspace")
	protected := filepath.Join(
		PreSimplificationV5BackupsDir(root),
		"pre-simplification-v5-20260729T010203Z.tar.gz",
	)
	if !IsPreSimplificationV5SnapshotPath(root, protected) {
		t.Fatal("canonical archive path was not recognized as protected")
	}
	if err := RefusePreSimplificationV5SnapshotMutation(root, protected); !errors.Is(err, ErrImmutablePreSimplificationV5Snapshot) {
		t.Fatalf("mutation guard error = %v", err)
	}
	for _, path := range []string{
		filepath.Join(PreSimplificationV5BackupsDir(root), "pre-simplification-v5-not-a-time.tar.gz"),
		filepath.Join(root, "pre-simplification-v5-20260729T010203Z.tar.gz"),
		filepath.Join(PreSimplificationV5BackupsDir(root), "ordinary.tar.gz"),
	} {
		if IsPreSimplificationV5SnapshotPath(root, path) {
			t.Errorf("non-canonical path was treated as protected: %s", path)
		}
		if err := RefusePreSimplificationV5SnapshotMutation(root, path); err != nil {
			t.Errorf("ordinary path mutation was refused: %v", err)
		}
	}
}

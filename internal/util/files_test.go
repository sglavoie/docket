package util

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWithFileLock_ExclusiveSerializesConcurrentAccess(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			err := WithFileLock(lockPath, true, func() error {
				n := concurrent.Add(1)
				// Track max concurrency observed
				for {
					old := maxConcurrent.Load()
					if n <= old || maxConcurrent.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				concurrent.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	wg.Wait()

	if maxConcurrent.Load() != 1 {
		t.Errorf("expected max concurrency of 1, got %d", maxConcurrent.Load())
	}
}

func TestWithFileLock_SharedAllowsConcurrentReads(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	var wg sync.WaitGroup

	for range 5 {
		wg.Go(func() {
			err := WithFileLock(lockPath, false, func() error {
				n := concurrent.Add(1)
				for {
					old := maxConcurrent.Load()
					if n <= old || maxConcurrent.CompareAndSwap(old, n) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				concurrent.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}

	wg.Wait()

	if maxConcurrent.Load() < 2 {
		t.Errorf("expected shared locks to allow concurrency > 1, got %d", maxConcurrent.Load())
	}
}

func TestWithFileLock_CreatesLockFile(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	err := WithFileLock(lockPath, true, func() error {
		_, statErr := os.Stat(lockPath)
		if statErr != nil {
			t.Errorf("lock file should exist during fn: %v", statErr)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWithFileLock_PropagatesFnError(t *testing.T) {
	lockPath := filepath.Join(t.TempDir(), "test.lock")

	err := WithFileLock(lockPath, true, func() error {
		return os.ErrPermission
	})
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected ErrPermission, got %v", err)
	}
}

// TestAtomicWriteFile_NewFileUsesRequestedMode verifies a brand-new file is
// created with exactly the requested (private) mode.
func TestAtomicWriteFile_NewFileUsesRequestedMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "new.txt")
	if err := AtomicWriteFile(path, []byte("data"), FilePermissions); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(FilePermissions); got != want {
		t.Errorf("mode = %04o, want %04o", got, want)
	}
}

// TestAtomicWriteFile_TightensLooserExistingMode verifies a world-readable
// existing file is tightened down to the private default on rewrite.
func TestAtomicWriteFile_TightensLooserExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "loose.txt")
	if err := os.WriteFile(path, []byte("old"), 0o666); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("new"), FilePermissions); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o600); got != want {
		t.Errorf("mode = %04o, want %04o (looser existing mode should be tightened)", got, want)
	}
}

// TestAtomicWriteFile_PreservesStricterExistingMode verifies a user-hardened
// read-only file keeps its stricter mode rather than being widened to the
// default.
func TestAtomicWriteFile_PreservesStricterExistingMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "strict.txt")
	if err := os.WriteFile(path, []byte("old"), 0o400); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("new"), FilePermissions); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o400); got != want {
		t.Errorf("mode = %04o, want %04o (stricter existing mode should be preserved)", got, want)
	}
}

// TestAtomicWriteFile_SyncsParentDirectory verifies the rename durability hook
// fsyncs the parent directory of the written file.
func TestAtomicWriteFile_SyncsParentDirectory(t *testing.T) {
	orig := syncDir
	t.Cleanup(func() { syncDir = orig })

	var synced []string
	syncDir = func(dir string) error {
		synced = append(synced, dir)
		return orig(dir)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "durable.txt")
	if err := AtomicWriteFile(path, []byte("data"), FilePermissions); err != nil {
		t.Fatalf("AtomicWriteFile: %v", err)
	}
	if len(synced) != 1 || synced[0] != dir {
		t.Errorf("syncDir calls = %v, want exactly [%q]", synced, dir)
	}
}

// TestAtomicWriteFile_SyncDirErrorPropagates verifies a failure in the
// durability hook is surfaced to the caller.
func TestAtomicWriteFile_SyncDirErrorPropagates(t *testing.T) {
	orig := syncDir
	t.Cleanup(func() { syncDir = orig })
	syncDir = func(string) error { return os.ErrPermission }

	path := filepath.Join(t.TempDir(), "fail.txt")
	err := AtomicWriteFile(path, []byte("data"), FilePermissions)
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected propagated ErrPermission, got %v", err)
	}
}

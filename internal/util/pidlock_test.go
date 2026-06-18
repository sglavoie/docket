package util

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAcquirePIDLock_Success verifies a fresh acquire writes the PID.
func TestAcquirePIDLock_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := AcquirePIDLock(path); err != nil {
		t.Fatalf("AcquirePIDLock: %v", err)
	}
	defer ReleasePIDLock(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("parse pid: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("lock pid = %d, want %d", pid, os.Getpid())
	}
}

// TestAcquirePIDLock_StaleOverridden verifies a stale lock file (containing a
// non-running PID) is removed and re-acquired by this process.
func TestAcquirePIDLock_StaleOverridden(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	// PID 1 is init/systemd on Unix; we pick an obviously dead PID
	// by using a very large value that's effectively never a live process.
	if err := os.WriteFile(path, []byte("2147483646"), 0o644); err != nil {
		t.Fatalf("seed stale lock: %v", err)
	}
	if err := AcquirePIDLock(path); err != nil {
		t.Fatalf("AcquirePIDLock over stale lock: %v", err)
	}
	defer ReleasePIDLock(path)

	data, _ := os.ReadFile(path)
	pid, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	if pid != os.Getpid() {
		t.Errorf("stale lock not replaced: got pid=%d, want %d", pid, os.Getpid())
	}
}

// TestAcquirePIDLock_LiveLockRejected verifies a lock owned by a known-live
// PID (the current process) is rejected.
func TestAcquirePIDLock_LiveLockRejected(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("seed live lock: %v", err)
	}
	if err := AcquirePIDLock(path); err == nil {
		_ = os.Remove(path)
		t.Fatal("expected error when lock held by live PID, got nil")
	}
}

func TestAcquirePIDLock_NonPositivePIDTreatedAsStale(t *testing.T) {
	tests := []struct {
		name string
		pid  string
	}{
		{name: "zero", pid: "0"},
		{name: "negative", pid: "-1"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "pid.lock")
			if err := os.WriteFile(path, []byte(tc.pid), 0o644); err != nil {
				t.Fatalf("seed lock: %v", err)
			}
			if err := AcquirePIDLock(path); err != nil {
				t.Fatalf("AcquirePIDLock over non-positive PID %s: %v", tc.pid, err)
			}
			defer ReleasePIDLock(path)

			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read lock: %v", err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
			if err != nil {
				t.Fatalf("parse pid: %v", err)
			}
			if pid != os.Getpid() {
				t.Errorf("lock pid = %d, want %d", pid, os.Getpid())
			}
		})
	}
}

// TestAcquirePIDLock_StaleByAge verifies a too-old lock is treated as stale
// even if its PID happens to be alive.
func TestAcquirePIDLock_StaleByAge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("seed lock: %v", err)
	}
	// Backdate the file mtime past maxLockAge.
	old := time.Now().Add(-maxLockAge - time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	if err := AcquirePIDLock(path); err != nil {
		t.Fatalf("AcquirePIDLock over aged lock: %v", err)
	}
	defer ReleasePIDLock(path)
}

// TestAcquirePIDLock_ConcurrentRace verifies that with many goroutines racing
// to acquire a fresh lock, exactly one wins. Prior to the O_EXCL fix, two
// processes could both find no lock and both write, leaving no consistent owner.
func TestAcquirePIDLock_ConcurrentRace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	const goroutines = 16

	var wg sync.WaitGroup
	var mu sync.Mutex
	successes := 0
	wg.Add(goroutines)
	start := make(chan struct{})

	for range goroutines {
		go func() {
			defer wg.Done()
			<-start
			if err := AcquirePIDLock(path); err == nil {
				mu.Lock()
				successes++
				mu.Unlock()
			}
		}()
	}
	close(start)
	wg.Wait()

	// Because all goroutines run in the same process, isProcessAlive is true
	// for our PID, so once the lock is taken, all others must fail.
	if successes != 1 {
		t.Errorf("expected exactly 1 successful acquire, got %d", successes)
	}

	// Cleanup
	ReleasePIDLock(path)
}

// TestReleasePIDLock_Idempotent ensures release does not error on missing file.
func TestReleasePIDLock_Idempotent(t *testing.T) {
	ReleasePIDLock(filepath.Join(t.TempDir(), "never-existed.lock"))
}

func TestReleasePIDLock_RemovesOwnedLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("seed owned lock: %v", err)
	}

	ReleasePIDLock(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected owned lock removed, stat err = %v", err)
	}
}

func TestReleasePIDLock_PreservesForeignLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	foreignPID := os.Getpid() + 1
	if err := os.WriteFile(path, []byte(strconv.Itoa(foreignPID)), 0o644); err != nil {
		t.Fatalf("seed foreign lock: %v", err)
	}

	ReleasePIDLock(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("foreign lock should remain readable: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != strconv.Itoa(foreignPID) {
		t.Fatalf("foreign lock contents = %q, want %d", got, foreignPID)
	}
}

func TestReleasePIDLock_PreservesMalformedLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	if err := os.WriteFile(path, []byte("not-a-pid"), 0o644); err != nil {
		t.Fatalf("seed malformed lock: %v", err)
	}

	ReleasePIDLock(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("malformed lock should remain readable: %v", err)
	}
	if got := strings.TrimSpace(string(data)); got != "not-a-pid" {
		t.Fatalf("malformed lock contents = %q, want %q", got, "not-a-pid")
	}
}

func TestIsProcessAlive_NonPositivePID(t *testing.T) {
	tests := []int{0, -1, -42}
	for _, pid := range tests {
		if isProcessAlive(pid) {
			t.Fatalf("isProcessAlive(%d) = true, want false", pid)
		}
	}
}

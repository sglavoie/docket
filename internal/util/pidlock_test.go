package util

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestAcquirePIDLock_Success verifies a fresh acquire records the current PID
// and start identity in the lock file.
func TestAcquirePIDLock_Success(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	lock, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("AcquirePIDLock: %v", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock: %v", err)
	}
	if got := string(data); !strings.Contains(got, "pid="+strconv.Itoa(os.Getpid())) {
		t.Errorf("lock file = %q, want it to record pid=%d", got, os.Getpid())
	}
	if !strings.Contains(string(data), "started=") {
		t.Errorf("lock file = %q, want a started= start-identity line", data)
	}
}

// TestAcquirePIDLock_HeldRejectsSecondAcquire verifies that while one holder
// keeps the OS lock, a second acquire from a separate open file description is
// rejected — this is the held-lock authority replacing age-based expiry.
func TestAcquirePIDLock_HeldRejectsSecondAcquire(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	first, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("first AcquirePIDLock: %v", err)
	}

	if second, acquireErr := AcquirePIDLock(path); acquireErr == nil {
		second.Release()
		first.Release()
		t.Fatal("expected second acquire to be rejected while lock is held, got nil")
	} else if !strings.Contains(acquireErr.Error(), "already running") {
		first.Release()
		t.Fatalf("second acquire error = %v, want an 'already running' diagnostic", acquireErr)
	}

	// After the holder releases, the lock becomes available again.
	first.Release()
	third, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	third.Release()
}

// TestAcquirePIDLock_StaleDeadProcess verifies that a leftover lock file from a
// dead process (content present, but no OS lock held) is acquired successfully
// and its identity overwritten.
func TestAcquirePIDLock_StaleDeadProcess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	// A dead process leaves a file behind but no held OS lock.
	stale := "pid=2147483646\nstarted=2000-01-01T00:00:00Z\n"
	if err := os.WriteFile(path, []byte(stale), 0o600); err != nil {
		t.Fatalf("seed stale lock: %v", err)
	}

	lock, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("AcquirePIDLock over stale dead-process lock: %v", err)
	}
	defer lock.Release()

	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), "pid="+strconv.Itoa(os.Getpid())) {
		t.Errorf("stale lock not replaced: %q", data)
	}
}

// TestAcquirePIDLock_LiveProcessOlderThan24h verifies that a live holder is
// never treated as stale merely because its lock file is old. Under the old
// age-based expiry a >24h lock would have been stolen; with a held OS lock the
// age is irrelevant and a second acquire is still rejected.
func TestAcquirePIDLock_LiveProcessOlderThan24h(t *testing.T) {
	path := filepath.Join(t.TempDir(), "pid.lock")
	holder, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("acquire holder: %v", err)
	}
	defer holder.Release()

	// Backdate the lock file well past the former 24h expiry threshold.
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	if second, err := AcquirePIDLock(path); err == nil {
		second.Release()
		t.Fatal("expected aged-but-live lock to remain held, got successful acquire")
	}
}

// TestReleasePIDLock_Idempotent ensures Release tolerates nil receivers and
// repeated calls without panicking.
func TestReleasePIDLock_Idempotent(t *testing.T) {
	var nilLock *PIDLock
	nilLock.Release()

	path := filepath.Join(t.TempDir(), "pid.lock")
	lock, err := AcquirePIDLock(path)
	if err != nil {
		t.Fatalf("AcquirePIDLock: %v", err)
	}
	lock.Release()
	lock.Release()
}

// TestFormatLockOwner covers the diagnostic-message rendering variants.
func TestFormatLockOwner(t *testing.T) {
	tests := []struct {
		name  string
		owner lockOwner
		want  string
	}{
		{name: "pid and start", owner: lockOwner{pid: 42, startedAt: "2026-07-10T00:00:00Z"}, want: " (PID 42, started 2026-07-10T00:00:00Z)"},
		{name: "pid only", owner: lockOwner{pid: 42}, want: " (PID 42)"},
		{name: "empty", owner: lockOwner{pid: -1}, want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := formatLockOwner(tc.owner); got != tc.want {
				t.Errorf("formatLockOwner(%+v) = %q, want %q", tc.owner, got, tc.want)
			}
		})
	}
}

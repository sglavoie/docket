package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// maxLockAge is the maximum age for a lock file before it's considered stale,
// regardless of whether the PID appears alive (handles zombie/orphaned locks).
const maxLockAge = 24 * time.Hour

// AcquirePIDLock creates a lock file containing the current PID. If the file
// already exists and the recorded PID is still alive, it returns an error.
// Stale lock files (where the PID is no longer running or the lock is older
// than maxLockAge) are removed and the lock is re-acquired. The lock file is
// written via a temp-and-rename dance so concurrent readers never observe a
// half-written PID.
func AcquirePIDLock(path string) error {
	if err := tryCreateLock(path); err == nil {
		return nil
	} else if !os.IsExist(err) {
		return err
	}

	// Lock file already exists — inspect it.
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		// File raced away between OpenFile and ReadFile; retry once.
		if os.IsNotExist(readErr) {
			return tryCreateLock(path)
		}
		return fmt.Errorf("read lock file: %w", readErr)
	}
	pidStr := strings.TrimSpace(string(data))
	pid, parseErr := strconv.Atoi(pidStr)
	if parseErr != nil {
		// Empty or garbage content — treat as stale only if the file is also
		// old enough that we're sure no fresh writer is mid-flight.
		if !isLockStale(path) {
			return fmt.Errorf("tt TUI lock file unreadable (lock contention)")
		}
	} else if isProcessAlive(pid) && !isLockStale(path) {
		return fmt.Errorf("tt TUI already running (PID %d)", pid)
	}

	// Stale lock — remove and retry once with O_EXCL so a competing process
	// that wins the race surfaces as ErrExist rather than overwriting.
	if removeErr := os.Remove(path); removeErr != nil && !os.IsNotExist(removeErr) {
		return removeErr
	}
	if err := tryCreateLock(path); err != nil {
		if os.IsExist(err) {
			return fmt.Errorf("tt TUI already running (lock contention)")
		}
		return err
	}
	return nil
}

// tryCreateLock atomically creates the lock file with the current PID using a
// temp file + rename so concurrent readers never see an empty/half-written lock.
// Returns os.ErrExist (wrapped) if the destination already exists.
func tryCreateLock(path string) error {
	// Pre-flight existence check before creating a temp file so we don't
	// generate noise in the lock dir on the common already-held path.
	if _, err := os.Stat(path); err == nil {
		return &os.PathError{Op: "open", Path: path, Err: syscall.EEXIST}
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	tmp, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, writeErr := tmp.WriteString(strconv.Itoa(os.Getpid())); writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return writeErr
	}
	if closeErr := tmp.Close(); closeErr != nil {
		_ = os.Remove(tmpPath)
		return closeErr
	}
	// os.Link is atomic on POSIX and fails with EEXIST if the destination
	// already exists, giving us mutual exclusion that os.Rename lacks.
	if linkErr := os.Link(tmpPath, path); linkErr != nil {
		_ = os.Remove(tmpPath)
		return linkErr
	}
	_ = os.Remove(tmpPath)
	return nil
}

// ReleasePIDLock removes the lock file.
func ReleasePIDLock(path string) {
	_ = os.Remove(path)
}

// isLockStale returns true if the lock file is older than maxLockAge.
func isLockStale(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return time.Since(info.ModTime()) > maxLockAge
}

func isProcessAlive(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = proc.Signal(syscall.Signal(0))
	return err == nil
}

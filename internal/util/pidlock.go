package util

import (
	"fmt"
	"os"
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
// Stale lock files (where the PID is no longer running) are silently overwritten.
func AcquirePIDLock(path string) error {
	data, err := os.ReadFile(path)
	if err == nil {
		pidStr := strings.TrimSpace(string(data))
		if pid, parseErr := strconv.Atoi(pidStr); parseErr == nil {
			if isProcessAlive(pid) && !isLockStale(path) {
				return fmt.Errorf("tt TUI already running (PID %d)", pid)
			}
		}
	}

	return os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())), 0o644)
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

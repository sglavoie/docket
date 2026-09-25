package util

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// PIDLock is an exclusive, OS-enforced lock on a lock file, held for the
// lifetime of the owning process.
//
// Authority comes from a held advisory OS lock (flock), not from the lock
// file's age or the liveness of the recorded PID. While the owner is alive the
// kernel keeps the lock, so a second acquire fails immediately; when the owner
// dies — even uncleanly — the kernel releases the lock automatically, so a
// leftover lock file never blocks a new acquire regardless of how old it is.
//
// The PID and start identity written into the file are diagnostics only,
// surfaced in the "already running" error to help a human identify the holder.
type PIDLock struct {
	file *os.File
}

// lockOwner is the diagnostic identity recorded in a lock file. startedAt tags
// the moment the lock was acquired so a reused PID can be told apart from the
// original holder when inspecting the file.
type lockOwner struct {
	pid       int
	startedAt string
}

// AcquirePIDLock opens (creating if needed) the lock file at path and takes an
// exclusive, non-blocking OS lock on it. It returns an error if another live
// process already holds the lock. On success the returned *PIDLock keeps the
// underlying file descriptor open — the OS lock is held until Release is called
// or the process exits.
func AcquirePIDLock(path string) (*PIDLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FilePermissions)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if lockErr := flockExclusiveNB(f); lockErr != nil {
		if errors.Is(lockErr, errLockWouldBlock) {
			diag := formatLockOwner(readLockOwner(f))
			_ = f.Close()
			return nil, fmt.Errorf("tt TUI already running%s", diag)
		}
		_ = f.Close()
		return nil, fmt.Errorf("acquire lock: %w", lockErr)
	}

	if err := writeLockOwner(f); err != nil {
		_ = flockUnlock(f)
		_ = f.Close()
		return nil, err
	}

	return &PIDLock{file: f}, nil
}

// Release drops the OS lock and closes the lock file. The lock file itself is
// intentionally left in place: the held OS lock — not the file's presence — is
// what provides mutual exclusion, so leaving it avoids the classic unlink/flock
// race where two processes end up locking different inodes for the same path.
// Release is safe to call on a nil receiver and more than once.
func (l *PIDLock) Release() {
	if l == nil || l.file == nil {
		return
	}
	_ = flockUnlock(l.file)
	_ = l.file.Close()
	l.file = nil
}

// writeLockOwner truncates the lock file and records the current process
// identity for diagnostics.
func writeLockOwner(f *os.File) error {
	body := fmt.Sprintf("pid=%d\nstarted=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if err := f.Truncate(0); err != nil {
		return fmt.Errorf("truncate lock file: %w", err)
	}
	if _, err := f.WriteAt([]byte(body), 0); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}
	if err := SyncFile(f); err != nil {
		return fmt.Errorf("sync lock file: %w", err)
	}
	return nil
}

// readLockOwner parses the diagnostic identity from an already-open lock file.
// Missing or malformed fields degrade gracefully to a zero-valued owner.
func readLockOwner(f *os.File) lockOwner {
	owner := lockOwner{pid: -1}
	buf := make([]byte, 256)
	n, _ := f.ReadAt(buf, 0)
	for _, line := range strings.Split(string(buf[:n]), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "pid="):
			if v, err := strconv.Atoi(strings.TrimPrefix(line, "pid=")); err == nil {
				owner.pid = v
			}
		case strings.HasPrefix(line, "started="):
			owner.startedAt = strings.TrimPrefix(line, "started=")
		}
	}
	return owner
}

// formatLockOwner renders a human-readable suffix for the "already running"
// error, or an empty string when no useful identity was recorded.
func formatLockOwner(o lockOwner) string {
	switch {
	case o.pid > 0 && o.startedAt != "":
		return fmt.Sprintf(" (PID %d, started %s)", o.pid, o.startedAt)
	case o.pid > 0:
		return fmt.Sprintf(" (PID %d)", o.pid)
	default:
		return ""
	}
}

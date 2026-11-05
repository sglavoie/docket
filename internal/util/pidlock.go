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
// The PID, start identity, and session kind written into the file never decide
// exclusion: they ride the "already running" refusal, a *LockHeldError, so a
// human or a second launch can identify the holder.
type PIDLock struct {
	file *os.File
}

// The sessions a TUI process lock may name. A holder that names none — a
// workspace move, or a build older than the field — is never handed off to.
const (
	// SessionTerminal is an ordinary TUI drawn in a terminal. On macOS it
	// answers SIGUSR1 by asking its terminal to raise the window, which is part
	// of how a second launch hands off to it.
	SessionTerminal = "terminal"
	// SessionHeadless is a --headless session Docket.app runs with no terminal.
	SessionHeadless = "headless"
)

// lockOwner is the diagnostic identity recorded in a lock file. startedAt tags
// the moment the lock was acquired so a reused PID can be told apart from the
// original holder when inspecting the file. session is what kind of process
// holds it, empty when the holder recorded none.
type lockOwner struct {
	pid       int
	startedAt string
	session   string
}

// LockHeldError is AcquirePIDLock's refusal when another live process holds the
// lock. It carries the identity the holder recorded, so a caller can try to
// reach that holder; its message is the "already running" diagnostic.
type LockHeldError struct {
	// PID is the holder's recorded process ID, or -1 when none was recorded.
	PID int
	// StartedAt is the holder's recorded acquisition time, RFC3339 UTC.
	StartedAt string
	// Session is the kind of session the holder recorded, one of the Session
	// constants, or empty.
	Session string
}

func (e *LockHeldError) Error() string {
	return "docket TUI already running" + formatLockOwner(lockOwner{pid: e.PID, startedAt: e.StartedAt})
}

// AcquirePIDLock opens (creating if needed) the lock file at path and takes an
// exclusive, non-blocking OS lock on it. It returns an error if another live
// process already holds the lock. On success the returned *PIDLock keeps the
// underlying file descriptor open — the OS lock is held until Release is called
// or the process exits.
func AcquirePIDLock(path string) (*PIDLock, error) {
	return AcquireSessionLock(path, "")
}

// AcquireSessionLock is AcquirePIDLock for a TUI session, recording session
// beside the PID so a refused second launch knows what it found. An empty
// session records none.
func AcquireSessionLock(path, session string) (*PIDLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, FilePermissions)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	if lockErr := flockExclusiveNB(f); lockErr != nil {
		if errors.Is(lockErr, errLockWouldBlock) {
			owner := readLockOwner(f)
			_ = f.Close()
			return nil, &LockHeldError{PID: owner.pid, StartedAt: owner.startedAt, Session: owner.session}
		}
		_ = f.Close()
		return nil, fmt.Errorf("acquire lock: %w", lockErr)
	}

	if err := writeLockOwner(f, session); err != nil {
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
// identity for diagnostics, and the session kind when one is named.
func writeLockOwner(f *os.File, session string) error {
	body := fmt.Sprintf("pid=%d\nstarted=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if session != "" {
		body += "session=" + session + "\n"
	}
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
		case strings.HasPrefix(line, "session="):
			owner.session = strings.TrimPrefix(line, "session=")
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

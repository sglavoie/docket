package util

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// File permission constants used across the codebase.
const (
	DirPermissions  = 0755
	FilePermissions = 0644
)

// AtomicWriteFile writes data to a file atomically using a temporary file and rename.
// This prevents data corruption if the write is interrupted (e.g., power loss, crash).
// The file is created with the specified permissions.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Create temp file in the same directory to ensure atomic rename works
	// (rename across filesystems is not atomic)
	tmp, err := os.CreateTemp(dir, base+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on any error
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	// Write data to temp file
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	// Sync to ensure data is on disk before rename
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	// Set permissions on temp file before rename
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	success = true
	return nil
}

// WithFileLock acquires an advisory file lock on lockPath, runs fn, then releases
// the lock. If exclusive is true, an exclusive (write) lock is acquired; otherwise
// a shared (read) lock is used. The lock file is created if it does not exist.
func WithFileLock(lockPath string, exclusive bool, fn func() error) error {
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, FilePermissions)
	if err != nil {
		return fmt.Errorf("failed to open lock file: %w", err)
	}
	defer func() { _ = f.Close() }()

	how := syscall.LOCK_SH
	if exclusive {
		how = syscall.LOCK_EX
	}

	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN) //nolint:errcheck

	return fn()
}

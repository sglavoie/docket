package util

import (
	"fmt"
	"os"
	"path/filepath"
)

// File permission constants used across the codebase. Personal task data is
// private by default: directories are owner-only (0700) and files are
// owner-read/write (0600). Existing stricter modes are preserved on rewrite
// (see AtomicWriteFile); the defaults only tighten looser modes down to private.
const (
	DirPermissions  = 0700
	FilePermissions = 0600
)

// syncDir fsyncs a directory so that a create or rename within it is durably
// recorded in the directory entry, surviving a crash immediately after the
// operation returns. It is a package variable so tests can observe or stub the
// rename durability hook.
var syncDir = func(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

// AtomicWriteFile writes data to a file atomically using a temporary file and rename.
// This prevents data corruption if the write is interrupted (e.g., power loss, crash).
// The file is created with the specified permissions, except that a stricter mode
// on an existing destination is preserved rather than loosened. After the rename
// the parent directory is fsynced so the replacement is durable.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)

	// Preserve stricter existing modes: if the destination already exists with
	// more restrictive permissions, keep those bits rather than loosening them.
	// ANDing yields a mode no looser than either the requested default or the
	// existing file, so a user-hardened file (e.g. 0400) is never widened.
	if info, statErr := os.Stat(path); statErr == nil {
		perm &= info.Mode().Perm()
	}

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

	// Fsync the parent directory so the rename is durably recorded and cannot be
	// lost by a crash right after this call returns.
	if err := syncDir(dir); err != nil {
		return fmt.Errorf("failed to sync directory: %w", err)
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

	lock := flockShared
	if exclusive {
		lock = flockExclusive
	}

	if err := lock(f); err != nil {
		return fmt.Errorf("failed to acquire file lock: %w", err)
	}
	defer func() { _ = flockUnlock(f) }()

	return fn()
}

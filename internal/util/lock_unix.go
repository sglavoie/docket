//go:build darwin || linux

package util

import (
	"errors"
	"os"
	"syscall"
)

// errLockWouldBlock reports that a non-blocking exclusive lock attempt found the
// lock already held by another open file description (i.e. a live holder).
var errLockWouldBlock = errors.New("lock would block")

// flockShared acquires a blocking shared (read) advisory lock.
func flockShared(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_SH)
}

// flockExclusive acquires a blocking exclusive (write) advisory lock.
func flockExclusive(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX)
}

// flockExclusiveNB acquires an exclusive advisory lock without blocking. If the
// lock is already held it returns errLockWouldBlock; other failures are returned
// as-is.
func flockExclusiveNB(f *os.File) error {
	err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return errLockWouldBlock
	}
	return err
}

// flockUnlock releases any advisory lock held on f.
func flockUnlock(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

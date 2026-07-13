//go:build !darwin && !linux

package util

import (
	"errors"
	"os"
)

// errLockWouldBlock keeps the cross-platform contention sentinel available so
// callers can errors.Is against it even where advisory locking is unsupported.
var errLockWouldBlock = errors.New("lock would block")

// errLockUnsupported is returned by the locking primitives on platforms without
// a supported flock implementation. tt targets macOS and Linux; this stub only
// exists so the util package still compiles elsewhere.
var errLockUnsupported = errors.New("file locking not supported on this platform")

func flockShared(*os.File) error { return errLockUnsupported }

func flockExclusive(*os.File) error { return errLockUnsupported }

func flockExclusiveNB(*os.File) error { return errLockUnsupported }

func flockUnlock(*os.File) error { return nil }

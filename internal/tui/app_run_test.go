package tui

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/config"
)

// TestPIDLockPathIsLocalAndDataDirSpecific verifies the TUI PID lock stays under
// the machine-local runtime root (never the synced data dir) and that distinct
// data dirs resolve to distinct lock paths.
func TestPIDLockPathIsLocalAndDataDirSpecific(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	local := config.LocalDataRoot()

	// When the data dir is the local root, the lock lives directly under it.
	if got, want := pidLockPath(local), filepath.Join(local, "pid.lock"); got != want {
		t.Errorf("pidLockPath(local) = %q, want %q", got, want)
	}

	// An iCloud-style (non-local) data dir must not host the lock; it lives under
	// the local runtime dir instead.
	icloud := filepath.Join(home, "Library", "Mobile Documents", "com~apple~CloudDocs", "tt")
	lock := pidLockPath(icloud)
	if strings.HasPrefix(lock, filepath.Clean(icloud)) {
		t.Errorf("pidLockPath(icloud) = %q must not live under the synced data dir", lock)
	}
	if !strings.HasPrefix(lock, filepath.Clean(local)) {
		t.Errorf("pidLockPath(icloud) = %q must live under the local root %q", lock, local)
	}

	// Data-dir-specific: a different data dir yields a different lock path.
	if other := pidLockPath(icloud + "-other"); other == lock {
		t.Errorf("distinct data dirs must yield distinct lock paths, both = %q", lock)
	}
}

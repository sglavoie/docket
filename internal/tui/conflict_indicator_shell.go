package tui

import (
	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// The shell half of the outstanding-conflict indicator. conflict_indicator.go
// owns the line the Outliner's status strip draws; this file owns the count it
// draws, which is cached here so no render reads the filesystem.
//
// The store is read straight from the storage layer rather than through an
// internal/app port, following the direct storage.LoadProject reads the TUI
// already makes: nothing here validates, mutates, or commits, so there is no
// application-layer policy for a port to hold.

// countOutstandingConflicts reports how many conflict bundles the workspace has
// recorded. An absent directory is zero, not an error, and a read failure is
// also zero: an indicator that cannot be counted must not be asserted.
func countOutstandingConflicts(dataDir string) int {
	if dataDir == "" {
		return 0
	}
	records, err := storage.NewConflictStore(config.ConflictsDir(dataDir)).List()
	if err != nil {
		logging.Warn("counting outstanding conflicts: %v", err)
		return 0
	}
	return len(records)
}

// refreshOutstandingConflicts recounts the workspace's conflict bundles. It runs
// at launch and on the existing 60s periodic tick, which is the same cadence the
// rest of the shell's workspace state is refreshed on; it arms no timer of its
// own.
func (m *AppModel) refreshOutstandingConflicts() {
	if m == nil || m.runtime == nil {
		return
	}
	m.outstandingConflicts = countOutstandingConflicts(m.runtime.DataDir())
}

// outstandingConflictCount is the accessor injected into every Outliner, so the
// strip reads one shell-owned count instead of touching the filesystem during a
// render.
func (m *AppModel) outstandingConflictCount() int {
	if m == nil {
		return 0
	}
	return m.outstandingConflicts
}

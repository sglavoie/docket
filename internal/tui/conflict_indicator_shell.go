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
	if m == nil || m.domain.runtime == nil {
		return
	}
	m.feedback.outstandingConflicts = countOutstandingConflicts(m.domain.runtime.DataDir())
}

// The count itself lives on the feedback cluster, which also supplies the
// accessor injected into every Outliner; see feedback_controller.go.

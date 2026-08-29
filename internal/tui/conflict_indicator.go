package tui

import (
	"fmt"

	"github.com/sglavoie/tt/internal/config"
	"github.com/sglavoie/tt/internal/logging"
	"github.com/sglavoie/tt/internal/storage"
)

// The outstanding-conflict indicator is workspace state, not an event: it is a
// line the status strip carries for as long as `.tt/conflicts/` holds a bundle,
// and it disappears once the CLI resolves the last one. It is deliberately read
// straight from the storage layer rather than through an internal/app port,
// following the direct storage.LoadProject reads the TUI already makes: nothing
// here validates, mutates, or commits, so there is no application-layer policy
// for a port to hold.

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

// conflictIndicatorText is the strip's line for a given count. Zero renders
// nothing at all, so the strip keeps its height only while there is state to
// report.
func conflictIndicatorText(count int) string {
	switch {
	case count <= 0:
		return ""
	case count == 1:
		return "1 unresolved merge conflict — review with tt conflict list"
	default:
		return fmt.Sprintf("%d unresolved merge conflicts — review with tt conflict list", count)
	}
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

// setOutstandingConflicts injects the shell's conflict accessor. Like the note
// index it is a setter rather than a constructor parameter, because the Hub's
// preview painter builds an Outliner that must keep the nil accessor.
func (m *OutlinerModel) setOutstandingConflicts(accessor func() int) {
	m.outstandingConflicts = accessor
}

// conflictIndicator is the strip's line for the injected count.
func (m *OutlinerModel) conflictIndicator() string {
	if m == nil || m.outstandingConflicts == nil {
		return ""
	}
	return conflictIndicatorText(m.outstandingConflicts())
}

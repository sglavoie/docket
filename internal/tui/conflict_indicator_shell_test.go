package tui

import (
	"testing"
	"time"

	"github.com/sglavoie/docket/internal/config"
	"github.com/sglavoie/docket/internal/storage"
	"github.com/sglavoie/docket/internal/tui/status"
)

// The shell half of the outstanding-conflict coverage: counting bundles on
// disk, refreshing that count on the 60s tick, and staying nil-safe. How the
// count renders stays in conflict_indicator_test.go.

// recordTestConflict writes one bundle into the workspace's conflict directory
// and returns its id. The documents are irrelevant to the indicator, which reads
// only how many bundles exist.
func recordTestConflict(t *testing.T, store *storage.ConflictStore, projectName string) string {
	t.Helper()
	id, err := store.Save(storage.ConflictRecord{ProjectName: projectName}, nil, nil, nil)
	if err != nil {
		t.Fatalf("save conflict bundle: %v", err)
	}
	return id
}

// TestOutstandingConflictCountTracksTheStore covers the three states the
// indicator has to distinguish: a workspace that never recorded a conflict has
// no `.docket/conflicts/` directory at all, a workspace holding bundles counts them,
// and resolving the last one through the store returns the count to zero.
func TestOutstandingConflictCountTracksTheStore(t *testing.T) {
	dataDir := t.TempDir()
	if got := countOutstandingConflicts(dataDir); got != 0 {
		t.Fatalf("count with no conflicts directory = %d, want 0", got)
	}
	if got := countOutstandingConflicts(""); got != 0 {
		t.Fatalf("count with no data directory = %d, want 0", got)
	}

	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	first := recordTestConflict(t, store, "Alpha")
	if got := countOutstandingConflicts(dataDir); got != 1 {
		t.Fatalf("count after one bundle = %d, want 1", got)
	}

	second := recordTestConflict(t, store, "Beta")
	if got := countOutstandingConflicts(dataDir); got != 2 {
		t.Fatalf("count after two bundles = %d, want 2", got)
	}

	if err := store.Delete(first); err != nil {
		t.Fatalf("delete first conflict: %v", err)
	}
	if got := countOutstandingConflicts(dataDir); got != 1 {
		t.Fatalf("count after resolving one = %d, want 1", got)
	}
	if err := store.Delete(second); err != nil {
		t.Fatalf("delete second conflict: %v", err)
	}
	if got := countOutstandingConflicts(dataDir); got != 0 {
		t.Fatalf("count after resolving both = %d, want 0", got)
	}
}

// TestConflictIndicatorRefreshesOnThePeriodicTick holds the indicator to the
// existing 60s cadence: a bundle written by another machine while the TUI is
// open becomes visible on the next periodic tick, and clears on the tick after
// the CLI resolves it. No new timer is armed — the handler re-arms the same
// periodicSaveMsg it always did.
func TestConflictIndicatorRefreshesOnThePeriodicTick(t *testing.T) {
	dataDir := t.TempDir()
	shell := &AppModel{domain: domainController{runtime: testRuntimeAt(dataDir, newTestConfig())}}

	shell.refreshOutstandingConflicts()
	if got := shell.feedback.outstandingConflictCount(); got != 0 {
		t.Fatalf("count before any conflict = %d, want 0", got)
	}

	store := storage.NewConflictStore(config.ConflictsDir(dataDir))
	id := recordTestConflict(t, store, "Alpha")

	handled, cmd := shell.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()})
	if !handled {
		t.Fatal("periodicSaveMsg was not handled")
	}
	if cmd == nil {
		t.Fatal("the periodic tick must re-arm itself")
	}
	if got := shell.feedback.outstandingConflictCount(); got != 1 {
		t.Fatalf("count after the tick = %d, want 1", got)
	}

	if err := store.Delete(id); err != nil {
		t.Fatalf("resolve conflict: %v", err)
	}
	if _, cmd = shell.handleGlobalSaveMessage(status.PeriodicSaveMsg{At: time.Now()}); cmd == nil {
		t.Fatal("the periodic tick must re-arm itself after a resolution")
	}
	if got := shell.feedback.outstandingConflictCount(); got != 0 {
		t.Fatalf("count after the conflict was resolved = %d, want 0", got)
	}
}

// TestNilShellReportsNoOutstandingConflicts keeps the injected accessor safe for
// the construction sites that never own a shell. The count now lives on the
// feedback cluster, so that is where the nil answer is pinned; the refresh stays
// on AppModel and stays nil-safe beside it.
func TestNilShellReportsNoOutstandingConflicts(t *testing.T) {
	var cluster *feedbackController
	if got := cluster.outstandingConflictCount(); got != 0 {
		t.Fatalf("nil feedback cluster count = %d, want 0", got)
	}

	var shell *AppModel
	shell.refreshOutstandingConflicts()
}

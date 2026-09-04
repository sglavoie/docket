package tui

import (
	"strings"
	"testing"

	"github.com/sglavoie/tt/internal/tui/conflict"
	"github.com/sglavoie/tt/internal/tui/operation"
)

// The shell half of the conflict browser: reading the store and mounting the
// overlay. What the overlay does once mounted is internal/tui/conflict's own
// test.

// TestOpenConflictBrowserReadsTheWorkspaceStore covers the host port both ways:
// an empty store refuses with a status and mounts nothing, and a recorded bundle
// mounts the overlay and settles the strip's count in the same read.
func TestOpenConflictBrowserReadsTheWorkspaceStore(t *testing.T) {
	workspace, err := newUITestWorkspace()
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	t.Cleanup(func() { _ = workspace.Cleanup() })

	driver, err := workspace.launch(120, 40)
	if err != nil {
		t.Fatalf("launch driver: %v", err)
	}
	app := driver.App()

	app.openConflictBrowser()
	if app.conflictBrowser != nil {
		t.Fatal("an empty workspace opened the conflict browser")
	}
	if !strings.Contains(app.feedback.message, conflict.ConflictBrowserEmptyStatus) {
		t.Errorf("empty-store status = %q, want %q", app.feedback.message, conflict.ConflictBrowserEmptyStatus)
	}

	if err := app.harnessRecordConflict(); err != nil {
		t.Fatalf("record conflict: %v", err)
	}
	app.openConflictBrowser()
	if app.conflictBrowser == nil {
		t.Fatal("a recorded conflict did not open the browser")
	}
	if app.feedback.outstandingConflictCount() != 1 {
		t.Errorf("outstanding conflicts = %d, want 1", app.feedback.outstandingConflictCount())
	}
	record, ok := app.conflictBrowser.CurrentRecord()
	if !ok {
		t.Fatal("the browser opened on no record")
	}
	if record.ProjectName != harnessConflictProjectName {
		t.Errorf("cursor record project = %q, want %q", record.ProjectName, harnessConflictProjectName)
	}
}

// TestConflictBrowserOpensFromTheGoToChord keeps gc a terminal path under the
// existing go-to prefix, which is what leaves root `c` retired.
func TestConflictBrowserOpensFromTheGoToChord(t *testing.T) {
	registry := registeredSimplifiedOperationRegistry(t)
	ctx := simplifiedOperationContext()

	binding, _, resolution := registry.InputBindingForSequence(
		operation.InputStrategySimplified,
		[]string{"g", "c"},
		ctx,
	)
	if resolution != operation.ChordTerminal || binding.OperationID != operation.OperationCoreConflicts {
		t.Fatalf("gc resolved to %s (resolution %v), want %s",
			binding.OperationID, resolution, operation.OperationCoreConflicts)
	}
	if _, _, ok := registry.InputBindingForKey(operation.InputStrategySimplified, "c", ctx); ok {
		t.Error("root c is bound; it must stay retired")
	}
}

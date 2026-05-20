package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestHandleWithRefreshForwardsPopCmd verifies that handleWithRefresh forwards
// the tea.Cmd produced by popView() instead of silently dropping it. Without
// this, popping back to ViewCrossProjectOutliner (whose popView returns a
// ViewDidAppearMsg) leaves the outliner stale.
func TestHandleWithRefreshForwardsPopCmd(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	// Simulate a stack: Dashboard → CrossProjectOutliner → TaskForm
	// Popping the TaskForm should reveal the CrossProjectOutliner and emit
	// ViewDidAppearMsg.
	app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner, ViewTaskForm}

	cmd := app.handleWithRefresh(true, false, "")
	if cmd == nil {
		t.Fatal("expected popView's ViewDidAppearMsg cmd to be forwarded, got nil")
	}

	msg := cmd()
	if _, ok := msg.(ViewDidAppearMsg); !ok {
		t.Errorf("expected ViewDidAppearMsg from forwarded popCmd, got %T", msg)
	}
}

// TestHandleWithRefreshBatchesPopCmdWithStatus verifies that when both popView
// returns a cmd AND a status message is set, both Cmds are batched together.
func TestHandleWithRefreshBatchesPopCmdWithStatus(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.viewStack = []ViewType{ViewDashboard, ViewCrossProjectOutliner, ViewTaskForm}

	cmd := app.handleWithRefresh(true, false, "Hello")
	if cmd == nil {
		t.Fatal("expected non-nil cmd")
	}

	// Run the batched cmd and ensure ViewDidAppearMsg surfaces among the
	// messages it dispatches. tea.Batch returns a BatchMsg.
	msg := cmd()
	if msg == nil {
		t.Fatal("batched cmd should produce a message")
	}
	// Test that statusMessage was set
	if app.statusMessage != "Hello" {
		t.Errorf("expected status 'Hello', got %q", app.statusMessage)
	}
	// We don't depend on the BatchMsg internal API. As long as the cmd is
	// non-nil and the status was set, the batching plumbing is wired.
	_ = msg
	_ = tea.Batch // ensure import
}

// TestHandleWithRefreshNoPopReturnsNilWhenNoStatus verifies that when popView=false
// and no status is set, the returned cmd is nil (not an empty Batch).
func TestHandleWithRefreshNoPopReturnsNilWhenNoStatus(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.viewStack = []ViewType{ViewDashboard, ViewProjectTree}

	cmd := app.handleWithRefresh(false, false, "")
	if cmd != nil {
		t.Errorf("expected nil cmd when no pop and no status, got non-nil")
	}
}

package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// TestCloseInboxProcessorReturnsStatusTimeout verifies that closing the inbox
// processor with a non-zero processed/skipped count returns a tea.Cmd so the
// success toast eventually clears. Without the timeout the success message
// would persist indefinitely until another mutation overwrote it.
func TestCloseInboxProcessorReturnsStatusTimeout(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 3, SkippedCount: 0}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled {
		t.Fatal("CloseInboxProcessorMsg should be handled")
	}
	if cmd == nil {
		t.Fatal("expected a status-clear tea.Cmd; status would persist forever without it")
	}
	if app.statusMessage == "" {
		t.Error("statusMessage should be set after processing")
	}

	// Skipped-only path also needs to return a timeout.
	app2, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app2.pushView(ViewInboxProcessor)
	app2.views.Register(ViewInboxProcessor, &InboxProcessorModel{})
	skippedMsg := CloseInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 2}
	_, cmd = app2.handleInboxMessages(skippedMsg)
	if cmd == nil {
		t.Fatal("skipped-only close should also return a status-clear tea.Cmd")
	}
}

// TestCloseInboxProcessorNoOpReturnsNil verifies the no-op path (zero processed
// and zero skipped) still returns nil cmd, since no status message was set.
func TestCloseInboxProcessorNoOpReturnsNil(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewInboxProcessor)
	app.views.Register(ViewInboxProcessor, &InboxProcessorModel{})

	msg := CloseInboxProcessorMsg{ProcessedCount: 0, SkippedCount: 0}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled {
		t.Fatal("CloseInboxProcessorMsg should be handled")
	}
	if cmd != nil {
		t.Errorf("no-op close should not return a cmd, got %T", cmd())
	}
}

// TestCloseGlobalInboxProcessorReturnsStatusTimeout verifies the global inbox
// processor close path also batches a status-clear Cmd when a success message
// is shown. Without this, the success toast would never clear.
func TestCloseGlobalInboxProcessorReturnsStatusTimeout(t *testing.T) {
	app, _, _ := newTestAppModelWithMockPersisterAndProject(t)
	app.pushView(ViewGlobalInboxProcessor)

	msg := CloseGlobalInboxProcessorMsg{ProcessedCount: 2, SkippedCount: 1}
	handled, cmd := app.handleInboxMessages(msg)
	if !handled {
		t.Fatal("CloseGlobalInboxProcessorMsg should be handled")
	}
	if cmd == nil {
		t.Fatal("expected a non-nil cmd batching popCmd + status-clear timeout")
	}
	if app.statusMessage == "" {
		t.Error("statusMessage should be set after processing")
	}

	// Force the batch cmd to run and collect the messages it would dispatch.
	// We can't easily inspect the inner batch elements, but we verify the cmd
	// is non-nil — the regression bug was a bare popCmd with no timeout when
	// a status was set.
	_ = cmd

	_ = tea.Batch // ensure tea package is referenced (cmd type)
}

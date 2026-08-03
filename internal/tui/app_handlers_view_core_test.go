package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
)

func TestHandleViewMessagesBack(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCrossProjectOutliner)
	app.pushView(ViewTaskForm)

	msg := BackMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BackMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after back, got %v", app.currentView())
	}
}

func TestHandleViewMessagesError(t *testing.T) {
	app := newTestAppModel()
	testErr := errors.New("task not found")

	msg := ErrorMsg{Err: testErr}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ErrorMsg should be handled")
	}
	if !errors.Is(app.err, testErr) {
		t.Error("app.err should be set to the error from message")
	}
}

func TestHandleViewMessagesStatusClear(t *testing.T) {
	app := newTestAppModel()
	app.statusMessage = "Some status"

	msg := StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.statusMessage)
	}
}

func TestHandleViewMessagesRefresh(t *testing.T) {
	app := newTestAppModel()

	msg := RefreshMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RefreshMsg should be handled")
	}
}

func TestHandleViewMessagesStatusMsg(t *testing.T) {
	app := newTestAppModel()

	msg := StatusMsg("Operation completed")
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusMsg should be handled")
	}
	if app.statusMessage != "Operation completed" {
		t.Errorf("statusMessage should be 'Operation completed', got %q", app.statusMessage)
	}
	if app.statusIsInfo {
		t.Error("statusIsInfo should be false for StatusMsg")
	}
	if cmd == nil {
		t.Error("StatusMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesInfoStatusMsg(t *testing.T) {
	app := newTestAppModel()

	msg := InfoStatusMsg("Some info")
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("InfoStatusMsg should be handled")
	}
	if app.statusMessage != "Some info" {
		t.Errorf("statusMessage should be 'Some info', got %q", app.statusMessage)
	}
	if !app.statusIsInfo {
		t.Error("statusIsInfo should be true for InfoStatusMsg")
	}
	if cmd == nil {
		t.Error("InfoStatusMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesStatusClearResetsInfoFlag(t *testing.T) {
	app := newTestAppModel()
	app.statusMessage = "Some status"
	app.statusIsInfo = true

	msg := StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.statusMessage)
	}
	if app.statusIsInfo {
		t.Error("statusIsInfo should be cleared by StatusClearMsg")
	}
}

func TestSetUndoableStatusStoresCallbackAndExpiry(t *testing.T) {
	app := newTestAppModel()
	called := false
	cmd := app.setUndoableStatus("Deleted '#1 thing' · u to undo", func() tea.Cmd {
		called = true
		return nil
	})

	if app.statusMessage == "" {
		t.Error("statusMessage should be set")
	}
	if !app.statusIsInfo {
		t.Error("undo toast should be info-styled")
	}
	if app.statusUndoCb == nil {
		t.Error("statusUndoCb should be stored")
	}
	if !time.Now().Before(app.statusExpiresAt) {
		t.Error("statusExpiresAt should be in the future")
	}
	if cmd == nil {
		t.Error("setUndoableStatus should return a tick cmd")
	}
	app.statusUndoCb()
	if !called {
		t.Error("stored callback should be invokable")
	}
}

func TestUndoToastTimeoutClearsExpiredToast(t *testing.T) {
	app := newTestAppModel()
	_ = app.setUndoableStatus("toast", func() tea.Cmd { return nil })
	// Force the toast to look already-expired so the timeout handler clears it.
	app.statusExpiresAt = time.Now().Add(-time.Second)

	handled, _ := app.handleViewMessages(UndoToastTimeoutMsg{})
	if !handled {
		t.Error("UndoToastTimeoutMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("expired toast should be cleared, got %q", app.statusMessage)
	}
	if app.statusUndoCb != nil {
		t.Error("statusUndoCb should be nil after timeout clear")
	}
}

func TestUndoToastTimeoutLeavesReplacedToastAlone(t *testing.T) {
	app := newTestAppModel()
	// First toast — its tick is still pending.
	_ = app.setUndoableStatus("first", func() tea.Cmd { return nil })
	firstExpiry := app.statusExpiresAt
	// Replacement toast extends the deadline.
	_ = app.setUndoableStatus("second", func() tea.Cmd { return nil })

	if !app.statusExpiresAt.After(firstExpiry) {
		t.Fatal("replacement toast should push expiresAt later")
	}

	// First tick fires now (Now() < replacement expiresAt) — must NOT clear.
	handled, _ := app.handleViewMessages(UndoToastTimeoutMsg{})
	if !handled {
		t.Error("UndoToastTimeoutMsg should be handled")
	}
	if !strings.Contains(app.statusMessage, "second") {
		t.Errorf("replacement toast should remain, got %q", app.statusMessage)
	}
}

// TestStatusClearMsgPreservesActiveUndoToast verifies that a stale
// StatusClearMsg from an earlier 2s plain-status timer does not prematurely
// dismiss a newer undo toast that is still within its 6s window. Without
// guarding StatusClearMsg against an active undo toast, the sequence
// "plain status → undo toast within 2s" loses the toast the moment the
// plain status's tick fires.
func TestStatusClearMsgPreservesActiveUndoToast(t *testing.T) {
	app := newTestAppModel()

	// Plain status arms a 2s StatusClearMsg tick.
	app.setSuccessStatus("saved")
	// Before the 2s tick fires, a destructive action installs an undo toast.
	called := false
	_ = app.setUndoableStatus("destructive · u to undo", func() tea.Cmd {
		called = true
		return nil
	})

	// The earlier StatusClearMsg now arrives. It must NOT clear the live toast.
	handled, _ := app.handleViewMessages(StatusClearMsg{})
	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage == "" {
		t.Fatal("stale StatusClearMsg cleared a live undo toast")
	}
	if app.statusUndoCb == nil {
		t.Fatal("stale StatusClearMsg dropped the undo callback for a live toast")
	}
	if !strings.Contains(app.statusMessage, "destructive") {
		t.Errorf("expected toast preserved, got %q", app.statusMessage)
	}
	// `u` must still invoke the toast callback.
	app.statusUndoCb()
	if !called {
		t.Error("undo callback should still be invokable after stale StatusClearMsg")
	}
}

func TestStatusClearMsgDoesNotReplaceNewerStatusWithFooter(t *testing.T) {
	app := newTestAppModel()
	app.setSuccessStatus("first")
	scheduledAt := time.Now()
	app.setInfoStatus("newer guidance")

	handled, _ := app.handleViewMessages(StatusClearMsg{ScheduledAt: scheduledAt})
	if !handled {
		t.Fatal("StatusClearMsg should be handled")
	}
	if app.statusMessage != "newer guidance" {
		t.Fatalf("stale clear replaced newer status with the contextual footer: %q", app.statusMessage)
	}
}

func TestStatusClearMsgPreservesStatusWithSameTimestamp(t *testing.T) {
	app := newTestAppModel()
	app.setInfoStatus("coalesced-clock guidance")
	scheduledAt := app.statusUpdatedAt

	handled, _ := app.handleViewMessages(StatusClearMsg{ScheduledAt: scheduledAt})
	if !handled {
		t.Fatal("StatusClearMsg should be handled")
	}
	if app.statusMessage != "coalesced-clock guidance" {
		t.Fatalf("same-timestamp stale clear replaced newer status: %q", app.statusMessage)
	}
}

// TestStatusClearMsgClearsExpiredUndoToast verifies that StatusClearMsg still
// clears the screen when the undo toast has already expired — otherwise an
// expired toast string could linger indefinitely.
func TestStatusClearMsgClearsExpiredUndoToast(t *testing.T) {
	app := newTestAppModel()
	_ = app.setUndoableStatus("old", func() tea.Cmd { return nil })
	app.statusExpiresAt = time.Now().Add(-time.Second)

	handled, _ := app.handleViewMessages(StatusClearMsg{})
	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.statusMessage != "" {
		t.Errorf("expired toast should be cleared by StatusClearMsg, got %q", app.statusMessage)
	}
}

func TestFormatUndoToastIncludesContext(t *testing.T) {
	got := formatUndoToast("Delete Task", "#42 Login flow")
	want := "Delete Task '#42 Login flow' · u to undo"
	if got != want {
		t.Errorf("formatUndoToast = %q, want %q", got, want)
	}
	if formatUndoToast("Bulk Edit", "") != "Bulk Edit · u to undo" {
		t.Error("formatUndoToast should omit empty context")
	}
}

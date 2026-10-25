package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/status"
)

func TestHandleViewMessagesBack(t *testing.T) {
	app := newTestAppModel()
	app.pushView(presentation.ViewCrossProjectOutliner)
	app.pushView(presentation.ViewTaskComposer)

	msg := status.BackMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BackMsg should be handled")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after back, got %v", app.currentView())
	}
}

func TestHandleViewMessagesError(t *testing.T) {
	app := newTestAppModel()
	testErr := errors.New("task not found")

	msg := status.ErrorMsg{Err: testErr}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ErrorMsg should be handled")
	}
	if !errors.Is(app.feedback.err, testErr) {
		t.Error("app.feedback.err should be set to the error from message")
	}
}

func TestHandleViewMessagesStatusClear(t *testing.T) {
	app := newTestAppModel()
	app.feedback.message = "Some status"

	msg := status.StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.feedback.message != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.feedback.message)
	}
}

func TestHandleViewMessagesStatusMsg(t *testing.T) {
	app := newTestAppModel()

	msg := status.StatusMsg("Operation completed")
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusMsg should be handled")
	}
	if app.feedback.message != "Operation completed" {
		t.Errorf("statusMessage should be 'Operation completed', got %q", app.feedback.message)
	}
	if app.feedback.info {
		t.Error("statusIsInfo should be false for StatusMsg")
	}
	if cmd == nil {
		t.Error("StatusMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesInfoStatusMsg(t *testing.T) {
	app := newTestAppModel()

	msg := status.InfoStatusMsg("Some info")
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("InfoStatusMsg should be handled")
	}
	if app.feedback.message != "Some info" {
		t.Errorf("statusMessage should be 'Some info', got %q", app.feedback.message)
	}
	if !app.feedback.info {
		t.Error("statusIsInfo should be true for InfoStatusMsg")
	}
	if cmd == nil {
		t.Error("InfoStatusMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesStatusClearResetsInfoFlag(t *testing.T) {
	app := newTestAppModel()
	app.feedback.message = "Some status"
	app.feedback.info = true

	msg := status.StatusClearMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.feedback.message != "" {
		t.Errorf("statusMessage should be cleared, got %q", app.feedback.message)
	}
	if app.feedback.info {
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

	if app.feedback.message == "" {
		t.Error("statusMessage should be set")
	}
	if !app.feedback.info {
		t.Error("undo toast should be info-styled")
	}
	if app.feedback.undoCb == nil {
		t.Error("statusUndoCb should be stored")
	}
	if !time.Now().Before(app.feedback.expiresAt) {
		t.Error("statusExpiresAt should be in the future")
	}
	if cmd == nil {
		t.Error("setUndoableStatus should return a tick cmd")
	}
	app.feedback.undoCb()
	if !called {
		t.Error("stored callback should be invokable")
	}
}

func TestUndoToastTimeoutClearsExpiredToast(t *testing.T) {
	app := newTestAppModel()
	_ = app.setUndoableStatus("toast", func() tea.Cmd { return nil })
	// Force the toast to look already-expired so the timeout handler clears it.
	app.feedback.expiresAt = time.Now().Add(-time.Second)

	handled, _ := app.handleViewMessages(status.UndoToastTimeoutMsg{})
	if !handled {
		t.Error("UndoToastTimeoutMsg should be handled")
	}
	if app.feedback.message != "" {
		t.Errorf("expired toast should be cleared, got %q", app.feedback.message)
	}
	if app.feedback.undoCb != nil {
		t.Error("statusUndoCb should be nil after timeout clear")
	}
}

func TestUndoToastTimeoutLeavesReplacedToastAlone(t *testing.T) {
	app := newTestAppModel()
	// First toast — its tick is still pending.
	_ = app.setUndoableStatus("first", func() tea.Cmd { return nil })
	firstExpiry := app.feedback.expiresAt
	// Replacement toast extends the deadline.
	_ = app.setUndoableStatus("second", func() tea.Cmd { return nil })

	if !app.feedback.expiresAt.After(firstExpiry) {
		t.Fatal("replacement toast should push expiresAt later")
	}

	// First tick fires now (Now() < replacement expiresAt) — must NOT clear.
	handled, _ := app.handleViewMessages(status.UndoToastTimeoutMsg{})
	if !handled {
		t.Error("UndoToastTimeoutMsg should be handled")
	}
	if !strings.Contains(app.feedback.message, "second") {
		t.Errorf("replacement toast should remain, got %q", app.feedback.message)
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
	handled, _ := app.handleViewMessages(status.StatusClearMsg{})
	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.feedback.message == "" {
		t.Fatal("stale StatusClearMsg cleared a live undo toast")
	}
	if app.feedback.undoCb == nil {
		t.Fatal("stale StatusClearMsg dropped the undo callback for a live toast")
	}
	if !strings.Contains(app.feedback.message, "destructive") {
		t.Errorf("expected toast preserved, got %q", app.feedback.message)
	}
	// `u` must still invoke the toast callback.
	app.feedback.undoCb()
	if !called {
		t.Error("undo callback should still be invokable after stale StatusClearMsg")
	}
}

func TestStatusClearMsgDoesNotReplaceNewerStatusWithFooter(t *testing.T) {
	app := newTestAppModel()
	app.setSuccessStatus("first")
	scheduledAt := time.Now()
	app.setInfoStatus("newer guidance")

	handled, _ := app.handleViewMessages(status.StatusClearMsg{ScheduledAt: scheduledAt})
	if !handled {
		t.Fatal("StatusClearMsg should be handled")
	}
	if app.feedback.message != "newer guidance" {
		t.Fatalf("stale clear replaced newer status with the contextual footer: %q", app.feedback.message)
	}
}

func TestStatusClearMsgPreservesStatusWithSameTimestamp(t *testing.T) {
	app := newTestAppModel()
	app.setInfoStatus("coalesced-clock guidance")
	scheduledAt := app.feedback.updatedAt

	handled, _ := app.handleViewMessages(status.StatusClearMsg{ScheduledAt: scheduledAt})
	if !handled {
		t.Fatal("StatusClearMsg should be handled")
	}
	if app.feedback.message != "coalesced-clock guidance" {
		t.Fatalf("same-timestamp stale clear replaced newer status: %q", app.feedback.message)
	}
}

// TestStatusClearMsgClearsExpiredUndoToast verifies that StatusClearMsg still
// clears the screen when the undo toast has already expired — otherwise an
// expired toast string could linger indefinitely.
func TestStatusClearMsgClearsExpiredUndoToast(t *testing.T) {
	app := newTestAppModel()
	_ = app.setUndoableStatus("old", func() tea.Cmd { return nil })
	app.feedback.expiresAt = time.Now().Add(-time.Second)

	handled, _ := app.handleViewMessages(status.StatusClearMsg{})
	if !handled {
		t.Error("StatusClearMsg should be handled")
	}
	if app.feedback.message != "" {
		t.Errorf("expired toast should be cleared by StatusClearMsg, got %q", app.feedback.message)
	}
}

func TestFormatRecoveryFeedbackIncludesContext(t *testing.T) {
	got := formatRecoveryFeedback("Delete Task", "#42 Login flow", true)
	want := "Delete Task: #42 Login flow · u undo · Ctrl+O origin"
	if got != want {
		t.Errorf("formatRecoveryFeedback = %q, want %q", got, want)
	}
	if formatRecoveryFeedback("Bulk Edit", "", false) != "Bulk Edit · u undo" {
		t.Error("formatRecoveryFeedback should omit empty context and unrecorded origin")
	}
}

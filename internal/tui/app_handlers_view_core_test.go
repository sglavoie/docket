package tui

import (
	"errors"
	"testing"
)

func TestHandleViewMessagesBack(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectTree)
	app.pushView(ViewTaskEditor)

	msg := BackMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BackMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should be ProjectTree after back, got %v", app.currentView())
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

func TestHandleViewMessagesMoveBlocked(t *testing.T) {
	app := newTestAppModel()

	msg := MoveBlockedMsg{Reason: "cannot move completed task"}
	handled, cmd := app.handleViewMessages(msg)

	if !handled {
		t.Error("MoveBlockedMsg should be handled")
	}
	if app.statusMessage != "cannot move completed task" {
		t.Errorf("statusMessage should be set to reason, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("MoveBlockedMsg should return a status timeout command")
	}
}

func TestHandleViewMessagesRefresh(t *testing.T) {
	app := newTestAppModel()
	app.dashboard = NewDashboardModel(app.config, nil)

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

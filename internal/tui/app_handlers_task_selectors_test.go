package tui

import (
	"testing"
)

func TestHandleTaskMessagesCloseParentSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewParentSelector)

	app.Update(CloseParentSelectorMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseProjectSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewProjectSelector)

	app.Update(CloseProjectSelectorMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseLinkedTaskSelector(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewLinkedTaskSelector)

	app.Update(CloseLinkedTaskSelectorMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseCrossProjectLinker(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewCrossProjectLinker)

	app.Update(CloseCrossProjectLinkerMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing linker, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommandPaletteTaskActionNilTask(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteTaskActionMsg{Task: nil, Project: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteTaskActionMsg with nil task should be handled")
	}
}

func TestHandleTaskMessagesCommandPaletteBulkActionEmpty(t *testing.T) {
	app := newTestAppModel()

	msg := CommandPaletteBulkActionMsg{Tasks: nil, Action: "complete"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommandPaletteBulkActionMsg with empty tasks should be handled")
	}
}

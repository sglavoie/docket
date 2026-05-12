package tui

import "testing"

func TestHandleViewMessagesShowQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowQuickCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowQuickCaptureMsg should be handled")
	}
	if app.quickCaptureOverlay == nil {
		t.Error("quickCaptureOverlay should be set after ShowQuickCaptureMsg")
	}
	// Quick capture is now an overlay — it must not push onto the view stack.
	if app.currentView() == ViewQuickCapture {
		t.Error("ViewQuickCapture must not be pushed onto the view stack (it is now an overlay)")
	}
}

func TestHandleViewMessagesCloseQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()
	// Set up the overlay (new behavior: overlay, not view stack)
	app.quickCaptureOverlay = NewQuickCaptureModel(project, nil, false)

	app.Update(CloseQuickCaptureMsg{})

	if app.quickCaptureOverlay != nil {
		t.Error("quickCaptureOverlay should be nil after CloseQuickCaptureMsg")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should remain Dashboard after closing quick capture overlay, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowBulkCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowBulkCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowBulkCaptureMsg should be handled")
	}
	if app.currentView() != ViewBulkCapture {
		t.Errorf("currentView should be BulkCapture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesCloseBulkCapture(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkCapture)

	app.Update(CloseBulkCaptureMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing bulk capture, got %v", app.currentView())
	}
}

func TestHandleViewMessagesBulkEditDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewBulkEdit)

	msg := BulkEditDoneMsg{Applied: false}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BulkEditDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after bulk edit done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesRecurrenceEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewRecurrenceEditor)

	msg := RecurrenceEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RecurrenceEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after recurrence editor done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesTextEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTextEditor)

	msg := TextEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TextEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after text editor done, got %v", app.currentView())
	}
}

func TestHandleViewMessagesShowBulkEdit(t *testing.T) {
	app, project := newTestAppModelWithProject()
	tasks := project.Tasks

	msg := ShowBulkEditMsg{Tasks: tasks, Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowBulkEditMsg should be handled")
	}
	if app.currentView() != ViewBulkEdit {
		t.Errorf("currentView should be BulkEdit, got %v", app.currentView())
	}
}

func TestHandleViewMessagesBlockerAdded(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.dashboard = NewDashboardModel(app.config, nil)

	msg := BlockerAddedMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BlockerAddedMsg should be handled")
	}
}

func TestHandleViewMessagesAttachmentAdded(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.dashboard = NewDashboardModel(app.config, nil)

	msg := AttachmentAddedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("AttachmentAddedMsg should be handled")
	}
}

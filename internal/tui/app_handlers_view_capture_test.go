package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestHandleViewMessagesShowQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	msg := ShowQuickCaptureMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowQuickCaptureMsg should be handled")
	}
	// Quick capture is now a normal stack view, mirroring Bulk Capture.
	if app.currentView() != ViewQuickCapture {
		t.Errorf("currentView should be QuickCapture after ShowQuickCaptureMsg, got %v", app.currentView())
	}
	capture, ok := app.views.Get(ViewQuickCapture).(*QuickCaptureModel)
	if !ok {
		t.Fatalf("quick capture view = %T, want *QuickCaptureModel", app.views.Get(ViewQuickCapture))
	}
	if capture.project != project {
		t.Fatalf("quick capture project = %v, want contextual project %v (system Inbox was %v)", capture.project, project, inbox)
	}
	if capture.skipProjectSyntax {
		t.Fatal("contextual quick capture unexpectedly disabled project-sigil routing")
	}
	if len(capture.projectCache) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.projectCache))
	}
}

func TestHandleViewMessagesShowQuickCaptureDefaultsToSystemInbox(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	handled, _ := app.handleViewMessages(ShowQuickCaptureMsg{})
	if !handled {
		t.Fatal("ShowQuickCaptureMsg should be handled")
	}
	capture, ok := app.views.Get(ViewQuickCapture).(*QuickCaptureModel)
	if !ok {
		t.Fatalf("quick capture view = %T, want *QuickCaptureModel", app.views.Get(ViewQuickCapture))
	}
	if capture.project != inbox {
		t.Fatalf("quick capture project = %v, want system Inbox %v", capture.project, inbox)
	}
	if capture.skipProjectSyntax {
		t.Fatal("context-free quick capture unexpectedly disabled project-sigil routing")
	}
	if len(capture.projectCache) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.projectCache))
	}
}

func installTestSystemInbox(app *AppModel, projects ...*models.Project) *models.Project {
	inbox := testutil.NewTestProjectWithTasks("Inbox")
	setTestAppProjects(app, append(projects, inbox)...)
	catalog := app.runtime.Catalog()
	catalog.SystemInboxProjectID = inbox.ID
	app.runtime.SetCatalogInMemory(catalog)
	return inbox
}

func TestHandleViewMessagesCloseQuickCapture(t *testing.T) {
	app, project := newTestAppModelWithProject()
	// Push quick capture as a normal stack view (new behavior).
	app.showView(ViewQuickCapture, NewQuickCaptureModel(project, nil, false))
	if app.currentView() != ViewQuickCapture {
		t.Fatalf("setup: currentView should be QuickCapture, got %v", app.currentView())
	}

	app.Update(CloseQuickCaptureMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should pop back to Dashboard after CloseQuickCaptureMsg, got %v", app.currentView())
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

	msg := BlockerAddedMsg{Project: project}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("BlockerAddedMsg should be handled")
	}
}

func TestHandleViewMessagesAttachmentAdded(t *testing.T) {
	app, _ := newTestAppModelWithProject()

	msg := AttachmentAddedMsg{}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("AttachmentAddedMsg should be handled")
	}
}

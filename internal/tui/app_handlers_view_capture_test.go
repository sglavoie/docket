package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
)

func TestHandleViewMessagesShowTaskComposer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	msg := app.captureComposerMessage(project, app.activeProjects())
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTaskComposerMsg should be handled")
	}
	if app.currentView() != ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer after ShowTaskComposerMsg, got %v", app.currentView())
	}
	capture, ok := app.views.Get(ViewTaskComposer).(*TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.views.Get(ViewTaskComposer))
	}
	if capture.preset.Project != project {
		t.Fatalf("quick capture project = %v, want contextual project %v (system Inbox was %v)", capture.preset.Project, project, inbox)
	}
	if !capture.preset.AllowProjectSwitch {
		t.Fatal("contextual quick capture unexpectedly disabled project switching")
	}
	if len(capture.preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.preset.AllProjects))
	}
}

func TestTaskComposerCaptureDefaultsToSystemInbox(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	handled, _ := app.handleViewMessages(app.captureComposerMessage(nil, nil))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	capture, ok := app.views.Get(ViewTaskComposer).(*TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.views.Get(ViewTaskComposer))
	}
	if capture.preset.Project != inbox {
		t.Fatalf("quick capture project = %v, want system Inbox %v", capture.preset.Project, inbox)
	}
	if !capture.preset.AllowProjectSwitch {
		t.Fatal("context-free quick capture unexpectedly disabled project switching")
	}
	if len(capture.preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.preset.AllProjects))
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

func TestHandleViewMessagesRecurrenceEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewRecurrenceEditor)

	msg := RecurrenceEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("RecurrenceEditorDoneMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after recurrence editor done, got %v", app.currentView())
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
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after text editor done, got %v", app.currentView())
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

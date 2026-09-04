package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/testutil"
	"github.com/sglavoie/tt/internal/tui/chrome"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestHandleViewMessagesShowTaskComposer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	msg := app.captureComposerMessage(project, app.taskWorkspaceProjects())
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("ShowTaskComposerMsg should be handled")
	}
	if app.currentView() != presentation.ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer after ShowTaskComposerMsg, got %v", app.currentView())
	}
	capture, ok := app.views.Get(presentation.ViewTaskComposer).(*taskform.TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.views.Get(presentation.ViewTaskComposer))
	}
	if capture.Preset.Project != project {
		t.Fatalf("quick capture project = %v, want contextual project %v (system Inbox was %v)", capture.Preset.Project, project, inbox)
	}
	if !capture.Preset.AllowProjectSwitch {
		t.Fatal("contextual quick capture unexpectedly disabled project switching")
	}
	if len(capture.Preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.Preset.AllProjects))
	}
}

func TestTaskComposerCaptureDefaultsToSystemUnfiled(t *testing.T) {
	app, project := newTestAppModelWithProject()
	inbox := installTestSystemInbox(app, project)

	handled, _ := app.handleViewMessages(app.captureComposerMessage(nil, nil))
	if !handled {
		t.Fatal("ShowTaskComposerMsg should be handled")
	}
	capture, ok := app.views.Get(presentation.ViewTaskComposer).(*taskform.TaskComposerModel)
	if !ok {
		t.Fatalf("composer view = %T, want *TaskComposerModel", app.views.Get(presentation.ViewTaskComposer))
	}
	if capture.Preset.Project != inbox {
		t.Fatalf("quick capture project = %v, want system Inbox %v", capture.Preset.Project, inbox)
	}
	if !capture.Preset.AllowProjectSwitch {
		t.Fatal("context-free quick capture unexpectedly disabled project switching")
	}
	if len(capture.Preset.AllProjects) != 2 {
		t.Fatalf("quick capture project cache has %d projects, want 2", len(capture.Preset.AllProjects))
	}
}

func installTestSystemInbox(app *AppModel, projects ...*models.Project) *models.Project {
	inbox := testutil.NewTestProjectWithTasks("Unfiled")
	setTestAppProjects(app, append(projects, inbox)...)
	catalog := app.runtime.Catalog()
	catalog.SystemUnfiledProjectID = inbox.ID
	catalog.SystemUnfiledProjectID = inbox.ID
	app.runtime.SetCatalogInMemory(catalog)
	return inbox
}

func TestHandleViewMessagesTextEditorDone(t *testing.T) {
	app := newTestAppModel()
	app.pushView(presentation.ViewTextEditor)

	msg := chrome.TextEditorDoneMsg{Canceled: true}
	handled, _ := app.handleViewMessages(msg)

	if !handled {
		t.Error("TextEditorDoneMsg should be handled")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after text editor done, got %v", app.currentView())
	}
}

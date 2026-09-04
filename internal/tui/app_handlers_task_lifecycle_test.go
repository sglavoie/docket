package tui

import (
	"testing"

	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/presentation"
	"github.com/sglavoie/tt/internal/tui/taskform"
)

func TestHandleTaskMessagesCreateTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := taskform.CreateTaskMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg should be handled")
	}
	if app.currentView() != presentation.ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer, got %v", app.currentView())
	}

	if app.currentProject != project {
		t.Error("currentProject should still be set")
	}
}

func TestHandleTaskMessagesCreateTaskWithParent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	parentTask := project.Tasks[0]

	msg := taskform.CreateTaskMsg{ParentTask: parentTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg with parent should be handled")
	}
}

func TestHandleTaskMessagesEditTask(t *testing.T) {
	_, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	out := NewOutlinerModel(project, nil, nil)

	updated, _ := out.Update(StartTaskLineEditMsg{Project: project, Task: task})
	if updated != out || !out.quickEdit.IsActive() || out.quickEdit.GetTask() != task {
		t.Fatalf("inline title edit was not activated for %s/%s", project.ID, task.ID)
	}
}

func TestHandleTaskMessagesTaskCreated(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pushView(presentation.ViewTaskComposer)

	msg := coordinator.TaskCreatedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("coordinator.TaskCreatedMsg should be handled")
	}
	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after task created, got %v", app.currentView())
	}
}

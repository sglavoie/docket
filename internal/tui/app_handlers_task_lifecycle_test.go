package tui

import (
	"testing"

	taskapp "github.com/sglavoie/tt/internal/app"
)

func TestHandleTaskMessagesCreateTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := CreateTaskMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer, got %v", app.currentView())
	}

	if app.currentProject != project {
		t.Error("currentProject should still be set")
	}
}

func TestHandleTaskMessagesCreateTaskWithParent(t *testing.T) {
	app, project := newTestAppModelWithProject()
	parentTask := project.Tasks[0]

	msg := CreateTaskMsg{ParentTask: parentTask}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateTaskMsg with parent should be handled")
	}
}

func TestHandleTaskMessagesEditTask(t *testing.T) {
	model, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskTextEditorMsg{Project: project, Task: task, Operation: taskapp.TaskOperationEditTitle}
	handled, _ := model.handleEditorMessages(msg)

	if !handled {
		t.Error("ShowTaskTextEditorMsg should be handled")
	}
	if model.currentView() != ViewTextEditor {
		t.Errorf("currentView should be TextEditor, got %v", model.currentView())
	}
	if model.activeTaskComposer() || model.taskComposer != nil {
		t.Error("daily title edit opened the full Task Composer")
	}
	if model.pendingTaskTextEdit == nil {
		t.Fatal("daily title edit did not retain its stable task identity")
	}
	if model.pendingTaskTextEdit.Operation != taskapp.TaskOperationEditTitle {
		t.Errorf("operation = %q, want title edit", model.pendingTaskTextEdit.Operation)
	}
	if model.pendingTaskTextEdit.Identity.ProjectID != project.ID ||
		model.pendingTaskTextEdit.Identity.TaskID != task.ID {
		t.Errorf("pending identity = %+v, want %s/%s", model.pendingTaskTextEdit.Identity, project.ID, task.ID)
	}
}

func TestHandleTaskMessagesTaskCreated(t *testing.T) {
	app, _ := newTestAppModelWithProject()
	app.pushView(ViewTaskForm)

	msg := TaskCreatedMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskCreatedMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after task created, got %v", app.currentView())
	}
}

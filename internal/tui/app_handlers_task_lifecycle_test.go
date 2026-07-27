package tui

import "testing"

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
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := EditTaskMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("EditTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskComposer {
		t.Errorf("currentView should be TaskComposer, got %v", app.currentView())
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

func TestHandleTaskMessagesTaskDeleted(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, nil, app.runtime)

	msg := TaskDeletedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskDeletedMsg should be handled")
	}
}

func TestHandleTaskMessagesTasksArchived(t *testing.T) {
	app := newTestAppModel()

	msg := TasksArchivedMsg{Count: 5}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TasksArchivedMsg should be handled")
	}
	if app.statusMessage != "Archived 5 task(s) to completed.md" {
		t.Errorf("statusMessage should contain archive count, got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("TasksArchivedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesTaskStatusChanged(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, nil, app.runtime)

	msg := TaskStatusChangedMsg{Task: project.Tasks[0]}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("TaskStatusChangedMsg should be handled")
	}
}

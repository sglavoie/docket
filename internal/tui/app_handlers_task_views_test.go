package tui

import "testing"

func TestHandleTaskMessagesDependencySelectorDone(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	app.pushView(ViewDependencySelector)

	msg := DependencySelectorDoneMsg{HasChanges: false, Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DependencySelectorDoneMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing selector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowDependencyViewer(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowDependencyViewerMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDependencyViewer {
		t.Errorf("currentView should be DependencyViewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseDependencyViewer(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewDependencyViewer)

	app.Update(CloseDependencyViewerMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing viewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesUndoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	msg := UndoPerformedMsg{Action: "Task deleted"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("UndoPerformedMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should remain ProjectTree after undo, got %v", app.currentView())
	}
	if app.statusMessage != "Undid: Task deleted" {
		t.Errorf("statusMessage should contain undo action, got %q", app.statusMessage)
	}
}

func TestHandleTaskMessagesRedoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewProjectTree)

	msg := RedoPerformedMsg{Action: "Task restored"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("RedoPerformedMsg should be handled")
	}
	if app.currentView() != ViewProjectTree {
		t.Errorf("currentView should remain ProjectTree after redo, got %v", app.currentView())
	}
	if app.statusMessage != "Redo: Task restored" {
		t.Errorf("statusMessage should contain redo action, got %q", app.statusMessage)
	}
}

func TestHandleTaskMessagesShowTaskDetail(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskDetailMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskDetailMsg should be handled")
	}
	if app.currentView() != ViewTaskDetail {
		t.Errorf("currentView should be TaskDetail, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskDetail(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskDetail)

	app.Update(CloseTaskDetailMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing detail, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowTaskChecklist(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskChecklistMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskChecklistMsg should be handled")
	}
	if app.currentView() != ViewTaskChecklist {
		t.Errorf("currentView should be TaskChecklist, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskChecklist(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskChecklist)

	app.Update(CloseTaskChecklistMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing checklist, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesOpenTaskActivity(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := OpenTaskActivityMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenTaskActivityMsg should be handled")
	}
	if app.currentView() != ViewTaskActivity {
		t.Errorf("currentView should be TaskActivity, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskActivity(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskActivity)

	app.Update(CloseTaskActivityMsg{})

	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing activity, got %v", app.currentView())
	}
}

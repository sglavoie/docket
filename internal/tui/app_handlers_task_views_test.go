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

	msg := CloseDependencyViewerMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseDependencyViewerMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing viewer, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowUndoHistory(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := ShowUndoHistoryMsg{Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewUndoHistory {
		t.Errorf("currentView should be UndoHistory, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseUndoHistory(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewUndoHistory)

	msg := CloseUndoHistoryMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseUndoHistoryMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing undo history, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesUndoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.projectTree = NewProjectTreeModel(project, app.config, nil)
	app.pushView(ViewUndoHistory)

	msg := UndoPerformedMsg{Action: "Task deleted"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("UndoPerformedMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after undo, got %v", app.currentView())
	}
	if app.statusMessage != "Undid: Task deleted" {
		t.Errorf("statusMessage should contain undo action, got %q", app.statusMessage)
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

	msg := CloseTaskDetailMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskDetailMsg should be handled")
	}
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

	msg := CloseTaskChecklistMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskChecklistMsg should be handled")
	}
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

	msg := CloseTaskActivityMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CloseTaskActivityMsg should be handled")
	}
	if app.currentView() != ViewDashboard {
		t.Errorf("currentView should be Dashboard after closing activity, got %v", app.currentView())
	}
}

package tui

import "testing"

func TestHandleTaskMessagesUndoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, nil, app.runtime)
	app.pushView(ViewCrossProjectOutliner)

	msg := UndoPerformedMsg{Action: "Task deleted"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("UndoPerformedMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should remain Outliner after undo, got %v", app.currentView())
	}
	if app.statusMessage != "Undid: Task deleted" {
		t.Errorf("statusMessage should contain undo action, got %q", app.statusMessage)
	}
}

func TestHandleTaskMessagesRedoPerformed(t *testing.T) {
	app, project := newTestAppModelWithProject()
	app.crossOutliner = NewOutlinerModel(project, nil, app.runtime)
	app.pushView(ViewCrossProjectOutliner)

	msg := RedoPerformedMsg{Action: "Task restored"}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("RedoPerformedMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should remain Outliner after redo, got %v", app.currentView())
	}
	if app.statusMessage != "Redo: Task restored" {
		t.Errorf("statusMessage should contain redo action, got %q", app.statusMessage)
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

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing checklist, got %v", app.currentView())
	}
}

package tui

import (
	"testing"
)

func TestHandleTaskMessagesManageDependencies(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ManageDependenciesMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ManageDependenciesMsg should be handled")
	}
	if app.currentView() != ViewDependencySelector {
		t.Errorf("currentView should be DependencySelector, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCreateLinkedTask(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := CreateLinkedTaskMsg{SourceTask: task, Project: project, LinkType: LinkTypeBlocker}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CreateLinkedTaskMsg should be handled")
	}
	if app.currentView() != ViewTaskComposer {
		t.Errorf("currentView should be LinkedTaskCreator, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowCrossProjectLinker(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowCrossProjectLinkerMsg{SourceTask: task, SourceProject: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowCrossProjectLinkerMsg should be handled")
	}
	if app.currentView() != ViewCrossProjectLinker {
		t.Errorf("currentView should be CrossProjectLinker, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesShowProjectSelectorForMove(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowProjectSelectorForMoveMsg{Task: task, SourceProject: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowProjectSelectorForMoveMsg should be handled")
	}
	if app.currentView() != ViewProjectSelector {
		t.Errorf("currentView should be ProjectSelector, got %v", app.currentView())
	}
}

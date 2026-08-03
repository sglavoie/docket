package tui

import (
	"testing"
)

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

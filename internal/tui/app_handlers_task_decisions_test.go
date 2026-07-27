package tui

import "testing"

func TestHandleTaskMessagesShowTaskDecisions(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskDecisionsMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskDecisionsMsg should be handled")
	}
	if app.currentView() != ViewTaskDecisions {
		t.Errorf("currentView should be TaskDecisions, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskDecisions(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskDecisions)

	app.Update(CloseTaskDecisionsMsg{})

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing decisions, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesDecisionAdded(t *testing.T) {
	app := newTestAppModel()

	msg := DecisionAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DecisionAddedMsg should be handled")
	}
	if app.statusMessage != "Decision added" {
		t.Errorf("statusMessage should be 'Decision added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("DecisionAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesDecisionDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := DecisionDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DecisionDeletedMsg should be handled")
	}
	if app.statusMessage != "Decision deleted" {
		t.Errorf("statusMessage should be 'Decision deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("DecisionDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesDecisionEdited(t *testing.T) {
	app := newTestAppModel()

	msg := DecisionEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DecisionEditedMsg should be handled")
	}
	if app.statusMessage != "Decision updated" {
		t.Errorf("statusMessage should be 'Decision updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("DecisionEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesDecisionResolved(t *testing.T) {
	app := newTestAppModel()

	msg := DecisionResolvedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("DecisionResolvedMsg should be handled")
	}
	if app.statusMessage != "Decision resolved" {
		t.Errorf("statusMessage should be 'Decision resolved', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("DecisionResolvedMsg should return a status timeout command")
	}
}

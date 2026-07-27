package tui

import "testing"

func TestHandleTaskMessagesShowTaskComments(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := ShowTaskCommentsMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskCommentsMsg should be handled")
	}
	if app.currentView() != ViewTaskComments {
		t.Errorf("currentView should be TaskComments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskComments(t *testing.T) {
	app := newTestAppModel()
	app.pushView(ViewTaskComments)

	app.Update(CloseTaskCommentsMsg{})

	if app.currentView() != ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing comments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommentAdded(t *testing.T) {
	app := newTestAppModel()

	msg := CommentAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.statusMessage != "Comment added" {
		t.Errorf("statusMessage should be 'Comment added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := CommentDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentDeletedMsg should be handled")
	}
	if app.statusMessage != "Comment deleted" {
		t.Errorf("statusMessage should be 'Comment deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentEdited(t *testing.T) {
	app := newTestAppModel()

	msg := CommentEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentEditedMsg should be handled")
	}
	if app.statusMessage != "Comment updated" {
		t.Errorf("statusMessage should be 'Comment updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentResolved(t *testing.T) {
	app := newTestAppModel()

	msg := CommentResolvedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentResolvedMsg should be handled")
	}
	if app.statusMessage != "Comment resolved" {
		t.Errorf("statusMessage should be 'Comment resolved', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("CommentResolvedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemAdded(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemAddedMsg should be handled")
	}
	if app.statusMessage != "Checklist item added" {
		t.Errorf("statusMessage should be 'Checklist item added', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemDeletedMsg should be handled")
	}
	if app.statusMessage != "Checklist item deleted" {
		t.Errorf("statusMessage should be 'Checklist item deleted', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemEdited(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemEditedMsg should be handled")
	}
	if app.statusMessage != "Checklist item updated" {
		t.Errorf("statusMessage should be 'Checklist item updated', got %q", app.statusMessage)
	}
	if cmd == nil {
		t.Error("ChecklistItemEditedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesChecklistItemToggled(t *testing.T) {
	app := newTestAppModel()

	msg := ChecklistItemToggledMsg{}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ChecklistItemToggledMsg should be handled")
	}
}

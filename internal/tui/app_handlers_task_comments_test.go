package tui

import (
	"testing"

	"github.com/sglavoie/docket/internal/tui/presentation"
	"github.com/sglavoie/docket/internal/tui/taskdetail"
)

func TestHandleTaskMessagesShowTaskComments(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := taskdetail.ShowTaskCommentsMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("ShowTaskCommentsMsg should be handled")
	}
	if app.currentView() != presentation.ViewTaskComments {
		t.Errorf("currentView should be TaskComments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCloseTaskComments(t *testing.T) {
	app := newTestAppModel()
	app.pushView(presentation.ViewTaskComments)

	app.Update(taskdetail.CloseTaskCommentsMsg{})

	if app.currentView() != presentation.ViewCrossProjectOutliner {
		t.Errorf("currentView should be Outliner after closing comments, got %v", app.currentView())
	}
}

func TestHandleTaskMessagesCommentAdded(t *testing.T) {
	app := newTestAppModel()

	msg := taskdetail.CommentAddedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.feedback.message != "Comment added" {
		t.Errorf("statusMessage should be 'Comment added', got %q", app.feedback.message)
	}
	if cmd == nil {
		t.Error("CommentAddedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentDeleted(t *testing.T) {
	app := newTestAppModel()

	msg := taskdetail.CommentDeletedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentDeletedMsg should be handled")
	}
	if app.feedback.message != "Comment deleted" {
		t.Errorf("statusMessage should be 'Comment deleted', got %q", app.feedback.message)
	}
	if cmd == nil {
		t.Error("CommentDeletedMsg should return a status timeout command")
	}
}

func TestHandleTaskMessagesCommentEdited(t *testing.T) {
	app := newTestAppModel()

	msg := taskdetail.CommentEditedMsg{}
	handled, cmd := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentEditedMsg should be handled")
	}
	if app.feedback.message != "Comment updated" {
		t.Errorf("statusMessage should be 'Comment updated', got %q", app.feedback.message)
	}
	if cmd == nil {
		t.Error("CommentEditedMsg should return a status timeout command")
	}
}

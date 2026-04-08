package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestOpenQuickNoteMsgCreatesOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	msg := OpenQuickNoteMsg{Task: task, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenQuickNoteMsg should be handled")
	}
	if app.quickNoteOverlay == nil {
		t.Error("quickNoteOverlay should be set after OpenQuickNoteMsg")
	}
}

func TestOpenQuickNoteMsgNilTask(t *testing.T) {
	app, project := newTestAppModelWithProject()

	msg := OpenQuickNoteMsg{Task: nil, Project: project}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("OpenQuickNoteMsg with nil task should still be handled")
	}
	if app.quickNoteOverlay != nil {
		t.Error("quickNoteOverlay should not be set when task is nil")
	}
}

func TestCommentAddedMsgClearsOverlay(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	app.quickNoteOverlay = newQuickNoteOverlay(task, project)

	msg := CommentAddedMsg{Task: task}
	handled, _ := app.handleTaskMessages(msg)

	if !handled {
		t.Error("CommentAddedMsg should be handled")
	}
	if app.quickNoteOverlay != nil {
		t.Error("quickNoteOverlay should be cleared after CommentAddedMsg")
	}
}

func TestQuickNoteOverlayEscDismisses(t *testing.T) {
	app, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	app.quickNoteOverlay = newQuickNoteOverlay(task, project)

	if app.quickNoteOverlay == nil {
		t.Fatal("overlay should be set before test")
	}

	_, dismiss, _ := app.quickNoteOverlay.handleKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	if !dismiss {
		t.Error("Esc should set dismiss=true")
	}
}

func TestAppendCommentAddsComment(t *testing.T) {
	_, project := newTestAppModelWithProject()
	task := project.Tasks[0]
	initialCount := len(task.Comments)

	result, err := appendComment(project, task.ID, "  hello  ")
	if err != nil {
		t.Fatalf("appendComment should not error: %v", err)
	}
	if result == nil {
		t.Fatal("appendComment should return the task")
	}
	if len(result.Comments) != initialCount+1 {
		t.Errorf("expected %d comments, got %d", initialCount+1, len(result.Comments))
	}
	if result.Comments[len(result.Comments)-1].Text != "hello" {
		t.Errorf("comment text should be trimmed, got %q", result.Comments[len(result.Comments)-1].Text)
	}
}

func TestAppendCommentEmptyTextReturnsError(t *testing.T) {
	_, project := newTestAppModelWithProject()
	task := project.Tasks[0]

	_, err := appendComment(project, task.ID, "   ")
	if err == nil {
		t.Error("appendComment with empty text should return error")
	}
}

func TestAppendCommentUnknownTaskReturnsError(t *testing.T) {
	_, project := newTestAppModelWithProject()

	_, err := appendComment(project, "nonexistent-id", "some text")
	if err == nil {
		t.Error("appendComment with unknown task ID should return error")
	}
}

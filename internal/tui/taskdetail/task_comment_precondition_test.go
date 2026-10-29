package taskdetail

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/storage"
)

func TestTaskCommentEditRefusesAShiftedRowAndKeepsDraft(t *testing.T) {
	for _, when := range []string{"before submission", "after submission"} {
		t.Run(when, func(t *testing.T) {
			project, task := commentsFixture(t, "first", "second", "third")
			model := newTestCommentsPanel(task, project)
			model.Cursor = 1
			model.startEditComment()
			model.EditTextInput.SetValue("second, edited")
			var cmd tea.Cmd
			// The command has frozen its target, but the transaction has not run.
			if when == "after submission" {
				cmd = model.saveEditedComment()
			}
			project.Tasks[0].RemoveCommentAt(0, time.Now())
			if err := storage.SaveProject(project); err != nil {
				t.Fatal(err)
			}
			if when == "before submission" {
				cmd = model.saveEditedComment()
			}
			if _, ok := settle(t, model, cmd).(CommentMutationFailedMsg); !ok {
				t.Fatal("stale edit must be refused by the application service")
			}
			if !model.EditMode || model.EditTextInput.Value() != "second, edited" || model.Err == nil {
				t.Fatal("stale edit did not keep the draft and its refusal")
			}
			if err := storage.LoadProject(project); err != nil {
				t.Fatal(err)
			}
			comments := project.Tasks[0].Comments
			if len(comments) != 2 || comments[0].Text != "second" || comments[1].Text != "third" {
				t.Fatal("stale edit overwrote a surviving comment")
			}
		})
	}
}

func TestTaskCommentDeleteRefusesChangesAfterConfirmation(t *testing.T) {
	project, task := commentsFixture(t, "first", "second", "third")
	model := newTestCommentsPanel(task, project)
	model.Cursor = 1
	model.ConfirmDeleteSelectedComment()
	cmd := model.executeDelete()
	project.Tasks[0].RemoveCommentAt(0, time.Now())
	if err := storage.SaveProject(project); err != nil {
		t.Fatal(err)
	}
	if _, ok := settle(t, model, cmd).(CommentMutationFailedMsg); !ok {
		t.Fatal("stale deletion must be refused after confirmation")
	}
	if err := storage.LoadProject(project); err != nil {
		t.Fatal(err)
	}
	comments := project.Tasks[0].Comments
	if len(comments) != 2 || comments[0].Text != "second" || comments[1].Text != "third" {
		t.Fatal("confirmed deletion removed a different comment")
	}
}

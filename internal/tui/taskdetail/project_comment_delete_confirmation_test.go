package taskdetail

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/storage"
	"github.com/sglavoie/tt/internal/tui/status"
)

// answerDeleteConfirmation presses one key against an open confirmation the way
// the shell delivers it, so the acceptance runs through Update's own routing
// rather than through the action the routing happens to reach.
func answerDeleteConfirmation(t *testing.T, model *CommentsModel, key rune) tea.Cmd {
	t.Helper()
	if !model.IsConfirmActive() {
		t.Fatal("no delete confirmation is open")
	}
	_, cmd := model.Update(tea.KeyPressMsg{Code: key, Text: string(key)})
	return cmd
}

// refusalText runs a refusal command once and names the error it reported,
// failing when the command committed something instead. Running it a second time
// would submit whatever it carries twice, so the outcome is read from a single
// invocation.
func refusalText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()
	if cmd == nil {
		t.Fatal("the confirmed delete produced no command at all")
	}
	outcome := cmd()
	msg, ok := outcome.(status.ErrorMsg)
	if !ok {
		t.Fatalf("the confirmed delete resolved to %T, want a refusal", outcome)
	}
	if msg.Err == nil {
		t.Fatal("the refusal named no error")
	}
	return msg.Err.Error()
}

// saveExternalChange writes a change made by another surface so the application
// layer's fresh snapshot can see it. Mutating the in-memory project alone is
// invisible to a mutation, which re-resolves its target from storage.
func saveExternalChange(t *testing.T, project *models.Project) {
	t.Helper()
	if err := storage.SaveProject(project); err != nil {
		t.Fatalf("SaveProject: %v", err)
	}
}

// TestProjectCommentDeleteConfirmationRetainsTarget holds the whole of the
// frozen-target contract for deletion.
//
// A confirmation is a question about one comment, and the answer arrives however
// long afterwards the user takes to read it. In that gap a commit, an undo, or
// another machine can remove the comment, rewrite it, or push it to a different
// row — and this panel presents a project's comments newest first, so an
// unrelated addition alone reorders every row below it. The removal therefore
// addresses the comment the question named, not the cursor at the moment it is
// answered: a target that is gone or no longer says what the user read is
// refused, and nothing else is deleted in its place.
func TestProjectCommentDeleteConfirmationRetainsTarget(t *testing.T) {
	t.Run("deleted_target", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.Cursor = 0 // the newest comment, presented first

		model.ConfirmDeleteSelectedComment()
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)
		if !model.RefreshAfterReload() {
			t.Fatal("the refresh lost the owner")
		}
		// The cursor now sits on the surviving comment, which is precisely the row a
		// cursor-resolved removal would delete.
		if got := model.Entries()[model.Cursor].ID; got != olderCommentID {
			t.Fatalf("cursor landed on %q, want the neighbor that slid under it", got)
		}

		refusal := refusalText(t, answerDeleteConfirmation(t, model, 'y'))
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		if len(project.Comments) != 1 || project.Comments[0].ID != olderCommentID {
			t.Fatalf("stored comments = %+v, want the surviving comment intact", project.Comments)
		}
		if project.Comments[0].Text != "Older" {
			t.Fatalf("the surviving comment = %q, want it untouched", project.Comments[0].Text)
		}
	})

	t.Run("changed_target", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.Cursor = 0

		model.ConfirmDeleteSelectedComment()
		project.Comments[1].Text = "Newer, rewritten elsewhere"
		saveExternalChange(t, project)
		if !model.RefreshAfterReload() {
			t.Fatal("the refresh lost the owner")
		}

		refusal := refusalText(t, answerDeleteConfirmation(t, model, 'y'))
		if !strings.Contains(refusal, "changed since it was read") {
			t.Fatalf("refusal = %q, want it to name the changed target", refusal)
		}
		if len(project.Comments) != 2 {
			t.Fatalf("stored comments = %+v, want both of them", project.Comments)
		}
		if project.Comments[0].Text != "Older" || project.Comments[1].Text != "Newer, rewritten elsewhere" {
			t.Fatalf("a refused delete disturbed the stored comments: %+v", project.Comments)
		}
	})

	t.Run("reordered_list", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.Cursor = 0

		model.ConfirmDeleteSelectedComment()
		project.Comments = append(project.Comments, &models.ProjectComment{
			ID:        "cccccccc-dddd-eeee-ffff-000000000000",
			Text:      "Arrived elsewhere",
			CreatedAt: time.Date(2026, 3, 5, 9, 0, 0, 0, time.UTC),
		})
		saveExternalChange(t, project)
		if !model.RefreshAfterReload() {
			t.Fatal("the refresh lost the owner")
		}
		// The addition is newest, so it takes the top row and the confirmed comment
		// is no longer where the question was asked. Park the cursor on the new
		// arrival to say so explicitly: the answer must still name the comment the
		// question did.
		model.Cursor = 0
		if got := model.Entries()[0].Text; got != "Arrived elsewhere" {
			t.Fatalf("row 0 = %q, want the unrelated addition on top", got)
		}

		if _, ok := settle(t, model, answerDeleteConfirmation(t, model, 'y')).(CommentDeletedMsg); !ok {
			t.Fatal("the confirmed delete did not resolve to CommentDeletedMsg")
		}
		if len(project.Comments) != 2 {
			t.Fatalf("stored comments = %+v, want the two survivors", project.Comments)
		}
		if project.Comments[0].Text != "Older" || project.Comments[1].Text != "Arrived elsewhere" {
			t.Fatalf("the reordered delete removed the wrong comment: %+v", project.Comments)
		}
	})

	t.Run("canceled_confirmation", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.Cursor = 0

		model.ConfirmDeleteSelectedComment()
		if !model.RefreshAfterReload() {
			t.Fatal("the refresh lost the owner")
		}
		if cmd := answerDeleteConfirmation(t, model, 'n'); cmd != nil {
			t.Fatalf("a declined confirmation produced the command %#v", cmd())
		}
		if model.IsConfirmActive() {
			t.Fatal("the confirmation stayed open after it was declined")
		}
		if model.deleteRef != nil {
			t.Fatal("a declined confirmation kept its frozen target")
		}
		// The frozen target is spent, so nothing is left for a later delete to act
		// on without a question of its own.
		if cmd := model.executeDelete(); cmd != nil {
			t.Fatalf("an unconfirmed delete produced the command %#v", cmd())
		}
		if len(project.Comments) != 2 {
			t.Fatalf("stored comments = %+v, want both of them", project.Comments)
		}
		if project.Comments[0].Text != "Older" || project.Comments[1].Text != "Newer" {
			t.Fatalf("a canceled confirmation disturbed the stored comments: %+v", project.Comments)
		}
	})
}

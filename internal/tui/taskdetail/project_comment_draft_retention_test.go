package taskdetail

import (
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/sglavoie/tt/internal/models"
)

// retainedDraft is the text every subtest below types. It is short enough to
// survive the composer's own wrapping at the width these tests render at, so a
// missing draft is a missing draft rather than a line break.
const retainedDraft = "the replacement nobody else has"

// panelText renders the panel and strips styling. The buffer alone is not the
// contract here: the defect this file locks is a composer that held the draft and
// stopped drawing it, so every assertion reads what a user would see.
func panelText(t *testing.T, model *CommentsModel) string {
	t.Helper()
	return ansi.Strip(model.View().Content)
}

// assertVisible fails when the rendered panel is missing something the user needs
// on screen — their own text, or the reason the panel refused to write it.
func assertVisible(t *testing.T, model *CommentsModel, want ...string) {
	t.Helper()
	view := panelText(t, model)
	for _, fragment := range want {
		if !strings.Contains(view, fragment) {
			t.Fatalf("the panel does not show %q:\n%s", fragment, view)
		}
	}
}

// assertStoredComments names the stored collection by text in stored order, which
// is what says a refusal wrote nothing: not a smaller count, not a different
// count, but the same comments saying the same things.
func assertStoredComments(t *testing.T, project *models.Project, want ...string) {
	t.Helper()
	got := make([]string, 0, len(project.Comments))
	for _, comment := range project.Comments {
		got = append(got, comment.Text)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("stored comments = %q, want %q", got, want)
	}
}

// removedOwnerPanel mounts the project panel over a resolver that answers
// nothing, so the project is gone the way a delete or an archive-and-reload makes
// it gone, while the coordinators behind it still reach a real service seeded with
// it. A commit is therefore refused by the panel's own precondition rather than by
// an absent seam: if the refusal were removed, the mutation would land.
func removedOwnerPanel(t *testing.T) (*CommentsModel, *models.Project) {
	t.Helper()
	project := projectCommentsFixture(t)
	coordinators := newTestSecondaryCoordinators(project)
	model := newProjectCommentsPanel(project, coordinators.ProjectComment, func(string) *models.Project { return nil })
	model.SetSize(100, 30)
	return model, project
}

// TestProjectCommentDraftSurvivesTargetLoss is the no-lost-drafts contract for
// every way a comment edit can outlive the comment it was opened on.
//
// The five refusals below share one shape: the user has written something, the
// thing it was meant to replace is no longer reachable, and the only copy of the
// writing is in this process. Two of them arrive from an external editor, whose
// temporary file is deleted before the result is ever handed to the panel — after
// that there is no file to reopen and no undo to run, so a refusal that resets a
// buffer destroys work rather than protecting it. Each case therefore has to keep
// the exact text, say plainly why it refused, and commit nothing; and because a
// draft the composer stops drawing is a draft the user cannot recover, every case
// asserts the rendered view and not just the buffer behind it.
func TestProjectCommentDraftSurvivesTargetLoss(t *testing.T) {
	t.Run("save_deleted", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.SetSize(100, 30)
		model.Cursor = 0 // the newest comment, presented first
		model.startEditComment()
		model.EditTextInput.SetValue(retainedDraft)

		// Another surface removes exactly the comment being rewritten.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, model.saveEditedComment())
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		if !model.EditMode || model.EditTextInput.Value() != retainedDraft {
			t.Fatalf("the refused save lost the draft: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		assertVisible(t, model, retainedDraft, orphanedEditDraftNotice, errCommentDeletedExternally.Error())
		assertStoredComments(t, project, "Older")
	})

	t.Run("external_launch_deleted", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.SetSize(100, 30)
		model.Cursor = 0
		model.startEditComment()
		model.EditTextInput.SetValue(retainedDraft)

		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, model.editCommentInExternalEditorFromEditMode())
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		if !model.EditMode || model.EditTextInput.Value() != retainedDraft {
			t.Fatalf("the refused editor launch lost the draft: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		assertVisible(t, model, retainedDraft, orphanedEditDraftNotice, errCommentDeletedExternally.Error())
		assertStoredComments(t, project, "Older")

		// The draft leaves the same way any other unsaved edit does, and only that
		// way: nothing about the target's disappearance shortens the question.
		model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if !model.IsConfirmActive() {
			t.Fatal("Esc over a retained draft did not ask before discarding it")
		}
		model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
		if model.EditMode || model.EditTextInput.Value() != "" {
			t.Fatalf("a confirmed discard kept the composer: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		assertStoredComments(t, project, "Older")
	})

	t.Run("external_result_deleted", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.SetSize(100, 30)
		model.Cursor = 0
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		// The editor was launched from the list, so no composer is open; the comment
		// disappears while it is up, and the buffer comes back to a panel that has
		// nowhere to put it unless the capture happens before the refusal.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, model.applyCommentEditorResult(commentEditorFinishedMsg{
			owner: model.ownerKey(),
			kind:  commentMutationEdit,
			ref:   ref,
			text:  retainedDraft,
		}))
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		if !model.EditMode || model.EditTextInput.Value() != retainedDraft {
			t.Fatalf("the editor's text was dropped: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		assertVisible(t, model, retainedDraft, orphanedEditDraftNotice, errCommentDeletedExternally.Error())
		assertStoredComments(t, project, "Older")
	})

	t.Run("external_add_owner_gone", func(t *testing.T) {
		model, project := removedOwnerPanel(t)

		refusal := refusalText(t, model.applyCommentEditorResult(commentEditorFinishedMsg{
			owner: model.ownerKey(),
			kind:  commentMutationAdd,
			text:  retainedDraft,
		}))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		if !model.AddMode || model.TextInput.Value() != retainedDraft {
			t.Fatalf("the editor's new comment was dropped: adding=%v buffer=%q",
				model.AddMode, model.TextInput.Value())
		}
		assertVisible(t, model, retainedDraft, errCommentOwnerGone.Error())
		assertStoredComments(t, project, "Older", "Newer")
	})

	t.Run("external_edit_owner_gone", func(t *testing.T) {
		model, project := removedOwnerPanel(t)
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		refusal := refusalText(t, model.applyCommentEditorResult(commentEditorFinishedMsg{
			owner: model.ownerKey(),
			kind:  commentMutationEdit,
			ref:   ref,
			text:  retainedDraft,
		}))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		if !model.EditMode || model.EditTextInput.Value() != retainedDraft {
			t.Fatalf("the editor's text was dropped: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		// The list is the last one the panel could read, so the row is still drawn and
		// the composer is the ordinary one. What has to be on screen is the draft and
		// the reason it was not written.
		assertVisible(t, model, retainedDraft, errCommentOwnerGone.Error())
		assertStoredComments(t, project, "Older", "Newer")
	})
}

// TestProjectCommentPanelRefreshKeepsADraftOverARemovedOwner is the same promise
// against the one caller that can end the panel rather than refuse a write.
//
// A refresh is not a refusal: nobody asked for anything, so the shell reads the
// return value and pops the view when the owner is gone — no discard question, no
// error, no way back to the buffer. That is the right answer for a panel the user
// is only reading and the wrong one for a panel they are typing into, so the
// answer is the draft's presence, and the missing owner is reported where the
// draft is rather than only implied by a list that stopped moving.
func TestProjectCommentPanelRefreshKeepsADraftOverARemovedOwner(t *testing.T) {
	t.Run("add_draft", func(t *testing.T) {
		model, project := removedOwnerPanel(t)
		model.BeginAddMode()
		model.TextInput.SetValue(retainedDraft)

		if !model.RefreshAfterReload() {
			t.Fatal("the refresh asked the shell to dismiss a panel holding an add draft")
		}
		if !model.AddMode || model.TextInput.Value() != retainedDraft {
			t.Fatalf("the refresh cost the draft: adding=%v buffer=%q", model.AddMode, model.TextInput.Value())
		}
		assertVisible(t, model, retainedDraft, errCommentOwnerGone.Error())
		assertStoredComments(t, project, "Older", "Newer")

		// And the write is still refused, because staying readable is not the same as
		// becoming writable: the owner is gone either way.
		refusal := refusalText(t, model.saveComment())
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		assertStoredComments(t, project, "Older", "Newer")
		assertVisible(t, model, retainedDraft, errCommentOwnerGone.Error())
	})

	t.Run("edit_draft", func(t *testing.T) {
		model, project := removedOwnerPanel(t)
		model.Cursor = 0
		model.startEditComment()
		model.EditTextInput.SetValue(retainedDraft)

		if !model.RefreshAfterReload() {
			t.Fatal("the refresh asked the shell to dismiss a panel holding an edit draft")
		}
		if !model.EditMode || model.EditTextInput.Value() != retainedDraft {
			t.Fatalf("the refresh cost the draft: editing=%v buffer=%q", model.EditMode, model.EditTextInput.Value())
		}
		assertVisible(t, model, retainedDraft, errCommentOwnerGone.Error())
		assertStoredComments(t, project, "Older", "Newer")

		// The draft leaves the way any other unsaved edit does, and only that way.
		model.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
		if !model.IsConfirmActive() {
			t.Fatal("Esc over the retained draft did not ask before discarding it")
		}
		model.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
		if model.EditMode || model.EditTextInput.Value() != "" {
			t.Fatalf("a confirmed discard kept the composer: editing=%v buffer=%q",
				model.EditMode, model.EditTextInput.Value())
		}
		assertStoredComments(t, project, "Older", "Newer")
	})

	t.Run("no_draft", func(t *testing.T) {
		model, _ := removedOwnerPanel(t)

		// Nothing is being typed, so there is nothing the dismissal can take: the
		// shell's own answer to a vanished owner stands.
		if model.RefreshAfterReload() {
			t.Fatal("a browsing panel over a removed owner refused to be dismissed")
		}
	})
}

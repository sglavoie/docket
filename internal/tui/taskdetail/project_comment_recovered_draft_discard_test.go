package taskdetail

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
)

// discardDraftTail ends every draft below and exists nowhere else in the case
// that carries it. It is the part of the draft the composer cannot hold, so a
// guard that read the box would never see it — which is exactly the difference
// each case turns into a question.
const discardDraftTail = "TAIL-MARKER-8d47"

// discardDraftFixture builds a project inside a real directory whose newest
// comment says exactly what a case needs it to say. The text is the point here:
// these cases turn on the composer seating a recovered draft back to the very
// string the discard shortcut compares against, so the original cannot be an
// unrelated word chosen for readability.
func discardDraftFixture(t *testing.T, target string) (*CommentsModel, *models.Project) {
	t.Helper()
	project, err := models.NewProject("project-1", "Comments Project", t.TempDir())
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	created := time.Date(2026, 3, 4, 9, 0, 0, 0, time.UTC)
	project.Comments = []*models.ProjectComment{
		{ID: olderCommentID, Text: "Older", CreatedAt: created},
		{ID: newerCommentID, Text: target, CreatedAt: created.Add(time.Hour)},
	}
	coordinators := newTestSecondaryCoordinators(project)
	model := newProjectCommentsPanel(project, coordinators.ProjectComment, func(id string) *models.Project {
		if project.ID == id {
			return project
		}
		return nil
	})
	model.SetSize(100, 30)
	return model, project
}

// TestProjectCommentRecoveredDraftDiscardUsesFullText holds the discard question
// to the whole of what a discard would destroy.
//
// The panel keeps a verbatim copy of a recovered draft precisely because the
// composer beside it cannot: the box stops at CommentCharLimit and rewrites tabs
// as four spaces. Both of those can land the box on a string that looks like
// nothing was written — the original comment, or whitespace alone — while the
// copy behind it holds text that exists nowhere else in the process and cannot be
// reopened, the editor's temporary file being deleted before the result ever
// arrives. An Esc guard that measured only the box would then take the shortcut
// meant for an untouched composer and destroy the one complete copy without ever
// asking. Each case below drives a real external result and a real Esc through
// Update after the target or the owner is gone, and requires the question, the
// exact draft behind it, and a workspace nothing writes to.
func TestProjectCommentRecoveredDraftDiscardUsesFullText(t *testing.T) {
	t.Run("edit_truncated_to_original", func(t *testing.T) {
		// The composer has room for the original and not one rune more, so seating
		// the editor's result truncates it back to exactly what was being replaced.
		original := strings.Repeat("x", CommentCharLimit)
		draft := original + discardDraftTail

		model, project := discardDraftFixture(t, original)
		model.Cursor = 0 // the newest comment, presented first
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		// The editor was launched from the list, so the comment it was opened on is
		// removed by another surface while nothing is on screen to hold the result.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft))
		assertComposerSeatedBackTo(t, model, commentMutationEdit, original)
		assertVisible(t, model, errCommentDeletedExternally.Error())
		assertDiscardIsAskedFirst(t, model, project, commentMutationEdit, draft, "Older")
	})

	t.Run("edit_tabs_normalize_to_original", func(t *testing.T) {
		// Nothing here is long. The composer rewrites every tab as four spaces
		// however much room it has, and the original is spelled with those spaces,
		// so a short draft seats back to it just as completely as a truncated one.
		original := "column    second\nindented    value\n" + discardDraftTail
		draft := "column\tsecond\nindented\tvalue\n" + discardDraftTail

		model, project := discardDraftFixture(t, original)
		model.Cursor = 0
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft))
		assertComposerSeatedBackTo(t, model, commentMutationEdit, original)
		assertVisible(t, model, errCommentDeletedExternally.Error())
		assertDiscardIsAskedFirst(t, model, project, commentMutationEdit, draft, "Older")
	})

	t.Run("add_whitespace_prefix", func(t *testing.T) {
		// An add has no original to compare against, only a blank-box shortcut. A
		// result whose leading whitespace alone fills the composer is nonblank text
		// the panel accepted and then seated as nothing at all.
		draft := strings.Repeat(" ", CommentCharLimit+1) + discardDraftTail

		model, project := removedOwnerPanel(t)

		deliverToPanel(t, model, editorResult(model, commentMutationAdd, CommentRef{}, draft))
		if seated := model.TextInput.Value(); strings.TrimSpace(seated) != "" {
			t.Fatalf("the composer seated %q, want whitespace alone: this case proves nothing unless the blank-box shortcut is the one Esc would take",
				tailOf(seated))
		}
		assertVisible(t, model, errCommentOwnerGone.Error())
		assertDiscardIsAskedFirst(t, model, project, commentMutationAdd, draft, "Older", "Newer")
	})
}

// assertComposerSeatedBackTo pins the seating each edit case turns on. Without
// it a change in what the textarea does to a long or tab-carrying string would
// leave these cases passing while testing an ordinary edit: the shortcut they
// exist to defeat only fires when the box holds the original exactly.
func assertComposerSeatedBackTo(t *testing.T, model *CommentsModel, kind commentMutationKind, want string) {
	t.Helper()
	got := model.composerValue(kind)
	if got != want {
		t.Fatalf("the composer seated %q, want the original %q: this case proves nothing unless the unchanged-composer shortcut is the one Esc would take",
			tailOf(got), tailOf(want))
	}
}

// assertDiscardIsAskedFirst is the shared body of every case: Esc asks, a no
// keeps everything, and only an explicit yes ends the retention — with the whole
// draft still recoverable at each step and the stored comments untouched by all
// of it.
func assertDiscardIsAskedFirst(
	t *testing.T,
	model *CommentsModel,
	project *models.Project,
	kind commentMutationKind,
	draft string,
	stored ...string,
) {
	t.Helper()

	assertRecoverable(t, model, kind, draft)
	assertStoredComments(t, project, stored...)

	deliverToPanel(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.IsConfirmActive() {
		t.Fatal("Esc discarded the retained draft without asking: the guard answered from the composer, which is not holding the draft")
	}
	assertRecoverable(t, model, kind, draft)
	assertStoredComments(t, project, stored...)

	// Answering no keeps all three: the composer, the copy behind it, and the
	// comments the refusal never wrote to.
	deliverToPanel(t, model, tea.KeyPressMsg{Code: 'n', Text: "n"})
	if model.IsConfirmActive() {
		t.Fatal("the discard prompt stayed open after it was answered")
	}
	if !model.HasTextInput() {
		t.Fatalf("a rejected discard closed the composer anyway: adding=%v editing=%v", model.AddMode, model.EditMode)
	}
	assertRecoverable(t, model, kind, draft)
	assertStoredComments(t, project, stored...)

	// Asking again and saying yes is the one grammar allowed to end it.
	deliverToPanel(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.IsConfirmActive() {
		t.Fatal("the second Esc discarded the retained draft without asking")
	}
	deliverToPanel(t, model, tea.KeyPressMsg{Code: 'y', Text: "y"})
	if model.HasTextInput() {
		t.Fatalf("a confirmed discard left a composer open: adding=%v editing=%v", model.AddMode, model.EditMode)
	}
	if _, held := model.retainedDraftText(commentMutationAdd); held {
		t.Fatal("a confirmed discard kept the add draft")
	}
	if _, held := model.retainedDraftText(commentMutationEdit); held {
		t.Fatal("a confirmed discard kept the edit draft")
	}
	if view := panelText(t, model); strings.Contains(view, discardDraftTail) {
		t.Fatalf("the discarded draft is still on screen:\n%s", view)
	}
	assertStoredComments(t, project, stored...)
}

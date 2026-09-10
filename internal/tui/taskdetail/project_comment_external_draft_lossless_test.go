package taskdetail

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// externalDraftSentinel ends every result below. It is the last thing the user
// wrote, so it is the first thing a lossy recovery drops: an assertion that finds
// it has proven the whole text arrived, and one that misses it has caught exactly
// the truncation this file exists for.
const externalDraftSentinel = "SENTINEL-END-4f21"

// losslessDraftCase is one external-editor result and the way it defeats a
// textarea. The two long ones exceed CommentCharLimit in runes, which is where
// the composer stops accepting input; the third is short and carries interior
// tabs, which the composer rewrites as spaces however much room it has. Between
// them they cover both halves of the sanitizer a quick-entry box applies and a
// recovered draft must not.
type losslessDraftCase struct {
	name string
	text string
}

func losslessDraftCases() []losslessDraftCase {
	return []losslessDraftCase{
		{
			name: "long_ascii",
			text: strings.Repeat("x", CommentCharLimit+64) + "\n" + externalDraftSentinel,
		},
		{
			name: "long_unicode",
			text: strings.Repeat("héllo→", CommentCharLimit/3) + "\n" + externalDraftSentinel,
		},
		{
			name: "tabs",
			text: "column\tsecond\tthird\nindented\tvalue\n" + externalDraftSentinel,
		},
	}
}

// assertRecoveryUIShowsTail reads the rendered panel and requires the sentinel to
// appear below the recovery notice, not merely somewhere on screen. The composer
// draws the head of the draft on its own, so a sentinel found anywhere would be
// satisfied by the truncated copy in the very box that dropped it.
func assertRecoveryUIShowsTail(t *testing.T, model *CommentsModel) {
	t.Helper()
	view := panelText(t, model)
	notice := strings.Index(view, recoveredDraftNotice)
	if notice < 0 {
		t.Fatalf("the panel never says the box is not showing the whole draft:\n%s", view)
	}
	if !strings.Contains(view[notice:], externalDraftSentinel) {
		t.Fatalf("the recovery block does not reach the end of the draft:\n%s", view[notice:])
	}
}

// discardRetainedDraft spends the one grammar that may drop a recovered draft:
// Esc asks, y answers. Nothing about the size of the draft or the disappearance
// of its target shortens that question.
func discardRetainedDraft(t *testing.T, model *CommentsModel) {
	t.Helper()
	deliverToPanel(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.IsConfirmActive() {
		t.Fatal("Esc over a recovered draft did not ask before discarding it")
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
	if view := panelText(t, model); strings.Contains(view, externalDraftSentinel) {
		t.Fatalf("the discarded draft is still on screen:\n%s", view)
	}
}

// ctrlS is the retry every case makes after its refusal.
var ctrlS = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}

// TestProjectCommentExternalDraftRetentionIsLossless is the lossless half of the
// no-lost-drafts contract.
//
// Keeping a refused draft is not the same as keeping all of it. The composer the
// panel seats an external editor's result into is a quick-entry box: it stops at
// CommentCharLimit and rewrites tabs as four spaces, both of which are correct
// for something being typed and destructive for something being recovered — and
// the editor's temporary file is deleted before the result ever reaches the
// panel, so whatever the box drops is gone from the process entirely. Every case
// below therefore asserts the editor's exact bytes are still reachable, that the
// end of the draft is on screen rather than merely stored, that the refusal is
// visible, that nothing was written; and then puts the retention through the
// three things that could quietly end it — a reload, a second refused save, and
// the discard question that is the only thing allowed to.
func TestProjectCommentExternalDraftRetentionIsLossless(t *testing.T) {
	t.Run("external_result_deleted", func(t *testing.T) {
		for _, testCase := range losslessDraftCases() {
			t.Run(testCase.name, func(t *testing.T) {
				model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
				model.SetSize(100, 30)
				model.Cursor = 0
				ref, ok := model.commentRefAt(0)
				if !ok {
					t.Fatal("the panel drew no comment row")
				}

				// The editor was launched from the list, so no composer is open when the
				// comment it was opened on is removed by another surface.
				project.Comments = project.Comments[:1]
				saveExternalChange(t, project)

				refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, testCase.text)))
				if !strings.Contains(refusal, "deleted externally") {
					t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
				}
				assertRetainedThroughRefusals(t, model, project, commentMutationEdit, testCase.text,
					errCommentDeletedExternally.Error(), "deleted externally", "Older")
			})
		}
	})

	t.Run("external_add_owner_gone", func(t *testing.T) {
		for _, testCase := range losslessDraftCases() {
			t.Run(testCase.name, func(t *testing.T) {
				model, project := removedOwnerPanel(t)

				refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationAdd, CommentRef{}, testCase.text)))
				if !strings.Contains(refusal, "no longer available") {
					t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
				}
				assertRetainedThroughRefusals(t, model, project, commentMutationAdd, testCase.text,
					errCommentOwnerGone.Error(), "no longer available", "Older", "Newer")
			})
		}
	})

	t.Run("external_edit_owner_gone", func(t *testing.T) {
		for _, testCase := range losslessDraftCases() {
			t.Run(testCase.name, func(t *testing.T) {
				model, project := removedOwnerPanel(t)
				ref, ok := model.commentRefAt(0)
				if !ok {
					t.Fatal("the panel drew no comment row")
				}

				refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, testCase.text)))
				if !strings.Contains(refusal, "no longer available") {
					t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
				}
				assertRetainedThroughRefusals(t, model, project, commentMutationEdit, testCase.text,
					errCommentOwnerGone.Error(), "no longer available", "Older", "Newer")
			})
		}
	})
}

// assertRetainedThroughRefusals runs the shared tail of every case: the state
// immediately after the refusal, then the three events that must not end the
// retention and the one that must.
func assertRetainedThroughRefusals(
	t *testing.T,
	model *CommentsModel,
	project *models.Project,
	kind commentMutationKind,
	text string,
	visibleRefusal string,
	retryRefusal string,
	stored ...string,
) {
	t.Helper()

	assertRecoverable(t, model, kind, text)
	assertRecoveryUIShowsTail(t, model)
	assertVisible(t, model, visibleRefusal)
	assertStoredComments(t, project, stored...)

	// A reload is not an answer to anything the user asked, so it may not cost
	// them the draft it happens to arrive beside.
	if !model.RefreshAfterReload() {
		t.Fatal("the refresh asked the shell to dismiss a panel holding a recovered draft")
	}
	assertRecoverable(t, model, kind, text)
	assertRecoveryUIShowsTail(t, model)

	// Saving again is refused for the same reason it was refused the first time,
	// and the retry carries the whole draft rather than the composer's copy of it:
	// a refusal that wrote the truncation back would be the defect wearing the
	// repair's clothes.
	retry := refusalText(t, deliverToPanel(t, model, ctrlS))
	if !strings.Contains(retry, retryRefusal) {
		t.Fatalf("retry refusal = %q, want %q", retry, retryRefusal)
	}
	assertRecoverable(t, model, kind, text)
	assertRecoveryUIShowsTail(t, model)
	assertVisible(t, model, visibleRefusal)
	assertStoredComments(t, project, stored...)

	discardRetainedDraft(t, model)
	assertStoredComments(t, project, stored...)
}

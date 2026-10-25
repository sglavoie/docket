package taskdetail

import (
	"strings"
	"testing"

	"github.com/sglavoie/docket/internal/models"
)

// editorClaimIsQualified reads one rendered sentence about Ctrl+E the way a user
// would: as an invitation, or as a condition it does not currently meet. It
// shares the closed qualifier set with the availability regression beside it,
// because the two files are asking the same question of the same sentences and a
// second vocabulary would let one of them start accepting what the other refuses.
func editorClaimIsQualified(claim string) bool {
	lowered := strings.ToLower(claim)
	for _, word := range editorClaimQualifiers {
		if strings.Contains(lowered, word) {
			return true
		}
	}
	return false
}

// assertEditorClaimsAreInvitations reads every sentence the panel is currently
// making about Ctrl+E, requires all of them to read as an open route, and returns
// how many there were.
//
// The count is the baseline the loss is measured against. The panel makes this
// claim in two places at once — the block above the composer and the refusal
// status under it — and the defect this file locks is one of them moving on while
// the other stays behind, so a later reading that found fewer sentences would be
// passing by having gone quiet rather than by having corrected itself.
func assertEditorClaimsAreInvitations(t *testing.T, model *CommentsModel, when string) int {
	t.Helper()
	claims := renderedEditorClaims(t, model)
	if len(claims) < 2 {
		t.Fatalf("%s: the panel makes %d claim(s) about Ctrl+E, want the recovery block's and the refusal status's:\n%s",
			when, len(claims), panelText(t, model))
	}
	for _, claim := range claims {
		if editorClaimIsQualified(claim) {
			t.Fatalf("%s: the panel qualifies an editor route that is open to the user: %q", when, claim)
		}
	}
	return len(claims)
}

// assertEditorClaimsCaughtUp is the contract itself. Everything the panel is
// still saying about Ctrl+E has to have been rewritten by the loss — not one
// sentence of it, all of it — and it has to still be saying as much as it was
// before, with the export route named as plainly as ever.
func assertEditorClaimsCaughtUp(t *testing.T, model *CommentsModel, when string, promises int) {
	t.Helper()
	claims := renderedEditorClaims(t, model)
	if len(claims) != promises {
		t.Fatalf("%s: the panel makes %d claim(s) about Ctrl+E, want the %d it made while the target was there:\n%s",
			when, len(claims), promises, panelText(t, model))
	}
	for _, claim := range claims {
		if !editorClaimIsQualified(claim) {
			t.Fatalf("%s: the panel still says %q while Ctrl+E answers with a refusal", when, claim)
		}
	}
	assertExportRouteStaysNamed(t, model, when)
}

// retainedEditDraftOverPresentTarget puts the panel in the state the transition
// starts from: an edit draft too long and too tabbed for the composer to hold,
// kept beside a comment and a project that are both still there.
//
// The refusal that strands it has to come from the far side of the panel's own
// preconditions, or the target would already be gone and there would be no
// transition to observe. So the project is archived for exactly one submission,
// which the application layer refuses while leaving the owner and the comment
// where they were, and then unarchived again.
func retainedEditDraftOverPresentTarget(
	t *testing.T,
	draft string,
	width, height int,
) (*CommentsModel, *models.Project, func(bool)) {
	t.Helper()
	model, project, ownerPresent := inlineEditFixture(t)
	model.SetSize(width, height)
	model.Cursor = 0
	ref, ok := model.commentRefAt(0)
	if !ok {
		t.Fatal("the panel drew no comment row")
	}

	project.Archived = true
	msg, refused := settle(t, model, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft))).(CommentMutationFailedMsg)
	if !refused {
		t.Fatal("the save against an archived project was not refused by the application layer")
	}
	if msg.Err == nil || !strings.Contains(msg.Err.Error(), "archived") {
		t.Fatalf("refusal = %v, want it to name the archived project", msg.Err)
	}
	project.Archived = false

	if !model.externalDraftEditorAvailable(commentMutationEdit) {
		t.Fatal("the draft's target is already unreachable, so there is no loss left to observe")
	}
	assertPreviewIsShortOfTheDraft(t, model, commentMutationEdit)
	return model, project, ownerPresent
}

// assertGuidanceCatchesUpWithTheLoss is the shared tail of both cases: the two
// events that can reach the panel after the target goes away, each followed by a
// reading of what is on screen.
//
// Neither is an inline edit, which is the point. The refusal status was written
// by one and would be rewritten by another, so a case that pressed a key into the
// composer again would be proving that the panel can say the right thing when
// asked afresh — not that it stops saying the wrong thing on its own. A refresh
// asks the panel nothing at all, and Ctrl+E asks it about the editor without
// touching the box.
//
// The draft and the workspace are then checked the way every case in this area
// checks them. Explaining a refusal is not a mutation, and neither is refusing
// again: the bytes the user can still recover are the editor's, and the only
// thing that changed in the project is the loss the test simulated itself.
func assertGuidanceCatchesUpWithTheLoss(
	t *testing.T,
	model *CommentsModel,
	project *models.Project,
	draft string,
	promises int,
	wantRefusal string,
	stored ...string,
) {
	t.Helper()
	if !model.RefreshAfterReload() {
		t.Fatal("the refresh asked the shell to dismiss a panel holding a recovered draft")
	}
	assertEditorClaimsCaughtUp(t, model, "after the refresh that observed the loss", promises)

	if refusal := refusalText(t, deliverToPanel(t, model, ctrlE)); !strings.Contains(refusal, wantRefusal) {
		t.Fatalf("Ctrl+E refusal = %q, want it to name %q", refusal, wantRefusal)
	}
	assertEditorClaimsCaughtUp(t, model, "after Ctrl+E answered with a refusal", promises)

	if !model.HasTextInput() {
		t.Fatal("the loss closed the composer over a kept draft")
	}
	assertRecoverable(t, model, commentMutationEdit, draft)

	// Last, because it takes the status line for the path it writes. That the path
	// arrives readable is the other half of the contract: recomputing the refusal
	// may not leak a sentence into a status the user has to read character for
	// character.
	assertExportedDraft(t, model, map[string]bool{}, draft)
	assertStoredComments(t, project, stored...)
}

// TestProjectCommentRecoveredDraftGuidanceRefreshesAfterTargetLoss holds the
// panel to its own advice after the state that advice was written in has gone.
//
// The guidance beside a recovered draft names two keys, and only one of them is
// unconditional: Ctrl+X writes a file out of memory the panel already holds,
// while Ctrl+E has to reach the project and the comment the composer was opened
// on. A draft can be stranded while both of those are still there — the
// application layer refuses a save against an archived project without taking
// anything away — and then lose them a moment later, to a delete on another
// surface or to the project going away. Nothing about that loss rewrites a
// sentence that was already printed, so a status message frozen at the moment of
// an inline-edit refusal would go on promising an editor beside a recovery block
// that had already recomputed and said the opposite.
//
// So each case below arrives at the refusal the honest way — the composer turns
// away two ordinary keystrokes while the target is present — reads the panel
// while every sentence about the editor is still an open invitation, and only
// then takes the target away. What follows asks the panel nothing new: a refresh,
// and the editor key itself. After each, everything on screen about Ctrl+E has to
// have caught up, none of it may have gone quiet to do so, Ctrl+X has to still be
// spelled out, and the draft and the project have to be exactly where the loss
// left them.
func TestProjectCommentRecoveredDraftGuidanceRefreshesAfterTargetLoss(t *testing.T) {
	// The export writes into the system temporary directory and Ctrl+E resolves an
	// editor from the environment. Both are pointed somewhere this run owns: the
	// drafts do not outlive the test, and no editor is ever launched — every press
	// of Ctrl+E here is refused before a command is built.
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("VISUAL", "true")
	t.Setenv("EDITOR", "true")

	// Wide enough that each composed sentence is one line on screen, which is what
	// lets a claim be read back off it. A width that split one would find fewer
	// claims than the panel makes, and both readings say so.
	const wide, tall = 160, 40
	draft := exportDraftText()

	t.Run("deleted_comment", func(t *testing.T) {
		model, project, _ := retainedEditDraftOverPresentTarget(t, draft, wide, tall)
		assertInlineEditIsRefused(t, model, commentMutationEdit, draft)
		promises := assertEditorClaimsAreInvitations(t, model, "while the comment was still in the list")

		// Another surface removes exactly the comment the composer was opened on,
		// while nothing the user does is what observes it.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		assertGuidanceCatchesUpWithTheLoss(
			t, model, project, draft, promises, "deleted externally", "Older")

		// The refusal is exactly as true after all of that as before it. Nothing the
		// guidance does may bring the comment back to make its earlier advice come
		// true.
		if _, found := model.refreshRef(model.editRef); found {
			t.Fatal("the panel resurrected the deleted comment")
		}
	})

	t.Run("missing_owner", func(t *testing.T) {
		model, project, ownerPresent := retainedEditDraftOverPresentTarget(t, draft, wide, tall)
		assertInlineEditIsRefused(t, model, commentMutationEdit, draft)
		promises := assertEditorClaimsAreInvitations(t, model, "while the project was still in the catalog")

		// The project goes away the way a delete or an archive-and-reload makes it go
		// away: the resolver stops answering for it, and the comments it carries are
		// left exactly as they are.
		ownerPresent(false)

		assertGuidanceCatchesUpWithTheLoss(
			t, model, project, draft, promises, "no longer available", "Older", "Newer")

		if model.owner.Reload() {
			t.Fatal("the comments panel recreated the removed owner")
		}
	})
}

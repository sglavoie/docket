package taskdetail

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/tui/status"
)

// ctrlE is the one key beside a recovered draft whose answer depends on the
// workspace. Ctrl+X asks nothing of anybody — it writes a file out of memory the
// panel already holds — while Ctrl+E has to reach an owner, and for an edit the
// comment the composer was opened on as well.
var ctrlE = tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl}

// editorClaimQualifiers is the closed set of words that turn a sentence about
// Ctrl+E from an instruction into a condition. The test matches meaning rather
// than wording, so the production hints may be rewritten freely; what it will not
// let them become is a sentence that reads as "press this" beside a key that
// answers with a refusal.
var editorClaimQualifiers = []string{"cannot", "can't", "needs", "requires", "unavailable", "not available", "no longer"}

// renderedEditorClaims collects every complete sentence the panel is currently
// making about Ctrl+E, read off the screen the user is looking at.
//
// A sentence is the unit because a sentence is what asserts something. The footer
// names the same key as a rank — a label between separators, carrying no
// terminator — and a rank says only that the key exists on this surface, which is
// true in every state this test drives. Requiring the claim to end on its own
// line is what leaves those ranks out without the test having to know what the
// footer says.
func renderedEditorClaims(t *testing.T, model *CommentsModel) []string {
	t.Helper()
	var claims []string
	for _, line := range strings.Split(panelText(t, model), "\n") {
		start := strings.Index(line, "Ctrl+E")
		if start < 0 {
			continue
		}
		rest := line[start:]
		end := strings.IndexByte(rest, '.')
		if end < 0 {
			continue
		}
		claims = append(claims, strings.TrimSpace(rest[:end+1]))
	}
	return claims
}

// assertEditorGuidanceMatchesAvailability is the whole contract in one place:
// everything the panel is saying about the external editor has to agree with what
// the editor key would actually do, and the export route has to stay named
// whichever answer that is.
//
// Both directions matter. A panel that promised the editor beside a draft whose
// target is gone would spend the user's one remaining move on a refusal; a panel
// that hedged the editor beside a draft it can still open would push them toward
// the clumsier of two working routes. And the export sentence is asserted in
// every state because it is the half that is true in all of them — the recovery
// is only reachable if the key that performs it is still on screen.
func assertEditorGuidanceMatchesAvailability(t *testing.T, model *CommentsModel, when string, editorAvailable bool) {
	t.Helper()
	claims := renderedEditorClaims(t, model)
	if len(claims) == 0 {
		t.Fatalf("%s: the panel makes no claim about Ctrl+E beside a draft it will not let the box edit:\n%s",
			when, panelText(t, model))
	}
	for _, claim := range claims {
		qualified := false
		for _, word := range editorClaimQualifiers {
			if strings.Contains(strings.ToLower(claim), word) {
				qualified = true
				break
			}
		}
		switch {
		case editorAvailable && qualified:
			t.Fatalf("%s: the panel qualifies an editor route that is open to the user: %q", when, claim)
		case editorAvailable && !strings.Contains(strings.ToLower(claim), "editor"):
			t.Fatalf("%s: the available route is named without saying it reaches an editor: %q", when, claim)
		case !editorAvailable && !qualified:
			t.Fatalf("%s: the panel says %q while Ctrl+E answers with a refusal", when, claim)
		}
	}
	assertExportRouteStaysNamed(t, model, when)
}

// assertExportRouteStaysNamed requires the one act that cannot fail to still be
// spelled out. Explaining why the editor is unreachable is worth nothing if the
// explanation crowds out the key that hands the draft back.
func assertExportRouteStaysNamed(t *testing.T, model *CommentsModel, when string) {
	t.Helper()
	view := panelText(t, model)
	for _, line := range strings.Split(view, "\n") {
		start := strings.Index(line, "Ctrl+X")
		if start < 0 {
			continue
		}
		if strings.Contains(line[start:], ".") {
			return
		}
	}
	t.Fatalf("%s: the panel stopped telling the user how to get the draft out:\n%s", when, view)
}

// assertEditorKeyAnswersAsPromised presses Ctrl+E and requires the answer the
// guidance just described, without ever running an editor: the launch resolves to
// bubbletea's exec message, which is data, so the control proves the panel handed
// the buffer on rather than proving anything about a subprocess.
//
// The draft is checked either way. A refused launch must leave it exactly as the
// external editor delivered it — that is the retention this whole area exists for
// — and an accepted one must not have quietly edited it on the way past.
func assertEditorKeyAnswersAsPromised(
	t *testing.T,
	model *CommentsModel,
	kind commentMutationKind,
	draft string,
	editorAvailable bool,
	wantRefusal string,
) {
	t.Helper()
	composer := model.composerValue(kind)
	// The refusal that stranded the draft is still on screen and is still true, so
	// what the control asserts is that the press added no second one — not that the
	// panel is reporting nothing at all. The comparison goes through errors.Is so a
	// refusal that merely wrapped the stranding one would still read as the same
	// claim rather than as a new one.
	stranded := model.Err
	cmd := deliverToPanel(t, model, ctrlE)

	if editorAvailable {
		if cmd == nil {
			t.Fatal("Ctrl+E did nothing at all beside a draft whose target is still there")
		}
		if msg, refused := cmd().(status.ErrorMsg); refused {
			t.Fatalf("Ctrl+E was refused with %v while the panel described the route as open", msg.Err)
		}
		if !errors.Is(model.Err, stranded) {
			t.Fatalf("the panel reports %v after a launch it said was available", model.Err)
		}
	} else {
		if refusal := refusalText(t, cmd); !strings.Contains(refusal, wantRefusal) {
			t.Fatalf("Ctrl+E refusal = %q, want it to name %q", refusal, wantRefusal)
		}
	}

	if got := model.composerValue(kind); got != composer {
		t.Fatalf("Ctrl+E rewrote the composer:\ngot  %q\nwant %q", tailOf(got), tailOf(composer))
	}
	if !model.HasTextInput() {
		t.Fatal("Ctrl+E closed the composer over a kept draft")
	}
	assertRecoverable(t, model, kind, draft)
}

// assertGuidanceTracksEditorAvailability is the shared body of all three cases.
//
// It reads the guidance before the user touches anything, spends the two ordinary
// edits the composer must refuse, reads it again now that the refusal status is on
// screen beside the recovery block, and only then presses the key both of them are
// about. The two readings are separate on purpose: the block and the status are
// the same claim printed in two places, and the second reading is the one where
// they are both visible and could disagree.
//
// Whatever the answer, the draft is then exported and the workspace named. Neither
// explaining the editor nor refusing it may cost the user a rune of their text or
// write anything to the project.
func assertGuidanceTracksEditorAvailability(
	t *testing.T,
	model *CommentsModel,
	project *models.Project,
	kind commentMutationKind,
	draft string,
	editorAvailable bool,
	wantRefusal string,
	stored ...string,
) {
	t.Helper()
	assertPreviewIsShortOfTheDraft(t, model, kind)

	assertEditorGuidanceMatchesAvailability(t, model, "before the inline edit", editorAvailable)
	assertInlineEditIsRefused(t, model, kind, draft)
	assertEditorGuidanceMatchesAvailability(t, model, "after the refused inline edit", editorAvailable)

	assertEditorKeyAnswersAsPromised(t, model, kind, draft, editorAvailable, wantRefusal)
	assertEditorGuidanceMatchesAvailability(t, model, "after pressing Ctrl+E", editorAvailable)

	assertExportedDraft(t, model, map[string]bool{}, draft)
	assertStoredComments(t, project, stored...)
}

// TestProjectCommentRecoveredDraftGuidanceMatchesEditorAvailability holds the
// panel to the acts it advertises beside a draft it is refusing to save.
//
// A recovered draft is the state in which the panel has the least to offer and
// says the most. The box below the notice is a read-only preview, the save has
// already been refused, and the recovery block and the inline-edit refusal both
// answer the question that leaves — what can I still do with my text — by naming
// two keys. One of them, Ctrl+X, needs nothing from the workspace and works in
// every state. The other, Ctrl+E, needs the owner and, for an edit, the comment
// the composer was opened on; and the whole reason a draft is being recovered is
// that one of those is gone. Naming it unconditionally there sends a user whose
// only copy of their work is in this process to a key that answers with a second
// refusal.
//
// So each case below puts the panel in that state by the route it really arrives
// on — an external editor's result landing on a target that left — and reads the
// rendered guidance as sentences rather than as the constants that produced them,
// requiring every claim about the editor to agree with what the key does when it
// is actually pressed. The two loss cases require the qualification and the
// control requires its absence, which is what keeps the guidance from being fixed
// by making it vague. Through all of it the draft has to survive intact and the
// project has to be untouched: explaining a refusal is not a mutation, and neither
// is refusing again.
func TestProjectCommentRecoveredDraftGuidanceMatchesEditorAvailability(t *testing.T) {
	// The export writes into the system temporary directory and the control
	// resolves an editor from the environment. Both are pointed somewhere this run
	// owns: the drafts do not outlive the test, and no editor is ever launched —
	// the resolved command is only ever inspected.
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("VISUAL", "true")
	t.Setenv("EDITOR", "true")

	// Wide enough that the composed hint is one line on screen, which is what lets
	// a sentence be read back off it. A width that split the claim would find no
	// claim at all, and assertEditorGuidanceMatchesAvailability says so.
	const wide, tall = 160, 40
	draft := exportDraftText()

	t.Run("deleted_comment", func(t *testing.T) {
		model, project := newTestProjectCommentsPanel(t, projectCommentsFixture(t))
		model.SetSize(wide, tall)
		model.Cursor = 0
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		// The editor was opened from the list, so the comment it was opened on is
		// removed by another surface while nothing is on screen to hold the result.
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft)))
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}

		assertGuidanceTracksEditorAvailability(
			t, model, project, commentMutationEdit, draft, false, "deleted externally", "Older")

		// The refusal is exactly as true after all of that as before it. Nothing the
		// guidance does may bring the comment back to make its own advice come true.
		if _, found := model.refreshRef(model.editRef); found {
			t.Fatal("the panel resurrected the deleted comment")
		}
	})

	t.Run("missing_owner", func(t *testing.T) {
		model, project := removedOwnerPanel(t)
		model.SetSize(wide, tall)
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft)))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}

		assertGuidanceTracksEditorAvailability(
			t, model, project, commentMutationEdit, draft, false, "no longer available", "Older", "Newer")

		if model.owner.Reload() {
			t.Fatal("the comments panel recreated the removed owner")
		}
	})

	t.Run("target_present", func(t *testing.T) {
		// The control has to be a genuine recovered draft, or it would prove only
		// that an ordinary composer offers an editor. So the draft arrives the same
		// way and is refused the same way — but on the far side of the panel's own
		// preconditions, by the application layer, which leaves the owner and the
		// comment both exactly where they were.
		model, project, _ := inlineEditFixture(t)
		model.SetSize(wide, tall)
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
			t.Fatal("the control is not a case with a reachable target, so it controls for nothing")
		}
		assertGuidanceTracksEditorAvailability(
			t, model, project, commentMutationEdit, draft, true, "", "Older", "Newer")
	})
}

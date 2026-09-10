package taskdetail

import (
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// The three fragments below are the refusal read the way a user reads it, spelled
// out here rather than taken from the production constants.
//
// That is the whole point of writing them twice. An assertion that named the
// constant would be rewritten by any repair that reworded it, and would then go on
// passing over a panel still making the claim in different words; these are what
// the sentence has to stop saying, whatever the panel decides to call it. The bare
// phrase catches a reworded one, and the two specific fragments say which sentence
// went missing when it fails.
const (
	staleRecoveryClaim       = "Read-only preview of the kept draft"
	staleRecoveryExportClaim = "Ctrl+X writes the whole of it to a file."
	anyReadOnlyClaim         = "read-only"
)

// ordinaryComposerText is what recoveredDraftEditPresses leaves in an empty box:
// the backspace finds nothing to delete and the rune lands. The same two presses
// earn the refusal over a preview and are simply taken by an ordinary composer,
// which is what makes "the box is editable again" a fact here rather than an
// absence.
const ordinaryComposerText = "Z"

// retainedLossyDraft puts the panel in the state every case here starts from: a
// draft too long and too tabbed for the composer to hold, kept beside a comment
// and a project that are both still there, with the read-only refusal on screen
// because the user has just tried to type into it.
//
// The refusal that strands the draft comes from the far side of the panel's own
// preconditions — the project is archived for exactly one submission, which the
// application layer refuses without taking anything away — so the target is
// present for everything that follows. That matters twice over here: a draft whose
// comment or project were gone could not be committed at all, and half of these
// cases end by committing one.
func retainedLossyDraft(t *testing.T, kind commentMutationKind, draft string) (*CommentsModel, *models.Project) {
	t.Helper()
	model, project, _ := inlineEditFixture(t)
	// Wide enough that each sentence the panel composes is one line on screen,
	// which is what lets a claim — and the absence of one — be read back off it.
	model.SetSize(160, 40)
	model.Cursor = 0

	ref := CommentRef{}
	if kind == commentMutationEdit {
		var ok bool
		if ref, ok = model.commentRefAt(0); !ok {
			t.Fatal("the panel drew no comment row")
		}
	}

	project.Archived = true
	msg, refused := settle(t, model, deliverToPanel(t, model, editorResult(model, kind, ref, draft))).(CommentMutationFailedMsg)
	if !refused {
		t.Fatal("the save against an archived project was not refused by the application layer")
	}
	if msg.Err == nil || !strings.Contains(msg.Err.Error(), "archived") {
		t.Fatalf("refusal = %v, want it to name the archived project", msg.Err)
	}
	project.Archived = false

	assertPreviewIsShortOfTheDraft(t, model, kind)
	assertRecoverable(t, model, kind, draft)
	assertPreviewRefusalIsOnScreen(t, model, kind)
	return model, project
}

// assertPreviewRefusalIsOnScreen spends two ordinary keystrokes on the preview and
// requires the panel to turn them away in writing.
//
// It is the baseline the rest of the file is measured against. A case that never
// saw the claim could pass its absence by never having earned it, so the refusal
// is put on screen through Update the way a user puts it there, and read off the
// rendered view rather than off the model. Both presses are spent because a box
// already holding CharLimit runes takes no rune at all: a case that typed one and
// stopped would be reading a press the textarea ignored rather than one the panel
// refused.
func assertPreviewRefusalIsOnScreen(t *testing.T, model *CommentsModel, kind commentMutationKind) {
	t.Helper()
	before := model.composerValue(kind)
	for _, press := range recoveredDraftEditPresses() {
		deliverToPanel(t, model, press)
		if got := model.composerValue(kind); got != before {
			t.Fatalf("%s edited the read-only preview, so there is no refusal to observe:\ngot  %q\nwant %q",
				press.String(), tailOf(got), tailOf(before))
		}
	}
	assertVisible(t, model, staleRecoveryClaim, staleRecoveryExportClaim)
}

// assertNoRecoveryStatusClaim requires the refusal's own two sentences to be off
// the screen. They are the status line's alone — the recovery block above the
// composer says the same thing in its own words — so this is the one reading that
// can be taken while a live preview is still drawn.
func assertNoRecoveryStatusClaim(t *testing.T, model *CommentsModel, when string) {
	t.Helper()
	view := panelText(t, model)
	for _, claim := range []string{staleRecoveryClaim, staleRecoveryExportClaim} {
		if strings.Contains(view, claim) {
			t.Fatalf("%s: the panel still says %q:\n%s", when, claim, view)
		}
	}
}

// assertNoReadOnlyClaim goes further and requires the panel to be making no claim
// about a read-only box at all, in any words.
//
// The blanket reading is what a reworded refusal cannot slip past, and it is only
// meaningful where there is no kept draft: the recovery block draws a read-only
// sentence of its own, truthfully, for as long as one is being previewed. Every
// caller here reads a panel that has none.
func assertNoReadOnlyClaim(t *testing.T, model *CommentsModel, when string) {
	t.Helper()
	assertNoRecoveryStatusClaim(t, model, when)
	if view := panelText(t, model); strings.Contains(strings.ToLower(view), anyReadOnlyClaim) {
		t.Fatalf("%s: the panel still describes something on screen as read-only:\n%s", when, view)
	}
}

// assertDraftClaimEndedWithTheDraft is the contract itself, read in the two places
// the stale line reaches: the browse view the ended draft leaves behind, and the
// next composer the user opens.
//
// The second is the one that turns a stale line into a false one. That composer is
// ordinary — nothing is kept behind it and every keystroke lands in it — so a
// sentence calling it a read-only preview of a kept draft is not out of date about
// a past state but wrong about the present one, and it would be inviting the user
// to Ctrl+X text the panel no longer has.
func assertDraftClaimEndedWithTheDraft(t *testing.T, model *CommentsModel, when string) {
	t.Helper()
	if model.HasTextInput() {
		t.Fatalf("%s: a composer is still open, so this reads nothing about the browse view", when)
	}
	if _, held := model.retainedDraftText(commentMutationAdd); held {
		t.Fatalf("%s: the panel is still holding the add draft", when)
	}
	if _, held := model.retainedDraftText(commentMutationEdit); held {
		t.Fatalf("%s: the panel is still holding the edit draft", when)
	}
	assertNoReadOnlyClaim(t, model, when+", in the browse view")

	model.BeginAddMode()
	for _, press := range recoveredDraftEditPresses() {
		deliverToPanel(t, model, press)
	}
	if got := model.TextInput.Value(); got != ordinaryComposerText {
		t.Fatalf("%s: the next composer holds %q after the same two presses, want %q: it is not the ordinary editable box this reads",
			when, got, ordinaryComposerText)
	}
	assertNoReadOnlyClaim(t, model, when+", in the ordinary composer opened after it")
}

// discardTheDraft ends a retention the way a user ends one: Esc asks, and only an
// explicit yes spends it.
func discardTheDraft(t *testing.T, model *CommentsModel) {
	t.Helper()
	deliverToPanel(t, model, tea.KeyPressMsg{Code: tea.KeyEscape})
	if !model.IsConfirmActive() {
		t.Fatal("Esc over a kept draft did not ask before discarding it")
	}
	deliverToPanel(t, model, tea.KeyPressMsg{Code: 'y', Text: "y"})
	if model.IsConfirmActive() {
		t.Fatal("the discard prompt stayed open after it was answered")
	}
}

// exportedDraftPath reads the path the panel is currently naming, without pressing
// anything. The screen is the contract: a path the user cannot read off it is a
// draft they cannot open.
func exportedDraftPath(t *testing.T, model *CommentsModel, when string) string {
	t.Helper()
	view := panelText(t, model)
	if !strings.Contains(view, strings.TrimSpace(draftExportStatusPrefix)) {
		t.Fatalf("%s: the panel is naming no file for the draft:\n%s", when, view)
	}
	path := exportedDraftPathFromView(view)
	if path == "" {
		t.Fatalf("%s: the panel named an empty path for the draft:\n%s", when, view)
	}
	return path
}

// TestCommentsRecoveredDraftStatusEndsWithDraft holds the read-only refusal to the
// lifetime of the draft it was written about.
//
// The refusal is a sticky status line, and sticky is right for almost everything
// this panel says: a refused save has to outlast the press that earned it, and an
// exported draft's path has to stay on screen precisely because the user needs it
// after the draft is gone. A sentence about what the box in front of you is cannot
// be sticky in the same way, because that box changes. Discarding a kept draft or
// committing it closes the composer and drops the retained copy, and neither writes
// a status of its own — so a line promising a read-only preview and offering to
// write the whole kept draft to a file would go on describing a browse view with no
// composer at all, and then the next ordinary composer the user opens, offering to
// rescue text the panel no longer has.
//
// So each case below reaches the refusal the honest way — a real lossy draft, a
// real keystroke turned away, the claim read off the rendered view — and then ends
// the draft through the two grammars that are allowed to end one: the confirmed
// discard, and a commit that actually lands. What follows asks the panel nothing:
// the browse view, and then a composer opened over it that has to be editable and
// has to be described as editable. The last case is the other half of the rule —
// the same act must not take an exported path with it, so that status is written
// first, the draft is ended, and the path and the file it names have to be exactly
// what they were.
func TestCommentsRecoveredDraftStatusEndsWithDraft(t *testing.T) {
	draft := exportDraftText()

	t.Run("discard_add", func(t *testing.T) {
		model, project := retainedLossyDraft(t, commentMutationAdd, draft)

		discardTheDraft(t, model)

		assertDraftClaimEndedWithTheDraft(t, model, "after the add draft was discarded")
		assertStoredComments(t, project, "Older", "Newer")
	})

	t.Run("discard_edit", func(t *testing.T) {
		model, project := retainedLossyDraft(t, commentMutationEdit, draft)

		discardTheDraft(t, model)

		assertDraftClaimEndedWithTheDraft(t, model, "after the edit draft was discarded")
		assertStoredComments(t, project, "Older", "Newer")
	})

	t.Run("commit_add", func(t *testing.T) {
		model, _ := retainedLossyDraft(t, commentMutationAdd, draft)

		if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentAddedMsg); !ok {
			t.Fatal("the retry against the restored project did not commit")
		}

		assertDraftClaimEndedWithTheDraft(t, model, "after the add draft was committed")
	})

	t.Run("commit_edit", func(t *testing.T) {
		model, _ := retainedLossyDraft(t, commentMutationEdit, draft)

		if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentEditedMsg); !ok {
			t.Fatal("the retry against the restored comment did not commit")
		}

		assertDraftClaimEndedWithTheDraft(t, model, "after the edit draft was committed")
	})

	t.Run("export_status_preserved", func(t *testing.T) {
		// The export writes into the system temporary directory, which is the point:
		// the file has to outlive the draft. Pointing that directory at the test's own
		// keeps this run from leaving copies of the draft behind.
		t.Setenv("TMPDIR", t.TempDir())

		model, _ := retainedLossyDraft(t, commentMutationAdd, draft)

		// Ctrl+X displaces the refusal by writing a status of its own, which is the
		// distinction this case exists for: ending the draft may retire the panel's
		// claim about the box and may not touch the one status a user has to be able
		// to read character for character.
		deliverToPanel(t, model, ctrlX)
		path := exportedDraftPath(t, model, "while the draft was still open")
		assertNoRecoveryStatusClaim(t, model, "once the export took the status line")

		discardTheDraft(t, model)

		if got := exportedDraftPath(t, model, "after the draft was discarded"); got != path {
			t.Fatalf("the exported path changed when the draft ended:\ngot  %q\nwant %q", got, path)
		}
		assertDraftClaimEndedWithTheDraft(t, model, "after the exported draft was discarded")
		if got := exportedDraftPath(t, model, "in the composer opened after the discard"); got != path {
			t.Fatalf("the exported path changed when the next composer opened:\ngot  %q\nwant %q", got, path)
		}

		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("the file the panel still names cannot be read: %v", err)
		}
		if string(contents) != draft {
			t.Fatalf("the exported draft is not the editor's result (%d rune(s) short):\ngot  %q\nwant %q",
				len([]rune(draft))-len([]rune(string(contents))), tailOf(string(contents)), tailOf(draft))
		}
		for _, marker := range []string{exportDraftHead, exportDraftMiddle, exportDraftTail} {
			if !strings.Contains(string(contents), marker) {
				t.Fatalf("the exported draft is missing %q", marker)
			}
		}
	})
}

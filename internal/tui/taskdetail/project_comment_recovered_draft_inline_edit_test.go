package taskdetail

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/docket/internal/models"
	"github.com/sglavoie/docket/internal/storage"
)

// The draft below is exportDraftText(): a head, a middle past CommentCharLimit,
// interior tabs on every line, and a tail. It is shared rather than copied
// because the shape is the argument in both files — the parts of one draft a
// lossy composer loses separately are the parts an edit to that composer can
// silently drop, and two builders would be two chances to stop agreeing about
// which parts those are.

// recoveredDraftEditPresses are two ordinary edits, made without meaning
// anything by them: a backspace over the character the cursor sits on, and a
// rune typed in its place. Between them they cover both halves of an edit, and
// neither is an instruction to replace text the user has never seen.
func recoveredDraftEditPresses() []tea.KeyPressMsg {
	return []tea.KeyPressMsg{
		{Code: tea.KeyBackspace},
		{Code: 'Z', Text: "Z"},
	}
}

// inlineEditFixture mounts the project panel over a resolver the test can switch
// off and back on.
//
// That switch is the apparatus the whole test needs: the owner disappears the way
// a delete or an archive-and-reload makes it disappear, and then comes back as
// itself — same identity, same comments, same panel — so the retry the user makes
// after the refusal is a retry of their own save rather than a second fixture
// standing in for one.
func inlineEditFixture(t *testing.T) (*CommentsModel, *models.Project, func(bool)) {
	t.Helper()
	project := projectCommentsFixture(t)
	present := true
	coordinators := newTestSecondaryCoordinators(project)
	model := newProjectCommentsPanel(project, coordinators.ProjectComment, func(id string) *models.Project {
		if present && project.ID == id {
			return project
		}
		return nil
	})
	model.SetSize(120, 40)
	return model, project, func(visible bool) { present = visible }
}

// persistedProjectComments reads the project back from storage through an object
// nothing in the test has ever held. What the panel wrote is the contract here,
// and the fixture pointer is refreshed by the snapshot on every commit, so
// asserting against it would be asking the writer to grade itself.
func persistedProjectComments(t *testing.T, project *models.Project) []*models.ProjectComment {
	t.Helper()
	fresh, err := models.NewProject(project.ID, project.Name, project.Path)
	if err != nil {
		t.Fatalf("NewProject: %v", err)
	}
	if err := storage.LoadProject(fresh); err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	return fresh.Comments
}

// assertPreviewIsShortOfTheDraft proves the box is not the draft, which every
// assertion below depends on: a composer that happened to be holding the whole
// text could not lose any of it however it were edited.
func assertPreviewIsShortOfTheDraft(t *testing.T, model *CommentsModel, kind commentMutationKind) {
	t.Helper()
	if !model.showsDraftPreview() {
		t.Fatal("the composer is not previewing a kept draft, so this case proves nothing about a read-only preview")
	}
	composer := model.composerValue(kind)
	if strings.Contains(composer, exportDraftMiddle) {
		t.Fatal("the composer holds the middle of the draft, so an edit to it could drop nothing")
	}
	if strings.Contains(composer, "\t") {
		t.Fatal("the composer kept a tab, so an edit to it could flatten nothing")
	}
}

// assertInlineEditIsRefused spends the two presses on a previewed draft and
// requires the box, the kept copy, and the composer itself to come back
// unchanged, with the refusal named on screen.
//
// The composer staying open is part of it. The draft exists nowhere else in the
// process, so a press that answered by closing the box would take the work as the
// price of protecting it, and one that raised the discard question would ask
// about a decision nobody made.
func assertInlineEditIsRefused(t *testing.T, model *CommentsModel, kind commentMutationKind, draft string) {
	t.Helper()
	before := model.composerValue(kind)
	for _, press := range recoveredDraftEditPresses() {
		deliverToPanel(t, model, press)
		if got := model.composerValue(kind); got != before {
			t.Fatalf("%s edited the read-only preview:\ngot  %q\nwant %q", press.String(), tailOf(got), tailOf(before))
		}
		assertRecoverable(t, model, kind, draft)
		if !model.HasTextInput() {
			t.Fatalf("%s closed the composer over a kept draft", press.String())
		}
		if model.IsConfirmActive() {
			t.Fatalf("%s raised a discard question nobody asked for", press.String())
		}
		assertVisible(t, model, draftPreviewReadOnlyStatus)
	}
}

// assertTheSamePressesEditALosslessDraft is the control for all of it.
//
// A refusal that cannot be told apart from a press the composer never received is
// not a refusal, so the same two keys are spent on a retained draft the box did
// take whole. There they must land — and the box must take back authority over
// what a save writes, which is the other half of what a read-only preview means:
// the box is only read-only while it is a rendering of something bigger.
func assertTheSamePressesEditALosslessDraft(t *testing.T, kind commentMutationKind) {
	t.Helper()
	model, _, ownerPresent := inlineEditFixture(t)
	ownerPresent(false)

	const short = "a draft the box can hold"
	ref := CommentRef{}
	if kind == commentMutationEdit {
		var ok bool
		if ref, ok = model.commentRefAt(0); !ok {
			t.Fatal("the panel drew no comment row")
		}
	}
	if refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, kind, ref, short))); !strings.Contains(refusal, "no longer available") {
		t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
	}
	if model.showsDraftPreview() {
		t.Fatalf("the composer could not hold %q, so it is the wrong control for an editable draft", short)
	}

	before := model.composerValue(kind)
	for _, press := range recoveredDraftEditPresses() {
		deliverToPanel(t, model, press)
	}
	after := model.composerValue(kind)
	if after == before {
		t.Fatalf("the presses left an editable composer at %q, so the read-only case proves nothing", before)
	}
	if got := model.submissionText(kind, after); got != after {
		t.Fatalf("a save from the edited composer would write %q, want the box's own %q", got, after)
	}
}

// assertRecoveryGuidanceMatchesTheSubmission reads what the panel is telling the
// user at the instant they press save, and requires it to be about the text the
// save will write: the notice claims the kept copy is what a save writes, the
// excerpt under it reaches the end of that copy, and the hint says the box above
// is a preview. A submission that had quietly become the box's truncated version
// would make all three of those sentences false at once.
func assertRecoveryGuidanceMatchesTheSubmission(t *testing.T, model *CommentsModel, kind commentMutationKind, draft string) {
	t.Helper()
	view := panelText(t, model)
	notice := strings.Index(view, recoveredDraftNotice)
	if notice < 0 {
		t.Fatalf("the panel never says the box is not showing the whole draft:\n%s", view)
	}
	block := view[notice:]
	if !strings.Contains(block, exportDraftTail) {
		t.Fatalf("the recovery block does not reach the end of the draft:\n%s", block)
	}
	if !strings.Contains(block, recoveredDraftReadOnlyHint) {
		t.Fatalf("the recovery block does not say the box above it is a preview:\n%s", block)
	}
	if got := model.submissionText(kind, model.composerValue(kind)); got != draft {
		t.Fatalf("a save would write %q, want the whole kept draft %q", tailOf(got), tailOf(draft))
	}
}

// assertCommitLevelRefusalKeepsTheDraft retries the save against an owner the
// panel's own preconditions accept and the application layer refuses.
//
// That distinction is the point. Every other refusal in this area is answered
// before the submission is built, so a preview that had become the submission
// would never be exercised; here the whole draft crosses the seam, comes back as
// a failure, and is seated again — which is the one path on which the panel could
// re-seat the composer's copy over the kept one and call it a restore.
func assertCommitLevelRefusalKeepsTheDraft(t *testing.T, model *CommentsModel, kind commentMutationKind, draft string) {
	t.Helper()
	msg, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentMutationFailedMsg)
	if !ok {
		t.Fatal("the save against an archived project was not refused by the application layer")
	}
	if msg.Err == nil || !strings.Contains(msg.Err.Error(), "archived") {
		t.Fatalf("refusal = %v, want it to name the archived project", msg.Err)
	}
	assertRecoverable(t, model, kind, draft)
	assertPreviewIsShortOfTheDraft(t, model, kind)
	assertVisible(t, model, "archived")
}

// TestProjectCommentRecoveredDraftInlineEditPreservesUnseenText holds the
// authority half of the no-lost-drafts contract.
//
// A recovered draft is seated into a quick-entry textarea that stops at
// CommentCharLimit and rewrites tabs as four spaces, so the box below the
// recovery notice is a rendering of the draft rather than the draft: the middle
// of a long one is on nobody's screen and the external editor's temporary file
// was deleted before the result ever arrived. If an ordinary press over the
// visible part handed that box authority over what a save writes, deleting one
// character the user can see would silently discard every character they cannot —
// the recovery would keep the draft right up until the moment its owner touched
// it.
//
// So each case below drives real edits through Update, requires the box and the
// kept copy to come back untouched with the refusal named, and then does the one
// thing that could still lose the text after all of that: a save that is refused
// at commit level, where the whole draft crosses the seam and has to be seated
// again. The owner is then restored and the same save retried, and what the
// workspace ends up holding — read back from storage, middle, tail and tabs
// included — is what the screen promised while the user was looking at it.
func TestProjectCommentRecoveredDraftInlineEditPreservesUnseenText(t *testing.T) {
	draft := exportDraftText()

	t.Run("add_draft", func(t *testing.T) {
		assertTheSamePressesEditALosslessDraft(t, commentMutationAdd)

		model, project, ownerPresent := inlineEditFixture(t)
		ownerPresent(false)

		refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationAdd, CommentRef{}, draft)))
		if !strings.Contains(refusal, "no longer available") {
			t.Fatalf("refusal = %q, want it to name the missing owner", refusal)
		}
		assertRecoverable(t, model, commentMutationAdd, draft)
		assertPreviewIsShortOfTheDraft(t, model, commentMutationAdd)
		assertVisible(t, model, recoveredDraftReadOnlyHint)
		assertInlineEditIsRefused(t, model, commentMutationAdd, draft)

		// The refusal is exactly as true after the presses as before them: nothing
		// the composer does may bring a removed project back or write to one.
		if model.owner.Reload() {
			t.Fatal("the comments panel recreated the removed owner")
		}
		assertStoredComments(t, project, "Older", "Newer")

		// The project comes back, archived. The panel's own preconditions pass —
		// the owner resolves and the list reloads — so the submission is built and
		// committed, and refused by the application layer instead.
		ownerPresent(true)
		project.Archived = true
		assertCommitLevelRefusalKeepsTheDraft(t, model, commentMutationAdd, draft)
		assertStoredComments(t, project, "Older", "Newer")

		project.Archived = false
		assertRecoveryGuidanceMatchesTheSubmission(t, model, commentMutationAdd, draft)
		if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentAddedMsg); !ok {
			t.Fatal("the retry against the restored project did not commit")
		}
		if model.AddMode {
			t.Fatal("the add composer stayed open after the transaction confirmed")
		}
		if _, held := model.retainedDraftText(commentMutationAdd); held {
			t.Fatal("a confirmed commit kept the recovered draft")
		}

		persisted := persistedProjectComments(t, project)
		if len(persisted) != 3 {
			t.Fatalf("stored comments = %d, want the fixture's two and the added one", len(persisted))
		}
		assertPersistedDraft(t, persisted[2].Text, draft)
	})

	t.Run("edit_draft", func(t *testing.T) {
		assertTheSamePressesEditALosslessDraft(t, commentMutationEdit)

		model, project, _ := inlineEditFixture(t)
		model.Cursor = 0
		ref, ok := model.commentRefAt(0)
		if !ok {
			t.Fatal("the panel drew no comment row")
		}

		// The editor was launched from the list, so nothing is on screen when the
		// comment it was opened on is removed by another surface.
		removed := project.Comments[1]
		project.Comments = project.Comments[:1]
		saveExternalChange(t, project)

		refusal := refusalText(t, deliverToPanel(t, model, editorResult(model, commentMutationEdit, ref, draft)))
		if !strings.Contains(refusal, "deleted externally") {
			t.Fatalf("refusal = %q, want it to name the vanished target", refusal)
		}
		assertRecoverable(t, model, commentMutationEdit, draft)
		assertPreviewIsShortOfTheDraft(t, model, commentMutationEdit)
		assertVisible(t, model, recoveredDraftReadOnlyHint)
		assertInlineEditIsRefused(t, model, commentMutationEdit, draft)
		assertStoredComments(t, project, "Older")

		// The comment returns as itself — same identity, same text, so the frozen
		// reference the composer was opened on resolves again — and the project is
		// archived, which moves the refusal to the far side of the seam.
		project.Comments = append(project.Comments, removed)
		saveExternalChange(t, project)
		project.Archived = true
		assertCommitLevelRefusalKeepsTheDraft(t, model, commentMutationEdit, draft)
		assertStoredComments(t, project, "Older", "Newer")

		project.Archived = false
		assertRecoveryGuidanceMatchesTheSubmission(t, model, commentMutationEdit, draft)
		if _, ok := settle(t, model, deliverToPanel(t, model, ctrlS)).(CommentEditedMsg); !ok {
			t.Fatal("the retry against the restored comment did not commit")
		}
		if model.EditMode {
			t.Fatal("the edit composer stayed open after the transaction confirmed")
		}
		if _, held := model.retainedDraftText(commentMutationEdit); held {
			t.Fatal("a confirmed commit kept the recovered draft")
		}

		persisted := persistedProjectComments(t, project)
		if len(persisted) != 2 {
			t.Fatalf("stored comments = %d, want the fixture's two", len(persisted))
		}
		if persisted[1].ID != newerCommentID {
			t.Fatalf("the edit minted a new identity: %q", persisted[1].ID)
		}
		if persisted[0].Text != "Older" {
			t.Fatalf("the edit touched the other comment: %q", persisted[0].Text)
		}
		assertPersistedDraft(t, persisted[1].Text, draft)
	})
}

// assertPersistedDraft names what the workspace ended up holding. Equality is the
// contract, and the three markers and the tab are what a failure message needs to
// say which part went missing: the head survives anything, the tail is what a
// truncating composer drops first, and the middle and the tabs exist nowhere the
// user could have retyped them from.
func assertPersistedDraft(t *testing.T, got, want string) {
	t.Helper()
	for _, marker := range []string{exportDraftHead, exportDraftMiddle, exportDraftTail} {
		if !strings.Contains(got, marker) {
			t.Fatalf("the stored comment is missing %q:\n%s", marker, tailOf(got))
		}
	}
	if !strings.Contains(got, "\t") {
		t.Fatalf("the stored comment kept no tab:\n%s", tailOf(got))
	}
	if got != want {
		t.Fatalf("the stored comment lost %d rune(s) of the draft:\ngot  %q\nwant %q",
			len([]rune(want))-len([]rune(got)), tailOf(got), tailOf(want))
	}
}

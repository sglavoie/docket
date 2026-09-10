package taskdetail

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
)

// commentMutationKind names which of the three commits is outstanding, because
// the success and failure transitions differ by act: an add closes the composer
// and lands the cursor on the new row, an edit closes the editor over the row it
// came from, and a removal only clamps.
type commentMutationKind int

const (
	commentMutationAdd commentMutationKind = iota
	commentMutationEdit
	commentMutationRemove
)

// commentMutation is one submitted-but-unconfirmed comment change. The panel
// holds at most one: a second submission is refused rather than queued, which is
// what makes double submission impossible without a lock.
//
// owner and token together identify the submission. The owner key rejects an
// outcome for a different entity, and the token rejects a stale outcome for this
// same entity — a panel dismissed and reopened on the same task would otherwise
// accept the abandoned commit's result.
type commentMutation struct {
	kind  commentMutationKind
	owner string
	token uint64
	ref   CommentRef
	text  string
	// priorIDs is the identity set the list held when an add was submitted, and
	// is nil for the other two kinds. It is what lets the confirmed add find the
	// row it created without knowing whether this owner prepends or appends.
	priorIDs map[string]bool
}

// commentMutationTokens mints those tokens. It is process-wide rather than
// per-panel so a reopened panel can never reuse a live token.
var commentMutationTokens atomic.Uint64

// commentEditorFinishedMsg carries an external editor's result back into
// Update. The editor runs on a command goroutine, so the callback must not touch
// the model: it returns this value instead, and the panel decides what the
// edited text means where every other state change is already made.
type commentEditorFinishedMsg struct {
	owner string
	kind  commentMutationKind
	ref   CommentRef
	text  string
}

// adaptCommentCommit runs an owner's coordinator command inline and adapts its
// outcome back to this panel's message contract: a rejected transaction becomes
// the correlated CommentMutationFailedMsg, and a committed one resolves to the
// success message stamped with the same correlation plus the transaction ID the
// shell's `u undo` toast is keyed on and the project ID its reload needs.
//
// It sits here rather than beside SubviewCoordinator because the four comment
// messages it stamps are this surface's vocabulary, which the coordinator layer
// sits below and cannot name.
func adaptCommentCommit(owner string, token uint64, commit tea.Cmd, success func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		outcome := commit()
		if msg, ok := outcome.(coordinator.TransactionErrorMsg); ok {
			return CommentMutationFailedMsg{Owner: owner, Token: token, Err: msg.Err}
		}
		transactionID, projectID := commentCommitCorrelation(outcome)
		switch msg := success().(type) {
		case CommentAddedMsg:
			msg.Owner, msg.Token, msg.ProjectID, msg.TransactionID = owner, token, projectID, transactionID
			return msg
		case CommentDeletedMsg:
			msg.Owner, msg.Token, msg.ProjectID, msg.TransactionID = owner, token, projectID, transactionID
			return msg
		case CommentEditedMsg:
			msg.Owner, msg.Token, msg.ProjectID, msg.TransactionID = owner, token, projectID, transactionID
			return msg
		default:
			return msg
		}
	}
}

// commentCommitCorrelation reads the transaction and project identity out of
// whichever committed outcome an owner's coordinator reports.
//
// The two shapes are not interchangeable and both are live here: a task comment
// commits one task and reports TransactionResultMsg, while a project comment
// commits a project and reports the batch result every project-scoped mutation
// uses. Reading only the first would leave the project path with an empty
// project id, and the shell's post-commit reload would then re-read the very
// state the commit replaced.
func commentCommitCorrelation(outcome tea.Msg) (transactionID, projectID string) {
	switch result := outcome.(type) {
	case coordinator.TransactionResultMsg:
		return result.Result.TransactionID, result.Result.ProjectID
	case coordinator.BatchTransactionResultMsg:
		if len(result.TouchedProjectIDs) > 0 {
			projectID = result.TouchedProjectIDs[0]
		}
		return result.TransactionID, projectID
	default:
		return "", ""
	}
}

// beginCommentMutation records the submission the returned command commits. The
// panel keeps the user's draft and selection exactly as they are: nothing about
// a comment changes on screen until the transaction confirms, so a refusal has
// something to restore and a repository-backed model is never edited in place.
func (m *CommentsModel) beginCommentMutation(kind commentMutationKind, ref CommentRef, text string, commit func() tea.Cmd, success func() tea.Msg) tea.Cmd {
	token := commentMutationTokens.Add(1)
	owner := m.ownerKey()
	pending := &commentMutation{kind: kind, owner: owner, token: token, ref: ref, text: text}
	if kind == commentMutationAdd {
		pending.priorIDs = m.currentCommentIDs()
	}
	m.pending = pending
	return adaptCommentCommit(owner, token, commit(), success)
}

// ownsPending reports whether an outcome belongs to the submission this panel is
// still waiting on. Everything else — another owner's commit, a superseded
// token, an outcome that outlived the panel's dismissal — is dropped.
func (m *CommentsModel) ownsPending(owner string, token uint64) bool {
	return m.pending != nil && m.pending.token == token && m.pending.owner == owner && owner == m.ownerKey()
}

// refuseWhilePending is the one duplicate-submission gate. Every mutating entry
// point asks it first, so a second Ctrl+S, a second confirmed delete, and a
// second external editor all bounce off the same state.
func (m *CommentsModel) refuseWhilePending() bool {
	if m.pending == nil {
		return false
	}
	m.SetStatus("Comment change already in progress")
	return true
}

// resyncEntries re-resolves the owner and re-projects its comments before a
// mutation measures an index against them, so an edit or a delete addresses the
// list the workspace holds rather than the one the panel last drew. It reports
// false when the owner itself is gone.
func (m *CommentsModel) resyncEntries() bool {
	if m.owner == nil || !m.owner.Reload() {
		return false
	}
	m.reloadEntries()
	return true
}

// These are the three refusals a comment mutation can earn before it is ever
// submitted. They are values rather than commands because each is reported
// twice: installed on the panel, where the composer paints it beside the draft
// it protected, and handed to the shell, whose status line names it too. A
// refusal only the shell hears is a refusal that arrives beside unsaved text
// with nothing on screen to explain it.
var (
	// errCommentOwnerGone refuses a mutation whose entity disappeared.
	errCommentOwnerGone = errors.New("the commented item is no longer available")
	// errCommentDeletedExternally refuses one whose target left the list.
	errCommentDeletedExternally = errors.New("comment was deleted externally")
	// errCommentChangedExternally refuses one whose target is still there but no
	// longer says what the user read. It is the panel's own copy of the
	// precondition the project use cases enforce, and it is what protects an owner
	// whose comments carry no identity for the application layer to check.
	errCommentChangedExternally = errors.New("comment changed since it was read; reopen it and try again")
)

// refuse reports a refusal on both routes and leaves every draft alone: the
// user's unsaved text is the only copy of it there is, so a refused act closes no
// composer and resets no buffer. Every pre-submission refusal goes through here,
// which is what keeps that promise in one place instead of at nine call sites.
func (m *CommentsModel) refuse(err error) tea.Cmd {
	m.Err = err
	return func() tea.Msg {
		return status.ErrorMsg{Err: err}
	}
}

// saveComment submits the add composer's text. The composer stays open with the
// text in it until the commit confirms.
func (m *CommentsModel) saveComment() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	text := m.submissionText(commentMutationAdd, m.TextInput.Value())
	if text == "" {
		m.Err = errors.New("comment text cannot be empty")
		return nil
	}
	if !m.resyncEntries() {
		return m.refuse(errCommentOwnerGone)
	}
	m.Err = nil
	return m.submitAdd(text)
}

func (m *CommentsModel) submitAdd(text string) tea.Cmd {
	return m.beginCommentMutation(commentMutationAdd, CommentRef{}, text,
		func() tea.Cmd { return m.owner.Add(text) },
		func() tea.Msg { return CommentAddedMsg{} })
}

func (m *CommentsModel) submitEdit(ref CommentRef, text string) tea.Cmd {
	return m.beginCommentMutation(commentMutationEdit, ref, text,
		func() tea.Cmd { return m.owner.Edit(ref, text) },
		func() tea.Msg { return CommentEditedMsg{} })
}

// executeDelete commits the removal the confirmation was opened on, addressing
// the reference that confirmation froze rather than the cursor.
//
// The question and its answer are separated by however long the user looks at
// the prompt, and a commit, an undo, or another machine can land in that gap. So
// the frozen target is re-resolved here and the removal is refused when the
// comment it named is gone or when its text is no longer the text the user was
// shown: a neighbor that slid into the row is a different comment, and deleting
// it would be a destructive change nobody confirmed. Answering the question
// spends it, whatever the outcome.
func (m *CommentsModel) executeDelete() tea.Cmd {
	ref, confirmed := m.spendDeleteConfirmation()
	if !confirmed {
		return nil
	}
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return m.refuse(errCommentOwnerGone)
	}
	target, ok := m.refreshRef(ref)
	if !ok {
		m.clampCursor()
		return m.refuse(errCommentDeletedExternally)
	}
	if m.entries[target.Index].Text != ref.Original {
		m.clampCursor()
		return m.refuse(errCommentChangedExternally)
	}
	m.Err = nil
	return m.beginCommentMutation(commentMutationRemove, target, "",
		func() tea.Cmd { return m.owner.Remove(target) },
		func() tea.Msg { return CommentDeletedMsg{} })
}

// startEditComment opens the editor over the row under the cursor and freezes
// the reference it was opened on, identity and original text included. That
// reference is what the save is measured against, so a list that reorders or
// reloads underneath the open editor cannot redirect the change.
func (m *CommentsModel) startEditComment() tea.Cmd {
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		return nil
	}
	m.EditMode = true
	m.editRef = ref
	m.EditTextInput.SetValue(ref.Original)
	m.EditTextInput.Focus()
	return textarea.Blink
}

// saveEditedComment submits the edit composer's text against the row it was
// opened on. The composer stays open until the commit confirms, so a refusal
// still holds the user's replacement.
//
// A target that vanished is refused the same way, and the composer stays open for
// it too. The comment the user was rewriting is gone, so the buffer is now the
// only copy of their replacement anywhere: closing the editor to report the
// refusal would destroy exactly the work the refusal exists to protect. It stays
// on screen until the user discards it themselves.
func (m *CommentsModel) saveEditedComment() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	text := m.submissionText(commentMutationEdit, m.EditTextInput.Value())
	if text == "" {
		m.Err = errors.New("comment text cannot be empty")
		return nil
	}
	if !m.resyncEntries() {
		return m.refuse(errCommentOwnerGone)
	}
	ref, ok := m.refreshRef(m.editRef)
	if !ok {
		return m.refuse(errCommentDeletedExternally)
	}
	m.Err = nil
	return m.submitEdit(ref, text)
}

// completeCommentMutation applies the success state a confirmed transaction
// earns: the list is re-projected from the owner (never patched in place), the
// composer that produced the change closes, and the cursor follows the act.
func (m *CommentsModel) completeCommentMutation(owner string, token uint64) {
	if !m.ownsPending(owner, token) {
		return
	}
	pending := m.pending
	m.pending = nil
	m.Err = nil
	m.resyncEntries()

	switch pending.kind {
	case commentMutationAdd:
		m.cancelAddMode()
		m.selectAddedComment(pending.priorIDs)
	case commentMutationEdit:
		m.cancelEditMode()
		m.clampCursor()
	case commentMutationRemove:
		if m.Cursor >= len(m.entries) && m.Cursor > 0 {
			m.Cursor--
		}
		m.clampCursor()
	}
}

// selectAddedComment lands the cursor on the row a confirmed add created. It
// finds it by the identity that was not in the list when the add was submitted,
// because an owner is free to present its newest comment first; only an owner
// with no identities at all falls back to the last row, where its appends land.
func (m *CommentsModel) selectAddedComment(prior map[string]bool) {
	for i, entry := range m.entries {
		if entry.ID != "" && !prior[entry.ID] {
			m.Cursor = i
			return
		}
	}
	m.Cursor = max(len(m.entries)-1, 0)
}

// failCommentMutation restores the draft a refused commit was carrying. The
// stored comments never moved, so there is nothing to roll back — only the
// user's text to put back in front of them, in the composer that owns it.
//
// It restores through the same two seating helpers the capture path uses, so the
// submitted text is retained verbatim here too: a commit that carried more than
// the composer can hold must not come back smaller than it went out.
func (m *CommentsModel) failCommentMutation(msg CommentMutationFailedMsg) tea.Cmd {
	if !m.ownsPending(msg.Owner, msg.Token) {
		return nil
	}
	pending := m.pending
	m.pending = nil
	m.Err = msg.Err

	switch pending.kind {
	case commentMutationAdd:
		m.retainAddDraft(pending.text)
		return textarea.Blink
	case commentMutationEdit:
		m.retainEditDraft(pending.ref, pending.text)
		return textarea.Blink
	case commentMutationRemove:
		return nil
	}
	return nil
}

// applyCommentEditorResult turns an external editor's buffer into a submission.
// It runs in Update, so this is where the pending state is recorded and where an
// abandoned editor — the panel moved to another owner while it was open — is
// dropped rather than applied to whatever is mounted now.
//
// The returned text is seated in a composer before the owner or the target is
// resolved, because the temporary file it came from is already deleted by then:
// once this function has the string, nothing else in the process does. A refusal
// after the capture leaves the user looking at their own text; a refusal before
// it would leave them looking at a list with the work they just did nowhere.
func (m *CommentsModel) applyCommentEditorResult(msg commentEditorFinishedMsg) tea.Cmd {
	if msg.owner != m.ownerKey() || m.pending != nil {
		return nil
	}
	switch msg.kind {
	case commentMutationAdd:
		if msg.text == "" {
			m.cancelAddMode()
			return nil
		}
		m.retainAddDraft(msg.text)
		if !m.resyncEntries() {
			return m.refuse(errCommentOwnerGone)
		}
		m.Err = nil
		return m.submitAdd(msg.text)
	case commentMutationEdit:
		if msg.text == msg.ref.Original {
			m.cancelEditMode()
			return nil
		}
		m.retainEditDraft(msg.ref, msg.text)
		if !m.resyncEntries() {
			return m.refuse(errCommentOwnerGone)
		}
		ref, ok := m.refreshRef(msg.ref)
		if !ok {
			return m.refuse(errCommentDeletedExternally)
		}
		m.Err = nil
		return m.submitEdit(ref, msg.text)
	case commentMutationRemove:
		return nil
	}
	return nil
}

// draftRecovery is one comment draft held verbatim beside the composer showing
// it, which is the only way the panel can promise to give it back.
//
// The composer is not that copy and cannot be made into one: its textarea
// rewrites tabs as spaces and stops accepting runes at CommentCharLimit, both of
// which are right for something being typed and wrong for something being
// recovered. An external editor's temporary file is already deleted by the time
// its result reaches the panel, so for that route this field holds the only
// complete copy of the user's work in the process.
type draftRecovery struct {
	// kind names the composer the text belongs to, so an add draft can never be
	// submitted as an edit or recovered under one.
	kind commentMutationKind
	// text is the draft exactly as it arrived — no limit, no sanitization.
	text string
	// seated is what the composer holds once that text has been put into it.
	// While the two still agree, the user has not touched the composer and the
	// verbatim text is what a save writes; once they diverge, the user is editing
	// and the composer is theirs.
	seated string
}

// retainAddDraft and retainEditDraft seat a draft in the composer that owns it
// and record the verbatim text beside it. They are the capture half of that
// recovery: whichever way the submission then goes, the whole text is somewhere
// the View draws, a save can still write, and the discard grammar can reach. A
// confirmed commit closes the composer through completeCommentMutation, so the
// capture costs a successful edit nothing.
func (m *CommentsModel) retainAddDraft(text string) {
	m.EditMode = false
	m.AddMode = true
	m.TextInput.SetValue(text)
	m.TextInput.Focus()
	m.recovered = &draftRecovery{kind: commentMutationAdd, text: text, seated: m.TextInput.Value()}
}

func (m *CommentsModel) retainEditDraft(ref CommentRef, text string) {
	m.AddMode = false
	m.EditMode = true
	m.editRef = ref
	m.EditTextInput.SetValue(text)
	m.EditTextInput.Focus()
	m.recovered = &draftRecovery{kind: commentMutationEdit, text: text, seated: m.EditTextInput.Value()}
}

// retainedDraftText is the verbatim draft being held for one composer.
func (m *CommentsModel) retainedDraftText(kind commentMutationKind) (string, bool) {
	if m.recovered == nil || m.recovered.kind != kind {
		return "", false
	}
	return m.recovered.text, true
}

// heldDraftPreview is a retained draft the composer showing it could not take
// whole, which is the state the composer stops being a buffer in.
//
// The seating is what decides it: a draft that went into the textarea unchanged
// is on screen in full and the box is the user's ordinary editor for it, while
// one the box truncated or flattened is being previewed rather than held. The
// difference matters to every reader of a draft, because in the second case what
// the panel writes, exports, and asks about is the copy beside the box and not
// the box — and the keys that would edit the box are refused for exactly that
// reason.
func (m *CommentsModel) heldDraftPreview(kind commentMutationKind) (string, bool) {
	if m.recovered == nil || m.recovered.kind != kind || m.recovered.text == m.recovered.seated {
		return "", false
	}
	return m.recovered.text, true
}

// showsDraftPreview reports whether the open composer is previewing a kept draft
// rather than holding an editable one. The keymap reads it, because a footer
// offering to insert a newline into a box that refuses the press describes a
// grammar the panel does not have.
func (m *CommentsModel) showsDraftPreview() bool {
	_, held := m.heldDraftPreview(m.openComposerKind())
	return held
}

// draftEditorRouteHint and draftEditorRouteUnavailableHint are what the panel
// adds about Ctrl+E wherever it explains that a composer has become a read-only
// preview. There is one string per state and both are shared by the recovery
// block and the refusal status, because those two are the same claim printed in
// two places: if they could disagree, one of them would still be advertising an
// act the other had already withdrawn.
const (
	draftEditorRouteHint            = " Ctrl+E opens it in an external editor."
	draftEditorRouteUnavailableHint = " Ctrl+E needs a target this draft no longer has."
)

// draftEditorGuidance is that sentence for one composer, and it is the whole
// reason the availability below is asked at all.
//
// A retained draft is the state in which the panel has the least to offer and
// says the most: the box under it cannot be edited, the save has already been
// refused, and the two keys named are all that is left. Naming a key that
// answers with a refusal spends the user's one remaining move on nothing, so the
// editor half is written from what that key would actually do and the export
// half — which needs nothing from the workspace — is stated unconditionally
// beside it.
func (m *CommentsModel) draftEditorGuidance(kind commentMutationKind) string {
	if m.externalDraftEditorAvailable(kind) {
		return draftEditorRouteHint
	}
	return draftEditorRouteUnavailableHint
}

// externalDraftEditorAvailable reports whether Ctrl+E over the composer showing
// a kept draft could still reach an editor.
//
// It asks exactly what that key's own route asks and nothing more. The add
// editor needs an owner to head its buffer with; the edit editor needs both an
// owner the workspace still holds and the comment the composer was opened on,
// and refuses before creating a buffer when either is gone. A retained draft
// outlives both — that is the case the retention exists for — so this is the one
// question the guidance beside it cannot assume the answer to.
//
// The list it measures the comment against is the panel's own, which is the last
// one it could read: the guidance speaks for what is on screen, and a row the
// user can still see is a row Ctrl+E will still resolve.
func (m *CommentsModel) externalDraftEditorAvailable(kind commentMutationKind) bool {
	if m.owner == nil {
		return false
	}
	if kind == commentMutationAdd {
		return true
	}
	return m.owner.Available() && m.indexOfRef(m.editRef) >= 0
}

// openComposerKind names the composer a press or an act is addressed to. Only
// one is ever open, and Update routes edit before add, so the two answers cannot
// disagree.
func (m *CommentsModel) openComposerKind() commentMutationKind {
	if m.EditMode {
		return commentMutationEdit
	}
	return commentMutationAdd
}

// forgetRetainedDraft drops the held copy when its composer closes. Only an
// explicit discard, a confirmed commit, and leaving the panel reach it, which is
// what makes the retention outlast every refusal and every reload in between.
func (m *CommentsModel) forgetRetainedDraft(kind commentMutationKind) {
	if m.recovered != nil && m.recovered.kind == kind {
		m.recovered = nil
	}
}

// submissionText is the text a save from one composer commits.
//
// Typed input is trimmed the way it always has been. A retained draft is written
// exactly as it arrived instead, because the composer value beside it may be a
// truncated, tab-flattened rendering of it — submitting that on a retry would
// quietly persist the damage the retention exists to prevent.
//
// A preview keeps that authority unconditionally, and the box is read-only for
// as long as it does: the runes past the limit and the interior tabs are on
// nobody's screen, so an ordinary press over the visible part is not consent to
// drop them, and the only routes that end the retention say so first. A draft
// the composer did take whole is a different case — every rune of it is on
// screen and editable, so typing hands authority back to the box, which is then
// showing exactly what the user means.
func (m *CommentsModel) submissionText(kind commentMutationKind, composer string) string {
	text, ok := m.retainedDraftText(kind)
	if !ok {
		return strings.TrimSpace(composer)
	}
	if _, previewed := m.heldDraftPreview(kind); previewed || m.recovered.seated == composer {
		return text
	}
	return strings.TrimSpace(composer)
}

// draftExportStatusPrefix heads the line naming the file a draft was written to.
// It is a constant because the path is the whole point of the act: the panel
// tells the user where their text is, and that sentence is the only thing
// standing between a recovered draft and a user who can open it.
const draftExportStatusPrefix = "Draft written to "

// exportDraft writes the draft the open composer is holding to a file of its own
// and names that file on screen.
//
// It is the one act in this panel that asks the owner for nothing. A draft whose
// comment was deleted or whose project disappeared can be saved nowhere, and the
// composer showing it is a quick-entry box that stops at CommentCharLimit and
// rewrites tabs — so the interior of a long recovered draft is held by the
// process and reachable by nobody, the recovery block above it being an excerpt
// rather than the text. Writing a file is what makes the whole of it the user's
// again while the reason it could not be saved stays exactly where it was: no
// owner is resolved, no mutation is submitted, no comment is stored, and the
// composer, the retained copy, and the refusal on screen are all left alone.
//
// The bytes are the ones a save would commit, through the same submissionText
// authority rule, so the file and the panel can never disagree about what the
// draft is.
func (m *CommentsModel) exportDraft() {
	kind := m.openComposerKind()
	text := m.submissionText(kind, m.composerValue(kind))
	if text == "" {
		m.Err = errors.New("there is no draft text to write")
		return
	}
	path, err := external.CreateTempFileWithContent(text, "tt-comment-draft-", ".md")
	if err != nil {
		m.Err = fmt.Errorf("failed to write the draft: %w", err)
		return
	}
	m.SetStatus(draftExportStatusPrefix + path)
}

// composerValue is what the textarea for one kind is currently showing.
func (m *CommentsModel) composerValue(kind commentMutationKind) string {
	if kind == commentMutationAdd {
		return m.TextInput.Value()
	}
	return m.EditTextInput.Value()
}

// setComposerValue writes one textarea back. It exists for the read-only
// preview, which is enforced by undoing whatever a press did rather than by
// naming the presses that do something: the textarea's own grammar decides which
// keys change text, and a list of them kept here would be a second copy of it.
func (m *CommentsModel) setComposerValue(kind commentMutationKind, text string) {
	if kind == commentMutationAdd {
		m.TextInput.SetValue(text)
		return
	}
	m.EditTextInput.SetValue(text)
}

// lossyRetainedDraft is the retained draft whose composer could not take all of
// it, and is what the recovery block in the View exists to draw. A draft the
// composer holds in full needs no second rendering: it is already on screen,
// editable, and identical.
//
// It reads the seating rather than the box the way every other preview question
// does, so the block and the read-only rule can never disagree about which state
// the composer is in.
func (m *CommentsModel) lossyRetainedDraft() (string, bool) {
	if m.recovered == nil {
		return "", false
	}
	return m.heldDraftPreview(m.recovered.kind)
}

// editInExternalEditor opens the comment under the cursor in the user's editor.
// The buffer's preamble comes from the owner, so what the file says the user is
// editing is whatever the adapter says it is.
func (m *CommentsModel) editInExternalEditor() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return m.refuse(errCommentOwnerGone)
	}
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		m.clampCursor()
		return m.refuse(errCommentDeletedExternally)
	}
	owner := m.ownerKey()
	return m.editCommentWithExternalEditor(m.owner.ExternalEditHeader(ref)+ref.Original, true, func(newText string) tea.Msg {
		return commentEditorFinishedMsg{owner: owner, kind: commentMutationEdit, ref: ref, text: newText}
	})
}

func (m *CommentsModel) addCommentInExternalEditor() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if m.owner == nil {
		return m.refuse(errCommentOwnerGone)
	}
	owner := m.ownerKey()
	content := m.owner.ExternalAddHeader() + m.submissionText(commentMutationAdd, m.TextInput.Value())
	return m.editCommentWithExternalEditor(content, false, func(newText string) tea.Msg {
		return commentEditorFinishedMsg{owner: owner, kind: commentMutationAdd, text: newText}
	})
}

// editCommentInExternalEditorFromEditMode hands the open composer's buffer to the
// external editor. A vanished target refuses the launch and keeps the composer
// open for the same reason a refused save does: the buffer is the only copy of the
// replacement, and there is nowhere else for it to go.
func (m *CommentsModel) editCommentInExternalEditorFromEditMode() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return m.refuse(errCommentOwnerGone)
	}
	ref, ok := m.refreshRef(m.editRef)
	if !ok {
		return m.refuse(errCommentDeletedExternally)
	}
	owner := m.ownerKey()
	content := m.owner.ExternalEditHeader(ref) + m.submissionText(commentMutationEdit, m.EditTextInput.Value())
	return m.editCommentWithExternalEditor(content, true, func(newText string) tea.Msg {
		return commentEditorFinishedMsg{owner: owner, kind: commentMutationEdit, ref: ref, text: newText}
	})
}

func (m *CommentsModel) copyCommentText() {
	text := m.entries[m.Cursor].Text

	if err := external.CopyText(text); err != nil {
		if errors.Is(err, external.ErrClipboardUnsupported) {
			m.SetStatus(err.Error())
			return
		}
		m.Err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return
	}

	m.SetStatus("Comment copied to clipboard")
}

func (m *CommentsModel) CopyAllCommentsText() {
	if len(m.entries) == 0 {
		m.SetStatus("Nothing to copy")
		return
	}
	texts := make([]string, len(m.entries))
	for i, entry := range m.entries {
		texts[i] = entry.Text
	}

	if err := external.CopyText(formatNumberedMarkdownList(texts)); err != nil {
		if errors.Is(err, external.ErrClipboardUnsupported) {
			m.SetStatus(err.Error())
			return
		}
		m.Err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return
	}

	m.SetStatus("All comments copied to clipboard")
}

// formatNumberedMarkdownList renders items as a numbered Markdown list in
// display order (1-based). Multiline item text keeps its continuation lines,
// indented beneath the "N. " marker so the list stays readable when pasted.
func formatNumberedMarkdownList(items []string) string {
	var out []string
	for i, item := range items {
		marker := fmt.Sprintf("%d. ", i+1)
		indent := strings.Repeat(" ", len(marker))
		for j, line := range strings.Split(item, "\n") {
			if j == 0 {
				out = append(out, marker+line)
			} else {
				out = append(out, indent+line)
			}
		}
	}
	return strings.Join(out, "\n")
}

func (m *CommentsModel) editCommentWithExternalEditor(content string, emptyIsError bool, finish func(string) tea.Msg) tea.Cmd {
	tmpPath, err := m.createCommentTempFile(content)
	if err != nil {
		m.Err = err
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return status.ErrorCmd("%s", editorErr)
	}

	command := external.EditorCommand(editor, tmpPath)
	return tea.ExecProcess(command, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return status.ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		newText, readErr := readEditedCommentText(tmpPath)
		if readErr != nil {
			return status.ErrorMsg{Err: readErr}
		}
		if newText == "" && emptyIsError {
			return status.ErrorMsg{Err: errors.New("comment text cannot be empty")}
		}

		return finish(newText)
	})
}

func (m *CommentsModel) createCommentTempFile(content string) (string, error) {
	tmpFile, err := os.CreateTemp("", "tt-comment-*.md")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	tmpPath := tmpFile.Name()
	if _, err := tmpFile.WriteString(content); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpPath, nil
}

func readEditedCommentText(tmpPath string) (string, error) {
	modified, readErr := os.ReadFile(tmpPath)
	_ = os.Remove(tmpPath)
	if readErr != nil {
		return "", fmt.Errorf("failed to read modified file: %w", readErr)
	}

	return util.ParseEditedCommentContent(string(modified)), nil
}

package taskdetail

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
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
}

// commentMutationTokens mints those tokens. It is process-wide rather than
// per-panel so a reopened panel can never reuse a live token.
var commentMutationTokens atomic.Uint64

// adaptCommentCommit runs an owner's coordinator command inline and adapts its
// outcome back to this panel's message contract: a rejected transaction becomes
// the correlated CommentMutationFailedMsg, and a committed one resolves to the
// success message stamped with the same correlation plus the transaction ID the
// shell's `u undo` toast is keyed on, the project ID its reload needs, and — for
// an add — the comment identity the commit minted.
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
		transactionID, projectID, commentID := commentCommitCorrelation(outcome)
		switch msg := success().(type) {
		case CommentAddedMsg:
			msg.Owner, msg.Token, msg.ProjectID, msg.TransactionID = owner, token, projectID, transactionID
			msg.CommentID = commentID
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

// commentCommitCorrelation reads the transaction, project, and comment identity
// out of whichever committed outcome an owner's coordinator reports.
//
// The two shapes are not interchangeable and both are live here: a task comment
// commits one task and reports TransactionResultMsg, while a project comment
// commits a project and reports the batch result every project-scoped mutation
// uses. Reading only the first would leave the project path with an empty
// project id, and the shell's post-commit reload would then re-read the very
// state the commit replaced.
//
// The comment identity comes from the second shape alone, because it is the only
// one whose owner mints one. A task comment reports no identity here for the
// same reason it stores none: it is addressed by position all the way down.
func commentCommitCorrelation(outcome tea.Msg) (transactionID, projectID, commentID string) {
	switch result := outcome.(type) {
	case coordinator.TransactionResultMsg:
		return result.Result.TransactionID, result.Result.ProjectID, ""
	case coordinator.BatchTransactionResultMsg:
		if len(result.TouchedProjectIDs) > 0 {
			projectID = result.TouchedProjectIDs[0]
		}
		return result.TransactionID, projectID, result.CommentID
	default:
		return "", "", ""
	}
}

// beginCommentMutation records the submission the returned command commits. The
// panel keeps the user's draft and selection exactly as they are: nothing about
// a comment changes on screen until the transaction confirms, so a refusal has
// something to restore and a repository-backed model is never edited in place.
func (m *CommentsModel) beginCommentMutation(kind commentMutationKind, ref CommentRef, text string, commit func() tea.Cmd, success func() tea.Msg) tea.Cmd {
	token := commentMutationTokens.Add(1)
	owner := m.ownerKey()
	m.pending = &commentMutation{kind: kind, owner: owner, token: token, ref: ref, text: text}
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
//
// The stored text is seated through the same retention a recovered draft takes,
// because a comment already in the workspace can be longer than CommentCharLimit
// or carry interior tabs: the external editor writes both and the application
// layer stores both, so the quick-entry box the panel opens over one is not
// always able to show it. Handing the box's rendering of such a comment back to
// a save would persist a truncation and a tab flattening as if the user had made
// them, against a precondition still naming the whole original — the one shape in
// which an accepted commit destroys text nobody ever saw. So the verbatim
// original is kept beside the box and the box becomes a read-only preview of it
// for exactly as long as it cannot hold the whole thing.
//
// A comment the box does take whole is untouched by this: the seating matches, so
// nothing is previewed, no recovery block is drawn, and the first keystroke hands
// authority straight back to the box the way it always has.
func (m *CommentsModel) startEditComment() tea.Cmd {
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		return nil
	}
	m.retainEditDraft(ref, ref.Original)
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
//
// committedID is the identity the commit minted, which only an add reports and
// only an owner with identities has one for. The other two acts address a
// comment that already existed, so they carry nothing here.
func (m *CommentsModel) completeCommentMutation(owner string, token uint64, committedID string) {
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
		m.selectAddedComment(committedID)
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

// selectAddedComment lands the cursor on the row a confirmed add created,
// addressing it by the identity the commit reported and by nothing else.
//
// Position, text, and novelty are all wrong here, and novelty is the one that
// looks right: the list is re-projected after the commit, so a comment another
// surface, another machine, or an undo added during the pending window is new to
// this panel too. Selecting the first unseen row would then aim every next
// read, copy, edit, and delete at somebody else's comment on the strength of a
// successful local add. An identity that is no longer in the list — removed
// again in the same window — moves the cursor nowhere rather than guessing at a
// replacement.
//
// An owner that mints no identity reports none, and its appends land on the last
// row, which is where the cursor goes for it.
func (m *CommentsModel) selectAddedComment(committedID string) {
	if committedID == "" {
		m.Cursor = max(len(m.entries)-1, 0)
		return
	}
	for i, entry := range m.entries {
		if entry.ID == committedID {
			m.Cursor = i
			return
		}
	}
	m.clampCursor()
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

func (m *CommentsModel) copyCommentText() {
	text := m.entries[m.Cursor].Text

	if err := external.CopyText(text); err != nil {
		if errors.Is(err, external.ErrClipboardUnsupported) {
			m.SetStatus(err.Error())
			return
		}
		m.Err = fmt.Errorf("couldn't copy to clipboard: %w", err)
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
		m.Err = fmt.Errorf("couldn't copy to clipboard: %w", err)
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

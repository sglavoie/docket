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

// ownerGoneCmd refuses a mutation whose entity disappeared. It leaves every
// draft alone: the user's unsaved text is the only copy of it there is.
func ownerGoneCmd() tea.Cmd {
	return func() tea.Msg {
		return status.ErrorMsg{Err: errors.New("the commented item is no longer available")}
	}
}

func commentDeletedExternallyCmd() tea.Cmd {
	return func() tea.Msg {
		return status.ErrorMsg{Err: errors.New("comment was deleted externally")}
	}
}

// saveComment submits the add composer's text. The composer stays open with the
// text in it until the commit confirms.
func (m *CommentsModel) saveComment() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	text := strings.TrimSpace(m.TextInput.Value())
	if text == "" {
		m.Err = errors.New("comment text cannot be empty")
		return nil
	}
	if !m.resyncEntries() {
		return ownerGoneCmd()
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

func (m *CommentsModel) executeDelete() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return ownerGoneCmd()
	}
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		m.clampCursor()
		return func() tea.Msg { return status.ErrorMsg{Err: errors.New("failed to remove comment")} }
	}
	m.Err = nil
	return m.beginCommentMutation(commentMutationRemove, ref, "",
		func() tea.Cmd { return m.owner.Remove(ref) },
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
func (m *CommentsModel) saveEditedComment() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	text := strings.TrimSpace(m.EditTextInput.Value())
	if text == "" {
		m.Err = errors.New("comment text cannot be empty")
		return nil
	}
	if !m.resyncEntries() {
		return ownerGoneCmd()
	}
	ref, ok := m.refreshRef(m.editRef)
	if !ok {
		m.cancelEditMode()
		return commentDeletedExternallyCmd()
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
func (m *CommentsModel) failCommentMutation(msg CommentMutationFailedMsg) tea.Cmd {
	if !m.ownsPending(msg.Owner, msg.Token) {
		return nil
	}
	pending := m.pending
	m.pending = nil
	m.Err = msg.Err

	switch pending.kind {
	case commentMutationAdd:
		m.EditMode = false
		m.AddMode = true
		m.TextInput.SetValue(pending.text)
		m.TextInput.Focus()
		return textarea.Blink
	case commentMutationEdit:
		m.AddMode = false
		m.EditMode = true
		m.editRef = pending.ref
		m.EditTextInput.SetValue(pending.text)
		m.EditTextInput.Focus()
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
		if !m.resyncEntries() {
			return ownerGoneCmd()
		}
		m.TextInput.SetValue(msg.text)
		m.Err = nil
		return m.submitAdd(msg.text)
	case commentMutationEdit:
		if msg.text == msg.ref.Original {
			m.cancelEditMode()
			return nil
		}
		if !m.resyncEntries() {
			return ownerGoneCmd()
		}
		ref, ok := m.refreshRef(msg.ref)
		if !ok {
			m.cancelEditMode()
			return commentDeletedExternallyCmd()
		}
		m.Err = nil
		return m.submitEdit(ref, msg.text)
	case commentMutationRemove:
		return nil
	}
	return nil
}

// editInExternalEditor opens the comment under the cursor in the user's editor.
// The buffer's preamble comes from the owner, so what the file says the user is
// editing is whatever the adapter says it is.
func (m *CommentsModel) editInExternalEditor() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return ownerGoneCmd()
	}
	ref, ok := m.commentRefAt(m.Cursor)
	if !ok {
		m.clampCursor()
		return commentDeletedExternallyCmd()
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
		return ownerGoneCmd()
	}
	owner := m.ownerKey()
	content := m.owner.ExternalAddHeader() + strings.TrimSpace(m.TextInput.Value())
	return m.editCommentWithExternalEditor(content, false, func(newText string) tea.Msg {
		return commentEditorFinishedMsg{owner: owner, kind: commentMutationAdd, text: newText}
	})
}

func (m *CommentsModel) editCommentInExternalEditorFromEditMode() tea.Cmd {
	if m.refuseWhilePending() {
		return nil
	}
	if !m.resyncEntries() {
		return ownerGoneCmd()
	}
	ref, ok := m.refreshRef(m.editRef)
	if !ok {
		m.cancelEditMode()
		return commentDeletedExternallyCmd()
	}
	owner := m.ownerKey()
	content := m.owner.ExternalEditHeader(ref) + strings.TrimSpace(m.EditTextInput.Value())
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

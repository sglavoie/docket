package taskdetail

import (
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/util"
)

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

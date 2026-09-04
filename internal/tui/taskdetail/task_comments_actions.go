package taskdetail

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/coordinator"
	"github.com/sglavoie/tt/internal/tui/status"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// adaptSubviewCommit runs a subview coordinator command inline and adapts its
// outcome back to this panel's message contract: a TransactionErrorMsg surfaces
// as the shared ErrorMsg, and a committed transaction resolves to success(),
// which runs after the commit so it can re-resolve fresh task pointers. This is
// the single-task analog of the batch transaction adapter — the migrated panels
// keep their existing message vocabulary (and therefore their toasts and
// refresh behavior) while the durable write flows through the application
// layer.
//
// It sits here rather than beside SubviewCoordinator because the three comment
// messages it stamps are this surface's vocabulary, which the coordinator layer
// sits below and cannot name.
func adaptSubviewCommit(commit tea.Cmd, success func() tea.Msg) tea.Cmd {
	return func() tea.Msg {
		outcome := commit()
		if msg, ok := outcome.(coordinator.TransactionErrorMsg); ok {
			return status.ErrorMsg(msg)
		}
		msg := success()
		result, ok := outcome.(coordinator.TransactionResultMsg)
		if !ok {
			return msg
		}
		switch msg := msg.(type) {
		case CommentAddedMsg:
			msg.TransactionID = result.Result.TransactionID
			return msg
		case CommentDeletedMsg:
			msg.TransactionID = result.Result.TransactionID
			return msg
		case CommentEditedMsg:
			msg.TransactionID = result.Result.TransactionID
			return msg
		default:
			return msg
		}
	}
}

// The panel mutates its model synchronously in Update() (avoiding data races
// with View()); adaptSubviewCommit then hands the same mutation to the
// application layer, which re-applies it against a fresh snapshot and commits
// one ChangeSet before the panel's own success message resolves.
func (m *TaskCommentsModel) saveComment() tea.Cmd {
	text := m.TextInput.Value()
	task, err := appendComment(m.project, m.Task.ID, text)
	if err != nil {
		m.Err = err
		return nil
	}

	m.Task = task
	m.cancelAddMode()
	m.Err = nil
	m.Cursor = len(m.Task.Comments) - 1

	return adaptSubviewCommit(
		m.coordinator.AddComment(taskref.For(task, m.project), text),
		func() tea.Msg { return CommentAddedMsg{} },
	)
}

func (m *TaskCommentsModel) executeDelete() tea.Cmd {
	m.refreshTaskReference()
	index := m.Cursor
	if !m.Task.RemoveComment(index) {
		return func() tea.Msg { return status.ErrorMsg{Err: errors.New("failed to remove comment")} }
	}
	if m.Cursor >= len(m.Task.Comments) && m.Cursor > 0 {
		m.Cursor--
	}

	return adaptSubviewCommit(
		m.coordinator.RemoveComment(taskref.For(m.Task, m.project), index),
		func() tea.Msg { return CommentDeletedMsg{} },
	)
}

func (m *TaskCommentsModel) startEditComment() tea.Cmd {
	comment := m.Task.Comments[m.Cursor]
	m.EditMode = true
	m.editIndex = m.Cursor
	m.EditTextInput.SetValue(comment.Text)
	m.EditTextInput.Focus()
	return textarea.Blink
}

func (m *TaskCommentsModel) saveEditedComment() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	text := strings.TrimSpace(m.EditTextInput.Value())
	if text == "" {
		m.Err = fmt.Errorf("comment text cannot be empty")
		return nil
	}

	m.refreshTaskReference()
	if m.editIndex >= len(m.Task.Comments) {
		m.cancelEditMode()
		return func() tea.Msg { return status.ErrorMsg{Err: fmt.Errorf("comment was deleted externally")} }
	}

	editIndex := m.editIndex
	m.Task.Comments[editIndex].Text = text
	m.cancelEditMode()
	m.Err = nil

	return adaptSubviewCommit(
		m.coordinator.EditComment(taskref.For(m.Task, m.project), editIndex, text),
		func() tea.Msg { return CommentEditedMsg{} },
	)
}

func (m *TaskCommentsModel) editInExternalEditor() tea.Cmd {
	comment := m.Task.Comments[m.Cursor]
	editIndex := m.Cursor
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n%s", editIndex, m.Task.ShortID, m.Task.Title, comment.Text)

	task := m.Task
	project := m.project
	subview := m.coordinator
	return m.editCommentWithExternalEditor(content, true, func(newText string) tea.Msg {
		if newText == comment.Text {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}
		if editIndex >= len(task.Comments) {
			return status.ErrorMsg{Err: fmt.Errorf("comment was deleted externally")}
		}

		task.Comments[editIndex].Text = newText
		return adaptSubviewCommit(
			subview.EditComment(taskref.For(task, project), editIndex, newText),
			func() tea.Msg { return CommentEditedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) addCommentInExternalEditor() tea.Cmd {
	currentText := strings.TrimSpace(m.TextInput.Value())
	content := fmt.Sprintf("# New comment on #%d: %s\n\n<!-- Enter the comment text below. -->\n\n%s", m.Task.ShortID, m.Task.Title, currentText)

	task := m.Task
	project := m.project
	subview := m.coordinator
	m.cancelAddMode()

	return m.editCommentWithExternalEditor(content, false, func(newText string) tea.Msg {
		if newText == "" {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		task.AddComment(newText)
		return adaptSubviewCommit(
			subview.AddComment(taskref.For(task, project), newText),
			func() tea.Msg { return CommentAddedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) editCommentInExternalEditorFromEditMode() tea.Cmd {
	if m.editIndex >= len(m.Task.Comments) {
		m.cancelEditMode()
		return func() tea.Msg { return status.ErrorMsg{Err: fmt.Errorf("comment was deleted externally")} }
	}
	comment := m.Task.Comments[m.editIndex]
	editIndex := m.editIndex
	currentText := strings.TrimSpace(m.EditTextInput.Value())
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n%s", editIndex, m.Task.ShortID, m.Task.Title, currentText)

	task := m.Task
	project := m.project
	subview := m.coordinator
	originalText := comment.Text
	m.cancelEditMode()

	return m.editCommentWithExternalEditor(content, true, func(newText string) tea.Msg {
		if newText == originalText {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}
		if editIndex >= len(task.Comments) {
			return status.ErrorMsg{Err: fmt.Errorf("comment was deleted externally")}
		}

		task.Comments[editIndex].Text = newText
		return adaptSubviewCommit(
			subview.EditComment(taskref.For(task, project), editIndex, newText),
			func() tea.Msg { return CommentEditedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) copyCommentText() {
	comment := m.Task.Comments[m.Cursor]

	if err := external.CopyText(comment.Text); err != nil {
		if errors.Is(err, external.ErrClipboardUnsupported) {
			m.SetStatus(err.Error())
			return
		}
		m.Err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return
	}

	m.SetStatus("Comment copied to clipboard")
}

func (m *TaskCommentsModel) CopyAllCommentsText() {
	if len(m.Task.Comments) == 0 {
		m.SetStatus("Nothing to copy")
		return
	}
	texts := make([]string, len(m.Task.Comments))
	for i, c := range m.Task.Comments {
		texts[i] = c.Text
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

func (m *TaskCommentsModel) editCommentWithExternalEditor(content string, emptyIsError bool, apply func(string) tea.Msg) tea.Cmd {
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
			return status.ErrorMsg{Err: fmt.Errorf("comment text cannot be empty")}
		}

		return apply(newText)
	})
}

func (m *TaskCommentsModel) createCommentTempFile(content string) (string, error) {
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

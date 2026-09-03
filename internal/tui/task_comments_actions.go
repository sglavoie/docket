package tui

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/tui/taskref"
	"github.com/sglavoie/tt/internal/util"
)

// The panel mutates its model synchronously in Update() (avoiding data races
// with View()); adaptSubviewCommit then hands the same mutation to the
// application layer, which re-applies it against a fresh snapshot and commits
// one ChangeSet before the panel's own success message resolves.
func (m *TaskCommentsModel) saveComment() tea.Cmd {
	text := m.textInput.Value()
	task, err := appendComment(m.project, m.task.ID, text)
	if err != nil {
		m.err = err
		return nil
	}

	m.task = task
	m.cancelAddMode()
	m.err = nil
	m.cursor = len(m.task.Comments) - 1

	return adaptSubviewCommit(
		m.coordinator.AddComment(taskref.For(task, m.project), text),
		func() tea.Msg { return CommentAddedMsg{} },
	)
}

func (m *TaskCommentsModel) executeDelete() tea.Cmd {
	m.refreshTaskReference()
	index := m.cursor
	if !m.task.RemoveComment(index) {
		return func() tea.Msg { return ErrorMsg{Err: errors.New("failed to remove comment")} }
	}
	if m.cursor >= len(m.task.Comments) && m.cursor > 0 {
		m.cursor--
	}

	return adaptSubviewCommit(
		m.coordinator.RemoveComment(taskref.For(m.task, m.project), index),
		func() tea.Msg { return CommentDeletedMsg{} },
	)
}

func (m *TaskCommentsModel) startEditComment() tea.Cmd {
	comment := m.task.Comments[m.cursor]
	m.editMode = true
	m.editIndex = m.cursor
	m.editTextInput.SetValue(comment.Text)
	m.editTextInput.Focus()
	return textarea.Blink
}

func (m *TaskCommentsModel) saveEditedComment() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	text := strings.TrimSpace(m.editTextInput.Value())
	if text == "" {
		m.err = fmt.Errorf("comment text cannot be empty")
		return nil
	}

	m.refreshTaskReference()
	if m.editIndex >= len(m.task.Comments) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("comment was deleted externally")} }
	}

	editIndex := m.editIndex
	m.task.Comments[editIndex].Text = text
	m.cancelEditMode()
	m.err = nil

	return adaptSubviewCommit(
		m.coordinator.EditComment(taskref.For(m.task, m.project), editIndex, text),
		func() tea.Msg { return CommentEditedMsg{} },
	)
}

func (m *TaskCommentsModel) editInExternalEditor() tea.Cmd {
	comment := m.task.Comments[m.cursor]
	editIndex := m.cursor
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n%s", editIndex, m.task.ShortID, m.task.Title, comment.Text)

	task := m.task
	project := m.project
	coordinator := m.coordinator
	return m.editCommentWithExternalEditor(content, true, func(newText string) tea.Msg {
		if newText == comment.Text {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}
		if editIndex >= len(task.Comments) {
			return ErrorMsg{Err: fmt.Errorf("comment was deleted externally")}
		}

		task.Comments[editIndex].Text = newText
		return adaptSubviewCommit(
			coordinator.EditComment(taskref.For(task, project), editIndex, newText),
			func() tea.Msg { return CommentEditedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) addCommentInExternalEditor() tea.Cmd {
	currentText := strings.TrimSpace(m.textInput.Value())
	content := fmt.Sprintf("# New comment on #%d: %s\n\n<!-- Enter the comment text below. -->\n\n%s", m.task.ShortID, m.task.Title, currentText)

	task := m.task
	project := m.project
	coordinator := m.coordinator
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
			coordinator.AddComment(taskref.For(task, project), newText),
			func() tea.Msg { return CommentAddedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) editCommentInExternalEditorFromEditMode() tea.Cmd {
	if m.editIndex >= len(m.task.Comments) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("comment was deleted externally")} }
	}
	comment := m.task.Comments[m.editIndex]
	editIndex := m.editIndex
	currentText := strings.TrimSpace(m.editTextInput.Value())
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n%s", editIndex, m.task.ShortID, m.task.Title, currentText)

	task := m.task
	project := m.project
	coordinator := m.coordinator
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
			return ErrorMsg{Err: fmt.Errorf("comment was deleted externally")}
		}

		task.Comments[editIndex].Text = newText
		return adaptSubviewCommit(
			coordinator.EditComment(taskref.For(task, project), editIndex, newText),
			func() tea.Msg { return CommentEditedMsg{} },
		)()
	})
}

func (m *TaskCommentsModel) copyCommentText() {
	comment := m.task.Comments[m.cursor]

	if err := copyTextToClipboard(comment.Text); err != nil {
		if errors.Is(err, errClipboardUnsupported) {
			m.setStatus(err.Error())
			return
		}
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return
	}

	m.setStatus("Comment copied to clipboard")
}

func (m *TaskCommentsModel) copyAllCommentsText() {
	if len(m.task.Comments) == 0 {
		m.setStatus("Nothing to copy")
		return
	}
	texts := make([]string, len(m.task.Comments))
	for i, c := range m.task.Comments {
		texts[i] = c.Text
	}

	if err := copyTextToClipboard(formatNumberedMarkdownList(texts)); err != nil {
		if errors.Is(err, errClipboardUnsupported) {
			m.setStatus(err.Error())
			return
		}
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return
	}

	m.setStatus("All comments copied to clipboard")
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
		m.err = err
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}

	command := external.EditorCommand(editor, tmpPath)
	return tea.ExecProcess(command, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		newText, readErr := readEditedCommentText(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: readErr}
		}
		if newText == "" && emptyIsError {
			return ErrorMsg{Err: fmt.Errorf("comment text cannot be empty")}
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

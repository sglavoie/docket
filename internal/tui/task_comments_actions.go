package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/util"
)

func (m *TaskCommentsModel) saveComment() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	task, err := appendComment(m.project, m.task.ID, m.textInput.Value())
	if err != nil {
		m.err = err
		return nil
	}

	m.task = task
	m.cancelAddMode()
	m.err = nil
	m.cursor = len(m.task.Comments) - 1

	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return CommentAddedMsg{Task: task}
	}
}

func (m *TaskCommentsModel) executeDelete() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	if t := resolveTaskInProject(m.project, m.task.ID); t != nil {
		m.task = t
	}
	if !m.task.RemoveComment(m.cursor) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to remove comment")} }
	}
	if m.cursor >= len(m.task.Comments) && m.cursor > 0 {
		m.cursor--
	}

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return CommentDeletedMsg{Task: task}
	}
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

	if t := resolveTaskInProject(m.project, m.task.ID); t != nil {
		m.task = t
	}
	if m.editIndex >= len(m.task.Comments) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("comment was deleted externally")} }
	}

	m.task.Comments[m.editIndex].Text = text
	m.cancelEditMode()
	m.err = nil

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return CommentEditedMsg{Task: task}
	}
}

func (m *TaskCommentsModel) editInExternalEditor() tea.Cmd {
	comment := m.task.Comments[m.cursor]
	editIndex := m.cursor
	content := fmt.Sprintf("# Comment %d on #%d: %s\n\n<!-- Edit the comment text below. The timestamp will be preserved. -->\n\n%s", editIndex, m.task.ShortID, m.task.Title, comment.Text)

	task := m.task
	project := m.project
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
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return CommentEditedMsg{Task: task}
	})
}

func (m *TaskCommentsModel) addCommentInExternalEditor() tea.Cmd {
	currentText := strings.TrimSpace(m.textInput.Value())
	content := fmt.Sprintf("# New comment on #%d: %s\n\n<!-- Enter the comment text below. -->\n\n%s", m.task.ShortID, m.task.Title, currentText)

	task := m.task
	project := m.project
	m.cancelAddMode()

	return m.editCommentWithExternalEditor(content, false, func(newText string) tea.Msg {
		if newText == "" {
			return nil
		}
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		task.AddComment(newText)
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return CommentAddedMsg{Task: task}
	})
}

func (m *TaskCommentsModel) toggleResolved() tea.Cmd {
	// Mutate model state synchronously in Update() to avoid data races with View().
	if t := resolveTaskInProject(m.project, m.task.ID); t != nil {
		m.task = t
	}
	if !m.task.ToggleCommentResolved(m.cursor) {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("failed to toggle comment resolved state")} }
	}

	task := m.task
	project := m.project
	return func() tea.Msg {
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}
		return CommentResolvedMsg{Task: task}
	}
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
		if err := saveProjectNoReload(project); err != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to save project: %w", err)}
		}

		return CommentEditedMsg{Task: task}
	})
}

func (m *TaskCommentsModel) copyCommentText() tea.Cmd {
	comment := m.task.Comments[m.cursor]

	var clipCmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		clipCmd = exec.Command("pbcopy")
	case "linux":
		clipCmd = exec.Command("xclip", "-selection", "clipboard")
	case "windows":
		clipCmd = exec.Command("clip")
	default:
		m.setStatus("Clipboard not supported on this OS")
		return nil
	}

	clipCmd.Stdin = strings.NewReader(comment.Text)
	if err := clipCmd.Run(); err != nil {
		m.err = fmt.Errorf("failed to copy to clipboard: %w", err)
		return nil
	}

	m.setStatus("Comment copied to clipboard")
	return nil
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

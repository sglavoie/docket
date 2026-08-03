package tui

import (
	"fmt"
	"os"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/external"
	"github.com/sglavoie/tt/internal/models"
	"github.com/sglavoie/tt/internal/util"
)

func (m *TaskChecklistModel) editInExternalEditor() tea.Cmd {
	item := m.task.Checklist[m.cursor]
	editIndex := m.cursor

	// Create temp file content
	content := fmt.Sprintf("# Checklist Item %d for #%d: %s\n\n", editIndex, m.task.ShortID, m.task.Title)
	content += "<!-- Edit the item text below. -->\n\n"
	content += item.Text

	// Create temp file
	tmpPath, tmpErr := external.CreateTempFileWithContent(content, "tt-checklist-", ".md")
	if tmpErr != nil {
		m.err = tmpErr
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	task := m.task
	project := m.project
	coordinator := m.coordinator

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return ErrorMsg{Err: fmt.Errorf("item text cannot be empty")}
		}

		// Check if changed
		if newText == item.Text {
			return nil
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Bounds check after re-resolve (checklist may have changed)
		if editIndex < 0 || editIndex >= len(task.Checklist) {
			return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")}
		}

		// Update item (preserve done state)
		task.Checklist[editIndex].Text = newText

		return commitTaskItemMutation(project, task, coordinator.EditChecklistItem(taskIdentityFor(task, project), editIndex, newText), func(fresh *models.Task) tea.Msg {
			return ChecklistItemEditedMsg{}
		})()
	})
}

func (m *TaskChecklistModel) addItemInExternalEditor() tea.Cmd {
	// Get current textarea content
	currentText := strings.TrimSpace(m.textInput.Value())

	// Create temp file content
	content := fmt.Sprintf("# New checklist item for #%d: %s\n\n", m.task.ShortID, m.task.Title)
	content += "<!-- Enter the item text below. -->\n\n"
	content += currentText

	// Create temp file
	tmpPath, tmpErr := external.CreateTempFileWithContent(content, "tt-checklist-", ".md")
	if tmpErr != nil {
		m.err = tmpErr
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	task := m.task
	project := m.project
	coordinator := m.coordinator

	// Reset add mode before launching editor
	m.addMode = false
	m.textInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return nil
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Add the new item
		task.Checklist = append(task.Checklist, models.ChecklistItem{
			Text: newText,
			Done: false,
		})

		return commitTaskItemMutation(project, task, coordinator.AddChecklistItem(taskIdentityFor(task, project), newText), func(fresh *models.Task) tea.Msg {
			return ChecklistItemAddedMsg{}
		})()
	})
}

func (m *TaskChecklistModel) editItemInExternalEditorFromEditMode() tea.Cmd {
	if m.editIndex >= len(m.task.Checklist) {
		m.cancelEditMode()
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")} }
	}
	item := m.task.Checklist[m.editIndex]
	editIndex := m.editIndex

	// Get current textarea content (may have been modified)
	currentText := strings.TrimSpace(m.editTextInput.Value())

	// Create temp file content
	content := fmt.Sprintf("# Checklist Item %d for #%d: %s\n\n", editIndex, m.task.ShortID, m.task.Title)
	content += "<!-- Edit the item text below. -->\n\n"
	content += currentText

	// Create temp file
	tmpPath, tmpErr := external.CreateTempFileWithContent(content, "tt-checklist-", ".md")
	if tmpErr != nil {
		m.err = tmpErr
		return nil
	}

	editor, editorErr := external.ResolveEditor(m.editor)
	if editorErr != nil {
		_ = os.Remove(tmpPath)
		return errorCmd("%s", editorErr)
	}
	c := external.EditorCommand(editor, tmpPath)

	task := m.task
	project := m.project
	coordinator := m.coordinator
	originalText := item.Text

	// Reset edit mode before launching editor
	m.editMode = false
	m.editTextInput.Reset()

	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			_ = os.Remove(tmpPath)
			return ErrorMsg{Err: fmt.Errorf("editor failed: %w", err)}
		}

		// Read modified content
		modified, readErr := os.ReadFile(tmpPath)
		_ = os.Remove(tmpPath)
		if readErr != nil {
			return ErrorMsg{Err: fmt.Errorf("failed to read modified file: %w", readErr)}
		}

		// Parse the edited content
		newText := util.ParseEditedCommentContent(string(modified))
		if newText == "" {
			return ErrorMsg{Err: fmt.Errorf("item text cannot be empty")}
		}

		// Check if changed from original
		if newText == originalText {
			return nil
		}

		// Re-resolve task in case an async save replaced project.Tasks
		if t := project.GetTaskByID(task.ID); t != nil {
			task = t
		}

		// Bounds check after re-resolve (checklist may have changed)
		if editIndex < 0 || editIndex >= len(task.Checklist) {
			return ErrorMsg{Err: fmt.Errorf("checklist item no longer exists")}
		}

		// Update item (preserve done state)
		task.Checklist[editIndex].Text = newText

		return commitTaskItemMutation(project, task, coordinator.EditChecklistItem(taskIdentityFor(task, project), editIndex, newText), func(fresh *models.Task) tea.Msg {
			return ChecklistItemEditedMsg{}
		})()
	})
}

package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskCommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	if m.expandedView {
		return m.updateExpandedView(msg)
	}
	if m.confirm.IsActive() {
		return m.updateConfirmMode(msg)
	}
	if m.editMode {
		return m.updateEditMode(msg)
	}
	if m.addMode {
		return m.updateAddMode(msg)
	}

	return m.updateNormalMode(msg)
}

func (m *TaskCommentsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc", "enter":
			m.expandedView = false
		case "j", "down", "k", "up":
			return m, nil
		}
	}

	return m, nil
}

func (m *TaskCommentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		action := m.confirm.Action()
		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				switch action {
				case "delete":
					return m, m.executeDelete()
				case "discard-add":
					m.cancelAddMode()
					return m, nil
				case "discard-edit":
					m.cancelEditMode()
					return m, nil
				}
			}
			m.ClearError()
			return m, nil
		}
	}

	return m, nil
}

func (m *TaskCommentsModel) updateEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if m.hasUnsavedEditChanges() {
				m.confirm.Show("discard-edit", "Discard changes?")
				return m, nil
			}
			m.cancelEditMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveEditedComment()
		case "ctrl+e":
			return m, m.editCommentInExternalEditorFromEditMode()
		case "shift+enter":
			m.editTextInput.InsertString("\n")
			return m, nil
		}
	}

	m.editTextInput, cmd = m.editTextInput.Update(msg)
	return m, cmd
}

func (m *TaskCommentsModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textInput.Value()) != "" {
				m.confirm.Show("discard-add", "Discard comment?")
				return m, nil
			}
			m.cancelAddMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveComment()
		case "ctrl+e":
			return m, m.addCommentInExternalEditor()
		case "shift+enter":
			m.textInput.InsertString("\n")
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskCommentsModel) updateNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.WindowSizeMsg:
		m.BaseView.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		if m.SkipModifierInPrefix(msg) {
			return m, nil
		}
		if m.pendingPrefix == "g" {
			m.pendingPrefix = ""
			_, cmd = handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash)
			return m, cmd
		}

		return m, m.handleNormalKey(msg.String())
	}

	return m, nil
}

func (m *TaskCommentsModel) handleNormalKey(key string) tea.Cmd {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskCommentsMsg{} }
	case "j", "down":
		m.moveCursorDown()
	case "k", "up":
		m.moveCursorUp()
	case "g":
		m.pendingPrefix = "g"
	case "G":
		m.moveCursorBottom()
	case "a", "n":
		return m.beginAddMode()
	case "e":
		if m.hasSelection() {
			return m.startEditComment()
		}
	case "E":
		if m.hasSelection() {
			return m.editInExternalEditor()
		}
	case "d", "x":
		if m.hasSelection() {
			m.confirm.Show("delete", fmt.Sprintf("Delete comment [%d]?", m.cursor))
		}
	case "r":
		if m.hasSelection() {
			return m.toggleResolved()
		}
	case "y":
		if m.hasSelection() {
			return m.copyCommentText()
		}
	case "enter":
		if m.hasSelection() {
			m.expandedView = true
		}
	}

	return nil
}

func (m *TaskCommentsModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskCommentsModel) hasSelection() bool {
	return len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments)
}

func (m *TaskCommentsModel) hasUnsavedEditChanges() bool {
	return m.editIndex < len(m.task.Comments) && strings.TrimSpace(m.editTextInput.Value()) != m.task.Comments[m.editIndex].Text
}

func (m *TaskCommentsModel) moveCursorDown() {
	if len(m.task.Comments) > 0 && m.cursor < len(m.task.Comments)-1 {
		m.cursor++
	}
}

func (m *TaskCommentsModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *TaskCommentsModel) moveCursorBottom() {
	if len(m.task.Comments) > 0 {
		m.cursor = len(m.task.Comments) - 1
	}
}

func (m *TaskCommentsModel) beginAddMode() tea.Cmd {
	m.addMode = true
	m.textInput.Focus()
	return textarea.Blink
}

func (m *TaskCommentsModel) cancelAddMode() {
	m.addMode = false
	m.textInput.Reset()
}

func (m *TaskCommentsModel) cancelEditMode() {
	m.editMode = false
	m.editTextInput.Reset()
}

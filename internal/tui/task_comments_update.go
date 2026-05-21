package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskCommentsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	return updateTaskItemPanel(msg, taskItemPanelModeHandlers{
		expanded:       m.expandedView,
		confirmActive:  m.confirm.IsActive(),
		editMode:       m.editMode,
		addMode:        m.addMode,
		updateExpanded: m.updateExpandedView,
		updateConfirm:  m.updateConfirmMode,
		updateEdit:     m.updateEditMode,
		updateAdd:      m.updateAddMode,
		updateNormal:   m.updateNormalMode,
	})
}

func (m *TaskCommentsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
	return updateTaskItemPanelExpandedView(m, msg, &m.expandedView)
}

func (m *TaskCommentsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	return updateTaskItemPanelConfirmMode(m, msg, &m.BaseView, &m.InteractiveViewState, taskItemPanelConfirmActions{
		executeDelete: m.executeDelete,
		cancelAdd:     m.cancelAddMode,
		cancelEdit:    m.cancelEditMode,
	})
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
	return updateTaskItemPanelNormalMode(m, msg, &m.BaseView, &m.InteractiveViewState, m.handleNormalKey)
}

func (m *TaskCommentsModel) handleNormalKey(key string) tea.Cmd {
	return m.panelActions().handleNormalKey(&m.InteractiveViewState, key)
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

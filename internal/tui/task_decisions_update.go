package tui

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskDecisionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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

func (m *TaskDecisionsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
	return updateTaskItemPanelExpandedView(m, msg, &m.expandedView)
}

func (m *TaskDecisionsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	return updateTaskItemPanelConfirmMode(m, msg, &m.BaseView, &m.InteractiveViewState, taskItemPanelConfirmActions{
		executeDelete: m.executeDelete,
		cancelAdd:     m.cancelAddMode,
		cancelEdit:    m.cancelEditMode,
	})
}

func (m *TaskDecisionsModel) updateEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
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
			return m, m.saveEditedDecision()
		case "ctrl+e":
			return m, m.editDecisionInExternalEditorFromEditMode()
		case "tab":
			m.editActiveField = 1 - m.editActiveField
			return m, m.focusEditField()
		case "shift+enter":
			if m.editActiveField == 0 {
				m.editTextInput.InsertString("\n")
			} else {
				m.editRationale.InsertString("\n")
			}
			return m, nil
		}
	}

	if m.editActiveField == 0 {
		m.editTextInput, cmd = m.editTextInput.Update(msg)
	} else {
		m.editRationale, cmd = m.editRationale.Update(msg)
	}
	return m, cmd
}

func (m *TaskDecisionsModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textInput.Value()) != "" || strings.TrimSpace(m.rationaleInput.Value()) != "" {
				m.confirm.Show("discard-add", "Discard decision?")
				return m, nil
			}
			m.cancelAddMode()
			return m, nil
		case "ctrl+s":
			return m, m.saveDecision()
		case "ctrl+e":
			return m, m.addDecisionInExternalEditor()
		case "tab":
			m.activeField = 1 - m.activeField
			return m, m.focusAddField()
		case "shift+enter":
			if m.activeField == 0 {
				m.textInput.InsertString("\n")
			} else {
				m.rationaleInput.InsertString("\n")
			}
			return m, nil
		}
	}

	if m.activeField == 0 {
		m.textInput, cmd = m.textInput.Update(msg)
	} else {
		m.rationaleInput, cmd = m.rationaleInput.Update(msg)
	}
	return m, cmd
}

func (m *TaskDecisionsModel) updateNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	return updateTaskItemPanelNormalMode(m, msg, &m.BaseView, &m.InteractiveViewState, m.handleNormalKey)
}

func (m *TaskDecisionsModel) handleNormalKey(key string) tea.Cmd {
	return m.panelActions().handleNormalKey(&m.InteractiveViewState, key)
}

func (m *TaskDecisionsModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskDecisionsModel) hasSelection() bool {
	return len(m.task.Decisions) > 0 && m.cursor < len(m.task.Decisions)
}

func (m *TaskDecisionsModel) hasUnsavedEditChanges() bool {
	if m.editIndex >= len(m.task.Decisions) {
		return false
	}
	d := m.task.Decisions[m.editIndex]
	return strings.TrimSpace(m.editTextInput.Value()) != d.Text ||
		strings.TrimSpace(m.editRationale.Value()) != d.Rationale
}

func (m *TaskDecisionsModel) moveCursorDown() {
	if len(m.task.Decisions) > 0 && m.cursor < len(m.task.Decisions)-1 {
		m.cursor++
	}
}

func (m *TaskDecisionsModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *TaskDecisionsModel) moveCursorBottom() {
	if len(m.task.Decisions) > 0 {
		m.cursor = len(m.task.Decisions) - 1
	}
}

func (m *TaskDecisionsModel) beginAddMode() tea.Cmd {
	m.addMode = true
	m.activeField = 0
	m.textInput.Focus()
	m.rationaleInput.Blur()
	return textarea.Blink
}

func (m *TaskDecisionsModel) cancelAddMode() {
	m.addMode = false
	m.activeField = 0
	m.textInput.Reset()
	m.rationaleInput.Reset()
}

func (m *TaskDecisionsModel) cancelEditMode() {
	m.editMode = false
	m.editActiveField = 0
	m.editTextInput.Reset()
	m.editRationale.Reset()
}

func (m *TaskDecisionsModel) focusAddField() tea.Cmd {
	if m.activeField == 0 {
		m.rationaleInput.Blur()
		m.textInput.Focus()
	} else {
		m.textInput.Blur()
		m.rationaleInput.Focus()
	}
	return textarea.Blink
}

func (m *TaskDecisionsModel) focusEditField() tea.Cmd {
	if m.editActiveField == 0 {
		m.editRationale.Blur()
		m.editTextInput.Focus()
	} else {
		m.editTextInput.Blur()
		m.editRationale.Focus()
	}
	return textarea.Blink
}

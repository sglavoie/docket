package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
)

func (m *TaskChecklistModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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

func (m *TaskChecklistModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *TaskChecklistModel) updateEditMode(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "enter":
			return m, m.saveEditedItem()
		case "ctrl+e":
			return m, m.editItemInExternalEditorFromEditMode()
		}
	}

	m.editTextInput, cmd = m.editTextInput.Update(msg)
	return m, cmd
}

func (m *TaskChecklistModel) updateAddMode(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			if strings.TrimSpace(m.textInput.Value()) != "" {
				m.confirm.Show("discard-add", "Discard item?")
				return m, nil
			}
			m.cancelAddMode()
			return m, nil
		case "enter":
			return m, m.saveItem()
		case "ctrl+e":
			return m, m.addItemInExternalEditor()
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m *TaskChecklistModel) updateNormalMode(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *TaskChecklistModel) handleNormalKey(key string) tea.Cmd {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskChecklistMsg{} }
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
			return m.startEditItem()
		}
	case "E":
		if m.hasSelection() {
			return m.editInExternalEditor()
		}
	case "d", "x":
		if m.hasSelection() {
			m.confirm.Show("delete", fmt.Sprintf("Delete item [%d]?", m.cursor))
		}
	case "space", "enter":
		if m.hasSelection() {
			return m.toggleItem()
		}
	case "y":
		if m.hasSelection() {
			return m.copyItemText()
		}
	case "Y":
		return m.copyAllItemsText()
	case "J":
		if m.cursor < len(m.task.Checklist)-1 {
			return m.reorderItem(m.cursor, m.cursor+1)
		}
	case "K":
		if m.cursor > 0 {
			return m.reorderItem(m.cursor, m.cursor-1)
		}
	}

	return nil
}

func (m *TaskChecklistModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

func (m *TaskChecklistModel) hasSelection() bool {
	return len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist)
}

func (m *TaskChecklistModel) hasUnsavedEditChanges() bool {
	return m.editIndex < len(m.task.Checklist) && strings.TrimSpace(m.editTextInput.Value()) != m.task.Checklist[m.editIndex].Text
}

func (m *TaskChecklistModel) moveCursorDown() {
	if len(m.task.Checklist) > 0 && m.cursor < len(m.task.Checklist)-1 {
		m.cursor++
	}
}

func (m *TaskChecklistModel) moveCursorUp() {
	if m.cursor > 0 {
		m.cursor--
	}
}

func (m *TaskChecklistModel) moveCursorBottom() {
	if len(m.task.Checklist) > 0 {
		m.cursor = len(m.task.Checklist) - 1
	}
}

func (m *TaskChecklistModel) beginAddMode() tea.Cmd {
	m.ClearError()
	m.addMode = true
	m.textInput.Focus()
	return textarea.Blink
}

func (m *TaskChecklistModel) cancelAddMode() {
	m.addMode = false
	m.textInput.Reset()
	m.ClearError()
}

func (m *TaskChecklistModel) cancelEditMode() {
	m.editMode = false
	m.editTextInput.Reset()
	m.ClearError()
}

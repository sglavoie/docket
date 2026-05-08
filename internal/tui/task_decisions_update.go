package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

func (m *TaskDecisionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
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

func (m *TaskDecisionsModel) updateExpandedView(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m *TaskDecisionsModel) updateConfirmMode(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "ctrl+y":
			cats := allDecisionCategories()
			m.editCategoryIdx = (m.editCategoryIdx + 1) % len(cats)
			return m, nil
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
		case "ctrl+y":
			cats := allDecisionCategories()
			m.categoryIndex = (m.categoryIndex + 1) % len(cats)
			return m, nil
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

func (m *TaskDecisionsModel) handleNormalKey(key string) tea.Cmd {
	switch key {
	case "esc":
		return func() tea.Msg { return CloseTaskDecisionsMsg{} }
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
			return m.startEditDecision()
		}
	case "E":
		if m.hasSelection() {
			return m.editInExternalEditor()
		}
	case "d", "x":
		if m.hasSelection() {
			m.confirm.Show("delete", fmt.Sprintf("Delete decision [%d]?", m.cursor))
		}
	case "r":
		if m.hasSelection() {
			return m.toggleResolved()
		}
	case "y":
		if m.hasSelection() {
			return m.copyDecisionText()
		}
	case "enter":
		if m.hasSelection() {
			m.expandedView = true
		}
	}

	return nil
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
	m.categoryIndex = 0
	m.textInput.Focus()
	m.rationaleInput.Blur()
	return textarea.Blink
}

func (m *TaskDecisionsModel) cancelAddMode() {
	m.addMode = false
	m.activeField = 0
	m.categoryIndex = 0
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

func (m *TaskDecisionsModel) selectedCategory() models.DecisionCategory {
	cats := allDecisionCategories()
	if m.categoryIndex >= 0 && m.categoryIndex < len(cats) {
		return cats[m.categoryIndex]
	}
	return cats[0]
}

func (m *TaskDecisionsModel) selectedEditCategory() models.DecisionCategory {
	cats := allDecisionCategories()
	if m.editCategoryIdx >= 0 && m.editCategoryIdx < len(cats) {
		return cats[m.editCategoryIdx]
	}
	return cats[0]
}

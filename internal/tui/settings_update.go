package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Blur current field
			m.blurCurrentField()

			// Move focus (14 fields total: 0-13)
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % 14
			} else {
				m.focusIndex = (m.focusIndex + 13) % 14 // +13 is -1 mod 14
			}

			// Auto-switch tab to follow focus
			m.activeTab = tabForField(m.focusIndex)

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "[":
			m.blurCurrentField()
			if m.activeTab > 0 {
				m.activeTab--
			} else {
				m.activeTab = len(settingsTabNames) - 1
			}
			m.focusIndex = tabFieldStart[m.activeTab]
			m.focusCurrentField()
			return m, nil

		case "]":
			m.blurCurrentField()
			if m.activeTab < len(settingsTabNames)-1 {
				m.activeTab++
			} else {
				m.activeTab = 0
			}
			m.focusIndex = tabFieldStart[m.activeTab]
			m.focusCurrentField()
			return m, nil

		case " ":
			// Toggle fields
			if m.focusIndex == 9 {
				m.linkedTaskJump = !m.linkedTaskJump
			}
			return m, nil

		case "enter":
			if err := m.validateEditor(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateContext(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateUndoDepth(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateReviewSettings(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateRecentActivityLimit(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateAttachmentEditor(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateCodeEditor(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			if err := m.validateKanbanSettings(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			return m, m.saveSettings()

		case "left", "h":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == -1 {
					// From custom mode, go to last context
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}
			// Cycle through default project views
			if m.focusIndex == 3 {
				if m.defaultViewCursor > 0 {
					m.defaultViewCursor--
				}
			}

		case "right", "l":
			// Cycle through contexts when on context field
			if m.focusIndex == 1 && len(m.contexts) > 0 {
				if m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
					m.contextCursor++
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}
			// Cycle through default project views
			if m.focusIndex == 3 {
				if m.defaultViewCursor < len(defaultProjectViewOptions)-1 {
					m.defaultViewCursor++
				}
			}
		}
	}

	// Update the focused input
	switch m.focusIndex {
	case 0:
		m.editorInput, cmd = m.editorInput.Update(msg)
	case 1:
		oldValue := m.contextInput.Value()
		m.contextInput, cmd = m.contextInput.Update(msg)
		// If user typed something, switch to custom mode
		if m.contextInput.Value() != oldValue {
			m.contextCursor = -1
			// Check if typed value matches an existing context
			for i, ctx := range m.contexts {
				if ctx == m.contextInput.Value() {
					m.contextCursor = i
					break
				}
			}
		}
	case 2:
		m.undoDepthInput, cmd = m.undoDepthInput.Update(msg)
	// case 3: defaultView — enum selector, no text input
	case 4:
		m.dueSoonInput, cmd = m.dueSoonInput.Update(msg)
	case 5:
		m.futureInput, cmd = m.futureInput.Update(msg)
	case 6:
		m.oldDaysInput, cmd = m.oldDaysInput.Update(msg)
	case 7:
		m.completedHoursInput, cmd = m.completedHoursInput.Update(msg)
	case 8:
		m.recentActivityInput, cmd = m.recentActivityInput.Update(msg)
	case 10:
		m.attachmentEditorInput, cmd = m.attachmentEditorInput.Update(msg)
	case 11:
		m.codeEditorInput, cmd = m.codeEditorInput.Update(msg)
	case 12:
		m.kanbanMaxColumnsInput, cmd = m.kanbanMaxColumnsInput.Update(msg)
	case 13:
		m.kanbanMaxSwimlaneInput, cmd = m.kanbanMaxSwimlaneInput.Update(msg)
	}

	// Validate on each keystroke
	m.validationErr = ""
	switch m.focusIndex {
	case 0:
		if err := m.validateEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 1:
		if err := m.validateContext(); err != nil {
			m.validationErr = err.Error()
		}
	case 2:
		if err := m.validateUndoDepth(); err != nil {
			m.validationErr = err.Error()
		}
	// case 3: defaultView — always valid (enum selector)
	case 4, 5, 6, 7:
		if err := m.validateReviewSettings(); err != nil {
			m.validationErr = err.Error()
		}
	case 8:
		if err := m.validateRecentActivityLimit(); err != nil {
			m.validationErr = err.Error()
		}
	case 10:
		if err := m.validateAttachmentEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 11:
		if err := m.validateCodeEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 12, 13:
		if err := m.validateKanbanSettings(); err != nil {
			m.validationErr = err.Error()
		}
	}

	return m, cmd
}

// blurCurrentField removes focus from the current field.
func (m *SettingsModel) blurCurrentField() {
	switch m.focusIndex {
	case 0:
		m.editorInput.Blur()
	case 1:
		m.contextInput.Blur()
	case 2:
		m.undoDepthInput.Blur()
	case 3:
		// Enum selector - no blur needed
	case 4:
		m.dueSoonInput.Blur()
	case 5:
		m.futureInput.Blur()
	case 6:
		m.oldDaysInput.Blur()
	case 7:
		m.completedHoursInput.Blur()
	case 8:
		m.recentActivityInput.Blur()
	case 9:
		// Toggle field - no blur needed
	case 10:
		m.attachmentEditorInput.Blur()
	case 11:
		m.codeEditorInput.Blur()
	case 12:
		m.kanbanMaxColumnsInput.Blur()
	case 13:
		m.kanbanMaxSwimlaneInput.Blur()
	}
}

// focusCurrentField sets focus on the current field.
func (m *SettingsModel) focusCurrentField() {
	switch m.focusIndex {
	case 0:
		m.editorInput.Focus()
	case 1:
		m.contextInput.Focus()
	case 2:
		m.undoDepthInput.Focus()
	case 3:
		// Enum selector - no focus needed
	case 4:
		m.dueSoonInput.Focus()
	case 5:
		m.futureInput.Focus()
	case 6:
		m.oldDaysInput.Focus()
	case 7:
		m.completedHoursInput.Focus()
	case 8:
		m.recentActivityInput.Focus()
	case 9:
		// Toggle field - no focus needed
	case 10:
		m.attachmentEditorInput.Focus()
	case 11:
		m.codeEditorInput.Focus()
	case 12:
		m.kanbanMaxColumnsInput.Focus()
	case 13:
		m.kanbanMaxSwimlaneInput.Focus()
	}
}

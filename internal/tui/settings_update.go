package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// stageValidationClearMsg is sent after a timeout to clear the stage validation error.
type stageValidationClearMsg struct{}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case stageValidationClearMsg:
		m.stageValidationErr = ""
		return m, nil
	case tea.KeyMsg:
		// Stages tab: handle confirm dialog first
		if m.tabs.Active == 7 && m.stageConfirmDelete.IsActive() {
			confirmed, handled := m.stageConfirmDelete.HandleKey(msg.String())
			if handled {
				if confirmed && len(m.stages) > 0 {
					m.stages = append(m.stages[:m.stageCursor], m.stages[m.stageCursor+1:]...)
					if m.stageCursor >= len(m.stages) && m.stageCursor > 0 {
						m.stageCursor--
					}
				}
				return m, nil
			}
		}

		// Stages tab: handle all keys when on tab 7
		if m.tabs.Active == 7 && !m.stageEditMode {
			switch msg.String() {
			case "j", "down":
				if len(m.stages) > 0 && m.stageCursor < len(m.stages)-1 {
					m.stageCursor++
				}
				return m, nil
			case "k", "up":
				if len(m.stages) > 0 && m.stageCursor > 0 {
					m.stageCursor--
				}
				return m, nil
			case "a":
				m.stageEditMode = true
				m.stageAddMode = true
				m.stageInput.SetValue("")
				m.stageInput.Focus()
				return m, nil
			case "d", "delete", "backspace":
				if len(m.stages) > 0 {
					m.stageConfirmDelete.Show("delete", fmt.Sprintf("Delete stage '%s'?", m.stages[m.stageCursor]))
				}
				return m, nil
			case "J":
				if len(m.stages) > 1 && m.stageCursor < len(m.stages)-1 {
					m.stages[m.stageCursor], m.stages[m.stageCursor+1] = m.stages[m.stageCursor+1], m.stages[m.stageCursor]
					m.stageCursor++
				}
				return m, nil
			case "K":
				if len(m.stages) > 1 && m.stageCursor > 0 {
					m.stages[m.stageCursor], m.stages[m.stageCursor-1] = m.stages[m.stageCursor-1], m.stages[m.stageCursor]
					m.stageCursor--
				}
				return m, nil
			case "e":
				if len(m.stages) > 0 {
					m.stageEditMode = true
					m.stageAddMode = false
					m.stageInput.SetValue(m.stages[m.stageCursor])
					m.stageInput.Focus()
				}
				return m, nil
			}
		}

		switch msg.String() {
		case "esc":
			if m.stageEditMode {
				m.stageEditMode = false
				m.stageAddMode = false
				m.stageInput.Blur()
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab", "shift+tab":
			// Don't cycle tabs while editing a stage
			if m.stageEditMode {
				break
			}

			// Blur current field
			m.blurCurrentField()

			// Move focus across all settings fields
			if msg.String() == "tab" {
				m.focusIndex = (m.focusIndex + 1) % settingsFieldCount
			} else {
				m.focusIndex = (m.focusIndex + settingsFieldCount - 1) % settingsFieldCount
			}

			// Auto-switch tab to follow focus
			m.tabs.Active = tabForField(m.focusIndex)

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "[", "]":
			m.blurCurrentField()
			if handleTabCycleKey(msg.String(), &m.tabs, false, func() {
				m.focusIndex = tabFieldStart[m.tabs.Active]
				m.scrollOffset = 0
				m.focusCurrentField()
			}) {
				return m, nil
			}

		case " ":
			// Toggle fields
			if m.focusIndex == 12 {
				m.recentActivityShowAll = !m.recentActivityShowAll
			}
			if m.focusIndex == 13 {
				m.linkedTaskJump = !m.linkedTaskJump
			}
			// Tree column defaults (indices 18-30)
			if m.focusIndex >= 18 && m.focusIndex <= 30 {
				idx := m.focusIndex - 18
				if m.focusIndex == 19 { // Priority: cycle 3-way
					m.treePriorityMode = (m.treePriorityMode + 1) % 3
				} else {
					m.treeColumnDefaults[idx] = !m.treeColumnDefaults[idx]
				}
			}
			// Cross-project sidebar toggle (index 31)
			if m.focusIndex == 31 {
				m.crossOutlinerSidebar = !m.crossOutlinerSidebar
			}
			// Outliner section toggles (indices 32-38)
			if m.focusIndex >= 32 && m.focusIndex <= 38 {
				idx := m.focusIndex - 32
				m.outlinerCollapsed[idx] = !m.outlinerCollapsed[idx]
			}
			// Outliner column defaults (indices 39-51)
			if m.focusIndex >= 39 && m.focusIndex <= 51 {
				if m.focusIndex == 40 { // Priority: cycle 3-way
					m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
				} else {
					idx := m.focusIndex - 39
					m.outlinerColumnDefaults[idx] = !m.outlinerColumnDefaults[idx]
				}
			}
			// Alert toggles (indices 52, 54, 56, 57, 59, 61, 63, 65)
			switch m.focusIndex {
			case 52:
				m.alertDailyReviewEnabled = !m.alertDailyReviewEnabled
			case 54:
				m.alertWeeklyReviewEnabled = !m.alertWeeklyReviewEnabled
			case 56:
				m.alertOverdueEnabled = !m.alertOverdueEnabled
			case 57:
				m.alertDueSoonEnabled = !m.alertDueSoonEnabled
			case 59:
				m.alertStaleEnabled = !m.alertStaleEnabled
			case 61:
				m.alertInboxEnabled = !m.alertInboxEnabled
			case 63:
				m.alertBlockedEnabled = !m.alertBlockedEnabled
			case 65:
				m.alertProjectHealthEnabled = !m.alertProjectHealthEnabled
			}
			return m, nil

		case "enter":
			// Stages tab: submit edit input
			if m.tabs.Active == 7 && m.stageEditMode {
				if err := m.commitStageEdit(); err != nil {
					m.stageValidationErr = err.Error()
					return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
						return stageValidationClearMsg{}
					})
				}
				return m, nil
			}
			if err := m.validateAll(); err != nil {
				m.validationErr = err.Error()
				return m, nil
			}
			return m, m.saveSettings()

		case "ctrl+s":
			// Commit any in-progress stage edit before saving
			if m.stageEditMode {
				if err := m.commitStageEdit(); err != nil {
					m.stageValidationErr = err.Error()
					return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
						return stageValidationClearMsg{}
					})
				}
			}
			if err := m.validateAll(); err != nil {
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
			if m.focusIndex == 6 {
				if m.defaultViewCursor > 0 {
					m.defaultViewCursor--
				}
			}
			// Cycle priority mode left
			switch m.focusIndex {
			case 19:
				if m.treePriorityMode > 0 {
					m.treePriorityMode--
				}
			case 40:
				if m.outlinerPriorityMode > 0 {
					m.outlinerPriorityMode--
				}
			}
			// Weekly review day left
			if m.focusIndex == 55 && m.alertWeeklyReviewDay > 0 {
				m.alertWeeklyReviewDay--
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
			if m.focusIndex == 6 {
				if m.defaultViewCursor < len(defaultProjectViewOptions)-1 {
					m.defaultViewCursor++
				}
			}
			// Cycle priority mode right
			switch m.focusIndex {
			case 19:
				if m.treePriorityMode < 2 {
					m.treePriorityMode++
				}
			case 40:
				if m.outlinerPriorityMode < 2 {
					m.outlinerPriorityMode++
				}
			}
			// Weekly review day right
			if m.focusIndex == 55 && m.alertWeeklyReviewDay < 6 {
				m.alertWeeklyReviewDay++
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
	case 3:
		m.bgColorInput, cmd = m.bgColorInput.Update(msg)
	case 4:
		m.cursorColorInput, cmd = m.cursorColorInput.Update(msg)
	case 5:
		m.cursorTextColorInput, cmd = m.cursorTextColorInput.Update(msg)
	// case 6: defaultView — enum selector, no text input
	case 7:
		m.dueSoonInput, cmd = m.dueSoonInput.Update(msg)
	case 8:
		m.futureInput, cmd = m.futureInput.Update(msg)
	case 9:
		m.oldDaysInput, cmd = m.oldDaysInput.Update(msg)
	case 10:
		m.completedHoursInput, cmd = m.completedHoursInput.Update(msg)
	case 11:
		m.recentActivityInput, cmd = m.recentActivityInput.Update(msg)
	case 14:
		m.maxPinnedInput, cmd = m.maxPinnedInput.Update(msg)
	case 15:
		m.maxRecentTaskVisitsInput, cmd = m.maxRecentTaskVisitsInput.Update(msg)
	case 16:
		m.attachmentEditorInput, cmd = m.attachmentEditorInput.Update(msg)
	case 17:
		m.codeEditorInput, cmd = m.codeEditorInput.Update(msg)
	case 53:
		m.alertDailyReviewHourInput, cmd = m.alertDailyReviewHourInput.Update(msg)
	case 58:
		m.alertDueSoonDaysInput, cmd = m.alertDueSoonDaysInput.Update(msg)
	case 60:
		m.alertStaleDaysInput, cmd = m.alertStaleDaysInput.Update(msg)
	case 62:
		m.alertInboxThresholdInput, cmd = m.alertInboxThresholdInput.Update(msg)
	case 64:
		m.alertBlockedDaysInput, cmd = m.alertBlockedDaysInput.Update(msg)
	case 66:
		m.alertProjectStaleDaysInput, cmd = m.alertProjectStaleDaysInput.Update(msg)
	case 67:
		if m.stageEditMode {
			m.stageInput, cmd = m.stageInput.Update(msg)
		}
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
	case 3:
		if err := m.validateBackgroundColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 4:
		if err := m.validateCursorColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 5:
		if err := m.validateCursorTextColor(); err != nil {
			m.validationErr = err.Error()
		}
	// case 6: defaultView — always valid (enum selector)
	case 7, 8, 9, 10:
		if err := m.validateReviewSettings(); err != nil {
			m.validationErr = err.Error()
		}
	case 11:
		if err := m.validateRecentActivityLimit(); err != nil {
			m.validationErr = err.Error()
		}
	case 14:
		if err := m.validateMaxPinnedBarItems(); err != nil {
			m.validationErr = err.Error()
		}
	case 15:
		if err := m.validateMaxRecentTaskVisits(); err != nil {
			m.validationErr = err.Error()
		}
	case 16:
		if err := m.validateAttachmentEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 17:
		if err := m.validateCodeEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 53, 58, 60, 62, 64, 66:
		if err := m.validateAlertSettings(); err != nil {
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
		m.bgColorInput.Blur()
	case 4:
		m.cursorColorInput.Blur()
	case 5:
		m.cursorTextColorInput.Blur()
	case 6:
		// Enum selector - no blur needed
	case 7:
		m.dueSoonInput.Blur()
	case 8:
		m.futureInput.Blur()
	case 9:
		m.oldDaysInput.Blur()
	case 10:
		m.completedHoursInput.Blur()
	case 11:
		m.recentActivityInput.Blur()
	case 12, 13:
		// Toggle field - no blur needed
	case 14:
		m.maxPinnedInput.Blur()
	case 15:
		m.maxRecentTaskVisitsInput.Blur()
	case 16:
		m.attachmentEditorInput.Blur()
	case 17:
		m.codeEditorInput.Blur()
	case 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30:
		// Tree column toggle fields - no blur needed
	case 53:
		m.alertDailyReviewHourInput.Blur()
	case 58:
		m.alertDueSoonDaysInput.Blur()
	case 60:
		m.alertStaleDaysInput.Blur()
	case 62:
		m.alertInboxThresholdInput.Blur()
	case 64:
		m.alertBlockedDaysInput.Blur()
	case 66:
		m.alertProjectStaleDaysInput.Blur()
	default:
		// Cross-project sidebar toggle (31), outliner section toggles (32-38),
		// outliner column toggles (39-51), alert toggles/enums - no blur needed
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
		m.bgColorInput.Focus()
	case 4:
		m.cursorColorInput.Focus()
	case 5:
		m.cursorTextColorInput.Focus()
	case 6:
		// Enum selector - no focus needed
	case 7:
		m.dueSoonInput.Focus()
	case 8:
		m.futureInput.Focus()
	case 9:
		m.oldDaysInput.Focus()
	case 10:
		m.completedHoursInput.Focus()
	case 11:
		m.recentActivityInput.Focus()
	case 12, 13:
		// Toggle field - no focus needed
	case 14:
		m.maxPinnedInput.Focus()
	case 15:
		m.maxRecentTaskVisitsInput.Focus()
	case 16:
		m.attachmentEditorInput.Focus()
	case 17:
		m.codeEditorInput.Focus()
	case 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30:
		// Tree column toggle fields - no focus needed
	case 53:
		m.alertDailyReviewHourInput.Focus()
	case 58:
		m.alertDueSoonDaysInput.Focus()
	case 60:
		m.alertStaleDaysInput.Focus()
	case 62:
		m.alertInboxThresholdInput.Focus()
	case 64:
		m.alertBlockedDaysInput.Focus()
	case 66:
		m.alertProjectStaleDaysInput.Focus()
	default:
		// Cross-project sidebar toggle (31), outliner section toggles (32-38),
		// outliner column toggles (39-51), alert toggles/enums - no focus needed
	}
}

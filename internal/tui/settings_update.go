package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

// stageValidationClearMsg is sent after a timeout to clear the stage validation error.
type stageValidationClearMsg struct{}

func (m *SettingsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case stageValidationClearMsg:
		m.stageValidationErr = ""
		return m, nil
	case tea.KeyPressMsg:
		// Stages tab: handle confirm dialog first
		if m.tabs.Active == 8 && m.stageConfirmDelete.IsActive() {
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

		// Stages tab: handle all keys when on tab 8
		if m.tabs.Active == 8 && !m.stageEditMode {
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
			if m.subTabRowFocused {
				m.subTabRowFocused = false
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "(", ")":
			if subTabNamesForTab(m.tabs.Active) != nil {
				m.subTabRowFocused = !m.subTabRowFocused
			}
			return m, nil

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

			// Auto-switch tab and sub-tab to follow focus
			m.tabs.Active = tabForField(m.focusIndex)
			m.subTab = subTabForField(m.focusIndex)

			// Focus new field
			m.focusCurrentField()
			return m, nil

		case "[", "]":
			m.blurCurrentField()
			if m.subTabRowFocused && subTabNamesForTab(m.tabs.Active) != nil {
				sections := settingsTabSections[m.tabs.Active]
				delta := 1
				if msg.String() == "[" {
					delta = -1
				}
				CycleTab(&m.subTab, len(sections), delta)
				m.focusIndex = subTabFieldStart(m.tabs.Active, m.subTab)
				m.scrollOffset = 0
				m.focusCurrentField()
				return m, nil
			}
			if handleTabCycleKey(msg.String(), &m.tabs, false, func() {
				m.focusIndex = tabFieldStart[m.tabs.Active]
				m.subTab = 0
				m.subTabRowFocused = false
				m.scrollOffset = 0
				m.focusCurrentField()
			}) {
				return m, nil
			}

		case "space":
			// Toggle fields
			if m.focusIndex == 21 {
				m.recentActivityShowAll = !m.recentActivityShowAll
			}
			if m.focusIndex == 22 {
				m.linkedTaskJump = !m.linkedTaskJump
			}
			// Tree column defaults (indices 28-40)
			if m.focusIndex >= 28 && m.focusIndex <= 40 {
				idx := m.focusIndex - 28
				if m.focusIndex == 29 { // Priority: cycle 3-way
					m.treePriorityMode = (m.treePriorityMode + 1) % 3
				} else {
					m.treeColumnDefaults[idx] = !m.treeColumnDefaults[idx]
				}
			}
			// Cross-project sidebar toggle (index 41)
			if m.focusIndex == 41 {
				m.crossOutlinerSidebar = !m.crossOutlinerSidebar
			}
			// Outliner section toggles (indices 42-48)
			if m.focusIndex >= 42 && m.focusIndex <= 48 {
				idx := m.focusIndex - 42
				m.outlinerCollapsed[idx] = !m.outlinerCollapsed[idx]
			}
			// Outliner column defaults (indices 49-61)
			if m.focusIndex >= 49 && m.focusIndex <= 61 {
				if m.focusIndex == 50 { // Priority: cycle 3-way
					m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
				} else {
					idx := m.focusIndex - 49
					m.outlinerColumnDefaults[idx] = !m.outlinerColumnDefaults[idx]
				}
			}
			// Alert toggles (indices 62, 64, 66, 67, 69, 71, 73, 75)
			switch m.focusIndex {
			case 62:
				m.alertDailyReviewEnabled = !m.alertDailyReviewEnabled
			case 64:
				m.alertWeeklyReviewEnabled = !m.alertWeeklyReviewEnabled
			case 66:
				m.alertOverdueEnabled = !m.alertOverdueEnabled
			case 67:
				m.alertDueSoonEnabled = !m.alertDueSoonEnabled
			case 69:
				m.alertStaleEnabled = !m.alertStaleEnabled
			case 71:
				m.alertInboxEnabled = !m.alertInboxEnabled
			case 73:
				m.alertBlockedEnabled = !m.alertBlockedEnabled
			case 75:
				m.alertProjectHealthEnabled = !m.alertProjectHealthEnabled
			}
			return m, nil

		case "enter":
			// Stages tab: submit edit input
			if m.tabs.Active == 8 && m.stageEditMode {
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
			if m.focusIndex == 3 {
				if m.defaultViewCursor > 0 {
					m.defaultViewCursor--
				}
			}
			// Cycle priority mode left
			switch m.focusIndex {
			case 29:
				if m.treePriorityMode > 0 {
					m.treePriorityMode--
				}
			case 50:
				if m.outlinerPriorityMode > 0 {
					m.outlinerPriorityMode--
				}
			}
			// Weekly review day left
			if m.focusIndex == 65 && m.alertWeeklyReviewDay > 0 {
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
			if m.focusIndex == 3 {
				if m.defaultViewCursor < len(defaultProjectViewOptions)-1 {
					m.defaultViewCursor++
				}
			}
			// Cycle priority mode right
			switch m.focusIndex {
			case 29:
				if m.treePriorityMode < 2 {
					m.treePriorityMode++
				}
			case 50:
				if m.outlinerPriorityMode < 2 {
					m.outlinerPriorityMode++
				}
			}
			// Weekly review day right
			if m.focusIndex == 65 && m.alertWeeklyReviewDay < 6 {
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
	// case 3: defaultView — enum selector, no text input
	case 4:
		m.normalTextInput, cmd = m.normalTextInput.Update(msg)
	case 5:
		m.mutedInput, cmd = m.mutedInput.Update(msg)
	case 6:
		m.primaryInput, cmd = m.primaryInput.Update(msg)
	case 7:
		m.secondaryInput, cmd = m.secondaryInput.Update(msg)
	case 8:
		m.accentInput, cmd = m.accentInput.Update(msg)
	case 9:
		m.successInput, cmd = m.successInput.Update(msg)
	case 10:
		m.warningInput, cmd = m.warningInput.Update(msg)
	case 11:
		m.errorColorInput, cmd = m.errorColorInput.Update(msg)
	case 12:
		m.borderInput, cmd = m.borderInput.Update(msg)
	case 13:
		m.bgColorInput, cmd = m.bgColorInput.Update(msg)
	case 14:
		m.cursorColorInput, cmd = m.cursorColorInput.Update(msg)
	case 15:
		m.cursorTextColorInput, cmd = m.cursorTextColorInput.Update(msg)
	case 16:
		m.dueSoonInput, cmd = m.dueSoonInput.Update(msg)
	case 17:
		m.futureInput, cmd = m.futureInput.Update(msg)
	case 18:
		m.oldDaysInput, cmd = m.oldDaysInput.Update(msg)
	case 19:
		m.completedHoursInput, cmd = m.completedHoursInput.Update(msg)
	case 20:
		m.recentActivityInput, cmd = m.recentActivityInput.Update(msg)
	case 23:
		m.maxPinnedInput, cmd = m.maxPinnedInput.Update(msg)
	case 24:
		m.maxRecentTaskVisitsInput, cmd = m.maxRecentTaskVisitsInput.Update(msg)
	case 25:
		m.popupTimeoutInput, cmd = m.popupTimeoutInput.Update(msg)
	case 26:
		m.attachmentEditorInput, cmd = m.attachmentEditorInput.Update(msg)
	case 27:
		m.codeEditorInput, cmd = m.codeEditorInput.Update(msg)
	case 63:
		m.alertDailyReviewHourInput, cmd = m.alertDailyReviewHourInput.Update(msg)
	case 68:
		m.alertDueSoonDaysInput, cmd = m.alertDueSoonDaysInput.Update(msg)
	case 70:
		m.alertStaleDaysInput, cmd = m.alertStaleDaysInput.Update(msg)
	case 72:
		m.alertInboxThresholdInput, cmd = m.alertInboxThresholdInput.Update(msg)
	case 74:
		m.alertBlockedDaysInput, cmd = m.alertBlockedDaysInput.Update(msg)
	case 76:
		m.alertProjectStaleDaysInput, cmd = m.alertProjectStaleDaysInput.Update(msg)
	case 77:
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
	// case 3: defaultView — always valid (enum selector)
	case 4, 5, 6, 7, 8, 9, 10, 11, 12:
		if err := m.validatePaletteColors(); err != nil {
			m.validationErr = err.Error()
		}
	case 13:
		if err := m.validateBackgroundColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 14:
		if err := m.validateCursorColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 15:
		if err := m.validateCursorTextColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 16, 17, 18, 19:
		if err := m.validateReviewSettings(); err != nil {
			m.validationErr = err.Error()
		}
	case 20:
		if err := m.validateRecentActivityLimit(); err != nil {
			m.validationErr = err.Error()
		}
	case 23:
		if err := m.validateMaxPinnedBarItems(); err != nil {
			m.validationErr = err.Error()
		}
	case 24:
		if err := m.validateMaxRecentTaskVisits(); err != nil {
			m.validationErr = err.Error()
		}
	case 25:
		if err := m.validatePopupTimeout(); err != nil {
			m.validationErr = err.Error()
		}
	case 26:
		if err := m.validateAttachmentEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 27:
		if err := m.validateCodeEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 63, 68, 70, 72, 74, 76:
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
		// Enum selector - no blur needed
	case 4:
		m.normalTextInput.Blur()
	case 5:
		m.mutedInput.Blur()
	case 6:
		m.primaryInput.Blur()
	case 7:
		m.secondaryInput.Blur()
	case 8:
		m.accentInput.Blur()
	case 9:
		m.successInput.Blur()
	case 10:
		m.warningInput.Blur()
	case 11:
		m.errorColorInput.Blur()
	case 12:
		m.borderInput.Blur()
	case 13:
		m.bgColorInput.Blur()
	case 14:
		m.cursorColorInput.Blur()
	case 15:
		m.cursorTextColorInput.Blur()
	case 16:
		m.dueSoonInput.Blur()
	case 17:
		m.futureInput.Blur()
	case 18:
		m.oldDaysInput.Blur()
	case 19:
		m.completedHoursInput.Blur()
	case 20:
		m.recentActivityInput.Blur()
	case 21, 22:
		// Toggle field - no blur needed
	case 23:
		m.maxPinnedInput.Blur()
	case 24:
		m.maxRecentTaskVisitsInput.Blur()
	case 25:
		m.popupTimeoutInput.Blur()
	case 26:
		m.attachmentEditorInput.Blur()
	case 27:
		m.codeEditorInput.Blur()
	case 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40:
		// Tree column toggle fields - no blur needed
	case 63:
		m.alertDailyReviewHourInput.Blur()
	case 68:
		m.alertDueSoonDaysInput.Blur()
	case 70:
		m.alertStaleDaysInput.Blur()
	case 72:
		m.alertInboxThresholdInput.Blur()
	case 74:
		m.alertBlockedDaysInput.Blur()
	case 76:
		m.alertProjectStaleDaysInput.Blur()
	default:
		// Cross-project sidebar toggle (41), outliner section toggles (42-48),
		// outliner column toggles (49-61), alert toggles/enums - no blur needed
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
		m.normalTextInput.Focus()
	case 5:
		m.mutedInput.Focus()
	case 6:
		m.primaryInput.Focus()
	case 7:
		m.secondaryInput.Focus()
	case 8:
		m.accentInput.Focus()
	case 9:
		m.successInput.Focus()
	case 10:
		m.warningInput.Focus()
	case 11:
		m.errorColorInput.Focus()
	case 12:
		m.borderInput.Focus()
	case 13:
		m.bgColorInput.Focus()
	case 14:
		m.cursorColorInput.Focus()
	case 15:
		m.cursorTextColorInput.Focus()
	case 16:
		m.dueSoonInput.Focus()
	case 17:
		m.futureInput.Focus()
	case 18:
		m.oldDaysInput.Focus()
	case 19:
		m.completedHoursInput.Focus()
	case 20:
		m.recentActivityInput.Focus()
	case 21, 22:
		// Toggle field - no focus needed
	case 23:
		m.maxPinnedInput.Focus()
	case 24:
		m.maxRecentTaskVisitsInput.Focus()
	case 25:
		m.popupTimeoutInput.Focus()
	case 26:
		m.attachmentEditorInput.Focus()
	case 27:
		m.codeEditorInput.Focus()
	case 28, 29, 30, 31, 32, 33, 34, 35, 36, 37, 38, 39, 40:
		// Tree column toggle fields - no focus needed
	case 63:
		m.alertDailyReviewHourInput.Focus()
	case 68:
		m.alertDueSoonDaysInput.Focus()
	case 70:
		m.alertStaleDaysInput.Focus()
	case 72:
		m.alertInboxThresholdInput.Focus()
	case 74:
		m.alertBlockedDaysInput.Focus()
	case 76:
		m.alertProjectStaleDaysInput.Focus()
	default:
		// Cross-project sidebar toggle (41), outliner section toggles (42-48),
		// outliner column toggles (49-61), alert toggles/enums - no focus needed
	}
}

package tui

import tea "charm.land/bubbletea/v2"

func (m *SettingsModel) updateFocusedInput(msg tea.Msg) tea.Cmd { //nolint:gocyclo
	var cmd tea.Cmd

	switch m.focusIndex {
	case 0:
		m.editorInput, cmd = m.editorInput.Update(msg)
	case 1:
		oldValue := m.contextInput.Value()
		m.contextInput, cmd = m.contextInput.Update(msg)
		if m.contextInput.Value() != oldValue {
			m.contextCursor = -1
			for i, ctx := range m.contexts {
				if ctx == m.contextInput.Value() {
					m.contextCursor = i
					break
				}
			}
		}
	case 2:
		m.undoDepthInput, cmd = m.undoDepthInput.Update(msg)
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
		m.sectionLimitActiveInput, cmd = m.sectionLimitActiveInput.Update(msg)
	case 27:
		m.sectionLimitNextInput, cmd = m.sectionLimitNextInput.Update(msg)
	case 28:
		m.sectionLimitInboxInput, cmd = m.sectionLimitInboxInput.Update(msg)
	case 29:
		m.sectionLimitStandbyInput, cmd = m.sectionLimitStandbyInput.Update(msg)
	case 30:
		m.sectionLimitWaitingInput, cmd = m.sectionLimitWaitingInput.Update(msg)
	case 31:
		m.sectionLimitSomedayInput, cmd = m.sectionLimitSomedayInput.Update(msg)
	case 32:
		m.sectionLimitReviewInput, cmd = m.sectionLimitReviewInput.Update(msg)
	case 33:
		m.attachmentEditorInput, cmd = m.attachmentEditorInput.Update(msg)
	case 34:
		m.codeEditorInput, cmd = m.codeEditorInput.Update(msg)
	case 70:
		m.alertDailyReviewHourInput, cmd = m.alertDailyReviewHourInput.Update(msg)
	case 75:
		m.alertDueSoonDaysInput, cmd = m.alertDueSoonDaysInput.Update(msg)
	case 77:
		m.alertStaleDaysInput, cmd = m.alertStaleDaysInput.Update(msg)
	case 79:
		m.alertInboxThresholdInput, cmd = m.alertInboxThresholdInput.Update(msg)
	case 81:
		m.alertBlockedDaysInput, cmd = m.alertBlockedDaysInput.Update(msg)
	case 83:
		m.alertProjectStaleDaysInput, cmd = m.alertProjectStaleDaysInput.Update(msg)
	case 84:
		if m.stageEditMode {
			m.stageInput, cmd = m.stageInput.Update(msg)
		}
	}

	return cmd
}

func (m *SettingsModel) validateFocusedField() {
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
	case 26, 27, 28, 29, 30, 31, 32:
		if err := m.validateSectionLimits(); err != nil {
			m.validationErr = err.Error()
		}
	case 33:
		if err := m.validateAttachmentEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 34:
		if err := m.validateCodeEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 70, 75, 77, 79, 81, 83:
		if err := m.validateAlertSettings(); err != nil {
			m.validationErr = err.Error()
		}
	}
}

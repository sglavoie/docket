package tui

import tea "charm.land/bubbletea/v2"

func (m *SettingsModel) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch m.focusIndex {
	case 2:
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
	case 90:
		if m.stageEditMode {
			m.stageInput, cmd = m.stageInput.Update(msg)
		}
	default:
		if input := m.focusedInput(); input != nil {
			*input, cmd = input.Update(msg)
		}
	}

	return cmd
}

func (m *SettingsModel) validateFocusedField() { //nolint:gocyclo
	m.validationErr = ""

	switch m.focusIndex {
	case 0:
		if err := m.validateEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 1:
		if err := m.validateNotesEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 2:
		if err := m.validateContext(); err != nil {
			m.validationErr = err.Error()
		}
	case 3:
		if err := m.validateUndoDepth(); err != nil {
			m.validationErr = err.Error()
		}
	case 6, 7, 8, 9, 10, 11, 12, 13, 14:
		if err := m.validatePaletteColors(); err != nil {
			m.validationErr = err.Error()
		}
	case 15:
		if err := m.validateBackgroundColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 16:
		if err := m.validateCursorColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 17:
		if err := m.validateCursorTextColor(); err != nil {
			m.validationErr = err.Error()
		}
	case 18, 19, 20, 21, 22:
		if err := m.validatePriorityTitleColors(); err != nil {
			m.validationErr = err.Error()
		}
	case 23, 24, 25, 26:
		if err := m.validateReviewSettings(); err != nil {
			m.validationErr = err.Error()
		}
	case 27:
		if err := m.validateRecentActivityLimit(); err != nil {
			m.validationErr = err.Error()
		}
	case 30:
		if err := m.validateMaxPinnedBarItems(); err != nil {
			m.validationErr = err.Error()
		}
	case 31:
		if err := m.validateMaxRecentTaskVisits(); err != nil {
			m.validationErr = err.Error()
		}
	case 32:
		if err := m.validatePopupTimeout(); err != nil {
			m.validationErr = err.Error()
		}
	case 33, 34, 35, 36, 37, 38, 39:
		if err := m.validateSectionLimits(); err != nil {
			m.validationErr = err.Error()
		}
	case 40:
		if err := m.validateAttachmentEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 41:
		if err := m.validateCodeEditor(); err != nil {
			m.validationErr = err.Error()
		}
	case 77, 82, 84, 86, 88, 90:
		if err := m.validateAlertSettings(); err != nil {
			m.validationErr = err.Error()
		}
	}
}

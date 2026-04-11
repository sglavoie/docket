package tui

import tea "charm.land/bubbletea/v2"

func (m *SettingsModel) updateFocusedInput(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	switch m.focusIndex {
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
	case 84:
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

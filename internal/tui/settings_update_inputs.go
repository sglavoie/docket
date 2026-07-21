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
	default:
		if input := m.focusedInput(); input != nil {
			*input, cmd = input.Update(msg)
		}
	}

	return cmd
}

func (m *SettingsModel) validateFocusedField() { //nolint:gocyclo
	m.validationErr = ""
	setErr := func(err error) {
		if err != nil {
			m.validationErr = err.Error()
		}
	}

	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabGeneral:
		switch local {
		case 0:
			setErr(m.validateEditor())
		case 1:
			setErr(m.validateExternalEditor())
		case 2:
			setErr(m.validateContext())
		case 3:
			setErr(m.validateUndoDepth())
		}
	case tabPalette:
		switch {
		case local >= 1 && local <= 9:
			setErr(m.validatePaletteColors())
		case local == 10:
			setErr(m.validateBackgroundColor())
		case local == 11:
			setErr(m.validateCursorColor())
		case local == 12:
			setErr(m.validateCursorTextColor())
		case local >= 13 && local <= 17:
			setErr(m.validatePriorityTitleColors())
		}
	case tabReview:
		if local >= 0 && local <= 4 {
			setErr(m.validateReviewSettings())
		}
	case tabBehavior:
		switch local {
		case 0:
			setErr(m.validateRecentActivityLimit())
		case 3:
			setErr(m.validateMaxPinnedBarItems())
		case 4:
			setErr(m.validateMaxRecentTaskVisits())
		case 5, 6, 7, 8, 9, 10, 11:
			setErr(m.validateSectionLimits())
		case 13:
			setErr(m.validateResumeCardLimit())
		}
	case tabAttachments:
		switch local {
		case 0:
			setErr(m.validateAttachmentEditor())
		case 1:
			setErr(m.validateCodeEditor())
		}
	case tabNotifications:
		switch local {
		case 1, 6, 8, 10, 12, 14:
			setErr(m.validateAlertSettings())
		}
	}
}

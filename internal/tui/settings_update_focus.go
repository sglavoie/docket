package tui

import "charm.land/bubbles/v2/textinput"

// focusedInput returns a pointer to the textinput.Model for the current
// focusIndex, or nil if the index has no associated text input.
func (m *SettingsModel) focusedInput() *textinput.Model {
	switch m.focusIndex {
	case 0:
		return &m.editorInput
	case 1:
		return &m.contextInput
	case 2:
		return &m.undoDepthInput
	case 4:
		return &m.normalTextInput
	case 5:
		return &m.mutedInput
	case 6:
		return &m.primaryInput
	case 7:
		return &m.secondaryInput
	case 8:
		return &m.accentInput
	case 9:
		return &m.successInput
	case 10:
		return &m.warningInput
	case 11:
		return &m.errorColorInput
	case 12:
		return &m.borderInput
	case 13:
		return &m.bgColorInput
	case 14:
		return &m.cursorColorInput
	case 15:
		return &m.cursorTextColorInput
	case 16:
		return &m.dueSoonInput
	case 17:
		return &m.futureInput
	case 18:
		return &m.oldDaysInput
	case 19:
		return &m.completedHoursInput
	case 20:
		return &m.recentActivityInput
	case 23:
		return &m.maxPinnedInput
	case 24:
		return &m.maxRecentTaskVisitsInput
	case 25:
		return &m.popupTimeoutInput
	case 26:
		return &m.sectionLimitActiveInput
	case 27:
		return &m.sectionLimitNextInput
	case 28:
		return &m.sectionLimitInboxInput
	case 29:
		return &m.sectionLimitStandbyInput
	case 30:
		return &m.sectionLimitWaitingInput
	case 31:
		return &m.sectionLimitSomedayInput
	case 32:
		return &m.sectionLimitReviewInput
	case 33:
		return &m.attachmentEditorInput
	case 34:
		return &m.codeEditorInput
	case 70:
		return &m.alertDailyReviewHourInput
	case 75:
		return &m.alertDueSoonDaysInput
	case 77:
		return &m.alertStaleDaysInput
	case 79:
		return &m.alertInboxThresholdInput
	case 81:
		return &m.alertBlockedDaysInput
	case 83:
		return &m.alertProjectStaleDaysInput
	default:
		return nil
	}
}

func (m *SettingsModel) blurCurrentField() {
	if input := m.focusedInput(); input != nil {
		input.Blur()
	}
}

func (m *SettingsModel) focusCurrentField() {
	if input := m.focusedInput(); input != nil {
		input.Focus()
	}
}

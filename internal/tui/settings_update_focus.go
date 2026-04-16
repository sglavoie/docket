package tui

import "charm.land/bubbles/v2/textinput"

// focusedInput returns a pointer to the textinput.Model for the current
// focusIndex, or nil if the index has no associated text input.
func (m *SettingsModel) focusedInput() *textinput.Model { //nolint:gocyclo
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
		return &m.priority1TitleInput
	case 17:
		return &m.priority2TitleInput
	case 18:
		return &m.priority3TitleInput
	case 19:
		return &m.priority4TitleInput
	case 20:
		return &m.priority5TitleInput
	case 21:
		return &m.dueSoonInput
	case 22:
		return &m.futureInput
	case 23:
		return &m.oldDaysInput
	case 24:
		return &m.completedHoursInput
	case 25:
		return &m.recentActivityInput
	case 28:
		return &m.maxPinnedInput
	case 29:
		return &m.maxRecentTaskVisitsInput
	case 30:
		return &m.popupTimeoutInput
	case 31:
		return &m.sectionLimitActiveInput
	case 32:
		return &m.sectionLimitNextInput
	case 33:
		return &m.sectionLimitInboxInput
	case 34:
		return &m.sectionLimitStandbyInput
	case 35:
		return &m.sectionLimitWaitingInput
	case 36:
		return &m.sectionLimitSomedayInput
	case 37:
		return &m.sectionLimitReviewInput
	case 38:
		return &m.attachmentEditorInput
	case 39:
		return &m.codeEditorInput
	case 75:
		return &m.alertDailyReviewHourInput
	case 80:
		return &m.alertDueSoonDaysInput
	case 82:
		return &m.alertStaleDaysInput
	case 84:
		return &m.alertInboxThresholdInput
	case 86:
		return &m.alertBlockedDaysInput
	case 88:
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

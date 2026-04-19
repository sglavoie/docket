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
		return nil // placeholder: theme toggle (Session 3)
	case 5:
		return &m.normalTextInput
	case 6:
		return &m.mutedInput
	case 7:
		return &m.primaryInput
	case 8:
		return &m.secondaryInput
	case 9:
		return &m.accentInput
	case 10:
		return &m.successInput
	case 11:
		return &m.warningInput
	case 12:
		return &m.errorColorInput
	case 13:
		return &m.borderInput
	case 14:
		return &m.bgColorInput
	case 15:
		return &m.cursorColorInput
	case 16:
		return &m.cursorTextColorInput
	case 17:
		return &m.priority1TitleInput
	case 18:
		return &m.priority2TitleInput
	case 19:
		return &m.priority3TitleInput
	case 20:
		return &m.priority4TitleInput
	case 21:
		return &m.priority5TitleInput
	case 22:
		return &m.dueSoonInput
	case 23:
		return &m.futureInput
	case 24:
		return &m.oldDaysInput
	case 25:
		return &m.completedHoursInput
	case 26:
		return &m.recentActivityInput
	case 29:
		return &m.maxPinnedInput
	case 30:
		return &m.maxRecentTaskVisitsInput
	case 31:
		return &m.popupTimeoutInput
	case 32:
		return &m.sectionLimitActiveInput
	case 33:
		return &m.sectionLimitNextInput
	case 34:
		return &m.sectionLimitInboxInput
	case 35:
		return &m.sectionLimitStandbyInput
	case 36:
		return &m.sectionLimitWaitingInput
	case 37:
		return &m.sectionLimitSomedayInput
	case 38:
		return &m.sectionLimitReviewInput
	case 39:
		return &m.attachmentEditorInput
	case 40:
		return &m.codeEditorInput
	case 76:
		return &m.alertDailyReviewHourInput
	case 81:
		return &m.alertDueSoonDaysInput
	case 83:
		return &m.alertStaleDaysInput
	case 85:
		return &m.alertInboxThresholdInput
	case 87:
		return &m.alertBlockedDaysInput
	case 89:
		return &m.alertProjectStaleDaysInput
	case 95:
		return &m.kanbanStartColInput
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

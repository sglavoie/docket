package tui

import "charm.land/bubbles/v2/textinput"

// focusedInput returns a pointer to the textinput.Model for the current
// focusIndex, or nil if the index has no associated text input.
func (m *SettingsModel) focusedInput() *textinput.Model { //nolint:gocyclo
	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabGeneral:
		switch local {
		case 0:
			return &m.editorInput
		case 1:
			return &m.contextInput
		case 2:
			return &m.undoDepthInput
		}
	case tabPalette:
		switch local {
		case 1:
			return &m.normalTextInput
		case 2:
			return &m.mutedInput
		case 3:
			return &m.primaryInput
		case 4:
			return &m.secondaryInput
		case 5:
			return &m.accentInput
		case 6:
			return &m.successInput
		case 7:
			return &m.warningInput
		case 8:
			return &m.errorColorInput
		case 9:
			return &m.borderInput
		case 10:
			return &m.bgColorInput
		case 11:
			return &m.cursorColorInput
		case 12:
			return &m.cursorTextColorInput
		case 13:
			return &m.priority1TitleInput
		case 14:
			return &m.priority2TitleInput
		case 15:
			return &m.priority3TitleInput
		case 16:
			return &m.priority4TitleInput
		case 17:
			return &m.priority5TitleInput
		}
	case tabReview:
		switch local {
		case 0:
			return &m.dueSoonInput
		case 1:
			return &m.futureInput
		case 2:
			return &m.oldDaysInput
		case 3:
			return &m.completedHoursInput
		}
	case tabBehavior:
		switch local {
		case 0:
			return &m.recentActivityInput
		case 3:
			return &m.maxPinnedInput
		case 4:
			return &m.maxRecentTaskVisitsInput
		case 5:
			return &m.popupTimeoutInput
		case 6:
			return &m.sectionLimitActiveInput
		case 7:
			return &m.sectionLimitNextInput
		case 8:
			return &m.sectionLimitInboxInput
		case 9:
			return &m.sectionLimitStandbyInput
		case 10:
			return &m.sectionLimitWaitingInput
		case 11:
			return &m.sectionLimitSomedayInput
		case 12:
			return &m.sectionLimitReviewInput
		}
	case tabAttachments:
		switch local {
		case 0:
			return &m.attachmentEditorInput
		case 1:
			return &m.codeEditorInput
		}
	case tabKanban:
		if local == 1 {
			return &m.kanbanStartColInput
		}
	case tabNotifications:
		switch local {
		case 1:
			return &m.alertDailyReviewHourInput
		case 6:
			return &m.alertDueSoonDaysInput
		case 8:
			return &m.alertStaleDaysInput
		case 10:
			return &m.alertInboxThresholdInput
		case 12:
			return &m.alertBlockedDaysInput
		case 14:
			return &m.alertProjectStaleDaysInput
		}
	}
	return nil
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

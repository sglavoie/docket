package tui

func (m *SettingsModel) blurCurrentField() { //nolint:gocyclo
	switch m.focusIndex {
	case 0:
		m.editorInput.Blur()
	case 1:
		m.contextInput.Blur()
	case 2:
		m.undoDepthInput.Blur()
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
	case 23:
		m.maxPinnedInput.Blur()
	case 24:
		m.maxRecentTaskVisitsInput.Blur()
	case 25:
		m.popupTimeoutInput.Blur()
	case 26:
		m.sectionLimitActiveInput.Blur()
	case 27:
		m.sectionLimitNextInput.Blur()
	case 28:
		m.sectionLimitInboxInput.Blur()
	case 29:
		m.sectionLimitStandbyInput.Blur()
	case 30:
		m.sectionLimitWaitingInput.Blur()
	case 31:
		m.sectionLimitSomedayInput.Blur()
	case 32:
		m.sectionLimitReviewInput.Blur()
	case 33:
		m.attachmentEditorInput.Blur()
	case 34:
		m.codeEditorInput.Blur()
	case 70:
		m.alertDailyReviewHourInput.Blur()
	case 75:
		m.alertDueSoonDaysInput.Blur()
	case 77:
		m.alertStaleDaysInput.Blur()
	case 79:
		m.alertInboxThresholdInput.Blur()
	case 81:
		m.alertBlockedDaysInput.Blur()
	case 83:
		m.alertProjectStaleDaysInput.Blur()
	}
}

func (m *SettingsModel) focusCurrentField() { //nolint:gocyclo
	switch m.focusIndex {
	case 0:
		m.editorInput.Focus()
	case 1:
		m.contextInput.Focus()
	case 2:
		m.undoDepthInput.Focus()
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
	case 23:
		m.maxPinnedInput.Focus()
	case 24:
		m.maxRecentTaskVisitsInput.Focus()
	case 25:
		m.popupTimeoutInput.Focus()
	case 26:
		m.sectionLimitActiveInput.Focus()
	case 27:
		m.sectionLimitNextInput.Focus()
	case 28:
		m.sectionLimitInboxInput.Focus()
	case 29:
		m.sectionLimitStandbyInput.Focus()
	case 30:
		m.sectionLimitWaitingInput.Focus()
	case 31:
		m.sectionLimitSomedayInput.Focus()
	case 32:
		m.sectionLimitReviewInput.Focus()
	case 33:
		m.attachmentEditorInput.Focus()
	case 34:
		m.codeEditorInput.Focus()
	case 70:
		m.alertDailyReviewHourInput.Focus()
	case 75:
		m.alertDueSoonDaysInput.Focus()
	case 77:
		m.alertStaleDaysInput.Focus()
	case 79:
		m.alertInboxThresholdInput.Focus()
	case 81:
		m.alertBlockedDaysInput.Focus()
	case 83:
		m.alertProjectStaleDaysInput.Focus()
	}
}

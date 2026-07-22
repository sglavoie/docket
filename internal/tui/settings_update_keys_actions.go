package tui

func (m *SettingsModel) togglePaletteTheme() {
	snapshot := [20]string{
		m.normalTextInput.Value(),
		m.mutedInput.Value(),
		m.primaryInput.Value(),
		m.secondaryInput.Value(),
		m.accentInput.Value(),
		m.successInput.Value(),
		m.warningInput.Value(),
		m.errorColorInput.Value(),
		m.borderInput.Value(),
		m.bgColorInput.Value(),
		m.cursorColorInput.Value(),
		m.cursorTextColorInput.Value(),
		m.priority1TitleInput.Value(),
		m.priority2TitleInput.Value(),
		m.priority3TitleInput.Value(),
		m.priority4TitleInput.Value(),
		m.priority5TitleInput.Value(),
		m.activeHiddenDimmed[0],
		m.activeHiddenDimmed[1],
		m.activeHiddenDimmed[2],
	}
	m.normalTextInput.SetValue(m.inactiveThemeDraft[0])
	m.mutedInput.SetValue(m.inactiveThemeDraft[1])
	m.primaryInput.SetValue(m.inactiveThemeDraft[2])
	m.secondaryInput.SetValue(m.inactiveThemeDraft[3])
	m.accentInput.SetValue(m.inactiveThemeDraft[4])
	m.successInput.SetValue(m.inactiveThemeDraft[5])
	m.warningInput.SetValue(m.inactiveThemeDraft[6])
	m.errorColorInput.SetValue(m.inactiveThemeDraft[7])
	m.borderInput.SetValue(m.inactiveThemeDraft[8])
	m.bgColorInput.SetValue(m.inactiveThemeDraft[9])
	m.cursorColorInput.SetValue(m.inactiveThemeDraft[10])
	m.cursorTextColorInput.SetValue(m.inactiveThemeDraft[11])
	m.priority1TitleInput.SetValue(m.inactiveThemeDraft[12])
	m.priority2TitleInput.SetValue(m.inactiveThemeDraft[13])
	m.priority3TitleInput.SetValue(m.inactiveThemeDraft[14])
	m.priority4TitleInput.SetValue(m.inactiveThemeDraft[15])
	m.priority5TitleInput.SetValue(m.inactiveThemeDraft[16])
	m.activeHiddenDimmed = [3]string{
		m.inactiveThemeDraft[17],
		m.inactiveThemeDraft[18],
		m.inactiveThemeDraft[19],
	}
	m.inactiveThemeDraft = snapshot
	m.activePresetName, m.inactivePresetName = m.inactivePresetName, m.activePresetName
	if m.activeTheme == "light" {
		m.activeTheme = "dark"
	} else {
		m.activeTheme = "light"
	}
}

//nolint:gocyclo // toggles across many setting categories
func (m *SettingsModel) handleSpaceKey() {
	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabPalette:
		if local == 0 {
			m.togglePaletteTheme()
		}
	case tabBehavior:
		switch local {
		case 1:
			m.recentActivityShowAll = !m.recentActivityShowAll
		case 2:
			m.linkedTaskJump = !m.linkedTaskJump
		case 12:
			m.startViewCursor = (m.startViewCursor + 1) % len(startViewOptions)
		}
	case tabOutliner:
		columnsBase := subTabFieldStart(tabOutliner, 2) - tabFieldStart[tabOutliner]
		columnsEnd := columnsBase + len(AllColumns) - 1
		switch {
		case local == 0:
			m.crossOutlinerSidebar = !m.crossOutlinerSidebar
		case local == 1:
			m.crossOutlinerCycleOnly = !m.crossOutlinerCycleOnly
		case local >= 2 && local <= 8:
			m.outlinerCollapsed[local-2] = !m.outlinerCollapsed[local-2]
		case local == columnsBase+1:
			m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
		case local >= columnsBase && local <= columnsEnd:
			m.outlinerColumnDefaults[local-columnsBase] = !m.outlinerColumnDefaults[local-columnsBase]
		}
	case tabNotifications:
		switch local {
		case 0:
			m.alertDailyReviewEnabled = !m.alertDailyReviewEnabled
		case 2:
			m.alertWeeklyReviewEnabled = !m.alertWeeklyReviewEnabled
		case 4:
			m.alertOverdueEnabled = !m.alertOverdueEnabled
		case 5:
			m.alertDueSoonEnabled = !m.alertDueSoonEnabled
		case 7:
			m.alertStaleEnabled = !m.alertStaleEnabled
		case 9:
			m.alertInboxEnabled = !m.alertInboxEnabled
		case 11:
			m.alertBlockedEnabled = !m.alertBlockedEnabled
		case 13:
			m.alertProjectHealthEnabled = !m.alertProjectHealthEnabled
		}
	case tabAppearance:
		switch local {
		case 0:
			m.zebraStriping = !m.zebraStriping
		case 3:
			m.autoTheme = !m.autoTheme
		case 4:
			m.useEmoji = !m.useEmoji
		}
	}
}

func (m *SettingsModel) handleLeftKey() {
	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabGeneral:
		m.handleLeftGeneralKey(local)
	case tabPalette:
		m.handleLeftPaletteKey(local)
	case tabBehavior:
		m.handleLeftBehaviorKey(local)
	case tabOutliner:
		m.handleLeftOutlinerKey(local)
	case tabNotifications:
		m.handleLeftNotificationsKey(local)
	case tabAppearance:
		m.handleLeftAppearanceKey(local)
	}
}

func (m *SettingsModel) handleRightKey() {
	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabGeneral:
		m.handleRightGeneralKey(local)
	case tabPalette:
		m.handleRightPaletteKey(local)
	case tabBehavior:
		m.handleRightBehaviorKey(local)
	case tabOutliner:
		m.handleRightOutlinerKey(local)
	case tabNotifications:
		m.handleRightNotificationsKey(local)
	case tabAppearance:
		m.handleRightAppearanceKey(local)
	}
}

func (m *SettingsModel) handleLeftGeneralKey(local int) {
	if local == 2 {
		m.stepContextCursorLeft()
	}
}

func (m *SettingsModel) handleLeftPaletteKey(local int) {
	if local == 0 {
		m.togglePaletteTheme()
	}
}

func (m *SettingsModel) handleLeftBehaviorKey(local int) {
	switch local {
	case 12:
		stepBoundedCursor(&m.startViewCursor, -1, len(startViewOptions)-1)
	}
}

func (m *SettingsModel) handleLeftOutlinerKey(local int) {
	if local == 10 {
		stepBoundedCursor(&m.outlinerPriorityMode, -1, 2)
	}
}

func (m *SettingsModel) handleLeftNotificationsKey(local int) {
	if local == 3 {
		stepBoundedCursor(&m.alertWeeklyReviewDay, -1, 6)
	}
}

func (m *SettingsModel) handleLeftAppearanceKey(local int) {
	switch local {
	case 1:
		stepBoundedCursor(&m.weekEndDayCursor, -1, len(weekEndDayOptions)-1)
	case 2:
		stepBoundedCursor(&m.timeFormatCursor, -1, len(timeFormatOptions)-1)
	}
}

func (m *SettingsModel) handleRightGeneralKey(local int) {
	if local == 2 {
		m.stepContextCursorRight()
	}
}

func (m *SettingsModel) handleRightPaletteKey(local int) {
	if local == 0 {
		m.togglePaletteTheme()
	}
}

func (m *SettingsModel) handleRightBehaviorKey(local int) {
	switch local {
	case 12:
		stepBoundedCursor(&m.startViewCursor, 1, len(startViewOptions)-1)
	}
}

func (m *SettingsModel) handleRightOutlinerKey(local int) {
	if local == 10 {
		stepBoundedCursor(&m.outlinerPriorityMode, 1, 2)
	}
}

func (m *SettingsModel) handleRightNotificationsKey(local int) {
	if local == 3 {
		stepBoundedCursor(&m.alertWeeklyReviewDay, 1, 6)
	}
}

func (m *SettingsModel) handleRightAppearanceKey(local int) {
	switch local {
	case 1:
		stepBoundedCursor(&m.weekEndDayCursor, 1, len(weekEndDayOptions)-1)
	case 2:
		stepBoundedCursor(&m.timeFormatCursor, 1, len(timeFormatOptions)-1)
	}
}

func (m *SettingsModel) stepContextCursorLeft() {
	if len(m.contexts) == 0 {
		return
	}
	switch {
	case m.contextCursor > 0:
		m.contextCursor--
	case m.contextCursor == -1:
		m.contextCursor = len(m.contexts) - 1
	default:
		return
	}
	m.contextInput.SetValue(m.contexts[m.contextCursor])
}

func (m *SettingsModel) stepContextCursorRight() {
	if len(m.contexts) == 0 || m.contextCursor < 0 || m.contextCursor >= len(m.contexts)-1 {
		return
	}
	m.contextCursor++
	m.contextInput.SetValue(m.contexts[m.contextCursor])
}

func stepBoundedCursor(cursor *int, delta, maxValue int) {
	switch {
	case delta < 0 && *cursor > 0:
		*cursor--
	case delta > 0 && *cursor < maxValue:
		*cursor++
	}
}

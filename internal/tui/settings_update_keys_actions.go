package tui

import (
	"strconv"

	"github.com/sglavoie/tt/internal/config"
)

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
		case 13:
			m.startViewCursor = (m.startViewCursor + 1) % len(startViewOptions)
		case 14:
			m.taskFormCollapsed = !m.taskFormCollapsed
		case 15:
			m.focusBoardModeCursor = (m.focusBoardModeCursor + 1) % len(focusBoardModeOptions)
		case 16:
			m.dashboardLayoutCursor = (m.dashboardLayoutCursor + 1) % len(dashboardLayoutOptions)
		}
	case tabProjectTree:
		if local == 1 {
			m.treePriorityMode = (m.treePriorityMode + 1) % 3
		} else {
			m.treeColumnDefaults[local] = !m.treeColumnDefaults[local]
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
	case tabKanban:
		switch {
		case local == 0:
			m.kanbanDefaultModeCursor = (m.kanbanDefaultModeCursor + 1) % len(kanbanModeOptions)
		case local == 2:
			m.kanbanShowEmptyColumns = !m.kanbanShowEmptyColumns
		case local == 3:
			m.kanbanDetailPaneVisible = !m.kanbanDetailPaneVisible
		case local == 4:
			m.kanbanDetailHeightCursor = (m.kanbanDetailHeightCursor + 1) % len(kanbanDetailHeightOptions)
		case local >= 5 && local <= 10:
			m.kanbanEnabledModes[local-5] = !m.kanbanEnabledModes[local-5]
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
	case tabProjectTree:
		m.handleLeftProjectTreeKey(local)
	case tabOutliner:
		m.handleLeftOutlinerKey(local)
	case tabKanban:
		m.handleLeftKanbanKey(local)
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
	case tabProjectTree:
		m.handleRightProjectTreeKey(local)
	case tabOutliner:
		m.handleRightOutlinerKey(local)
	case tabKanban:
		m.handleRightKanbanKey(local)
	case tabNotifications:
		m.handleRightNotificationsKey(local)
	case tabAppearance:
		m.handleRightAppearanceKey(local)
	}
}

func (m *SettingsModel) handleLeftGeneralKey(local int) {
	switch local {
	case 2:
		m.stepContextCursorLeft()
	case 4:
		stepBoundedCursor(&m.defaultViewCursor, -1, len(defaultProjectViewOptions)-1)
	}
}

func (m *SettingsModel) handleLeftPaletteKey(local int) {
	if local == 0 {
		m.togglePaletteTheme()
	}
}

func (m *SettingsModel) handleLeftBehaviorKey(local int) {
	switch local {
	case 13:
		stepBoundedCursor(&m.startViewCursor, -1, len(startViewOptions)-1)
	case 15:
		stepBoundedCursor(&m.focusBoardModeCursor, -1, len(focusBoardModeOptions)-1)
	case 16:
		stepBoundedCursor(&m.dashboardLayoutCursor, -1, len(dashboardLayoutOptions)-1)
	}
}

func (m *SettingsModel) handleLeftProjectTreeKey(local int) {
	if local == 1 {
		stepBoundedCursor(&m.treePriorityMode, -1, 2)
	}
}

func (m *SettingsModel) handleLeftOutlinerKey(local int) {
	if local == 10 {
		stepBoundedCursor(&m.outlinerPriorityMode, -1, 2)
	}
}

func (m *SettingsModel) handleLeftKanbanKey(local int) {
	switch local {
	case 0:
		stepBoundedCursor(&m.kanbanDefaultModeCursor, -1, len(kanbanModeOptions)-1)
	case 4:
		stepBoundedCursor(&m.kanbanDetailHeightCursor, -1, len(kanbanDetailHeightOptions)-1)
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
	switch local {
	case 2:
		m.stepContextCursorRight()
	case 4:
		stepBoundedCursor(&m.defaultViewCursor, 1, len(defaultProjectViewOptions)-1)
	}
}

func (m *SettingsModel) handleRightPaletteKey(local int) {
	if local == 0 {
		m.togglePaletteTheme()
	}
}

func (m *SettingsModel) handleRightBehaviorKey(local int) {
	switch local {
	case 13:
		stepBoundedCursor(&m.startViewCursor, 1, len(startViewOptions)-1)
	case 15:
		stepBoundedCursor(&m.focusBoardModeCursor, 1, len(focusBoardModeOptions)-1)
	case 16:
		stepBoundedCursor(&m.dashboardLayoutCursor, 1, len(dashboardLayoutOptions)-1)
	}
}

func (m *SettingsModel) handleRightProjectTreeKey(local int) {
	if local == 1 {
		stepBoundedCursor(&m.treePriorityMode, 1, 2)
	}
}

func (m *SettingsModel) handleRightOutlinerKey(local int) {
	if local == 10 {
		stepBoundedCursor(&m.outlinerPriorityMode, 1, 2)
	}
}

func (m *SettingsModel) handleRightKanbanKey(local int) {
	switch local {
	case 0:
		stepBoundedCursor(&m.kanbanDefaultModeCursor, 1, len(kanbanModeOptions)-1)
	case 4:
		stepBoundedCursor(&m.kanbanDetailHeightCursor, 1, len(kanbanDetailHeightOptions)-1)
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

// handleResetToDefault resets the currently focused field to its default value.
//
//nolint:gocyclo // covers all field types across every settings tab
func (m *SettingsModel) handleResetToDefault() {
	meta, ok := settingsFieldByIndex(m.focusIndex)
	if !ok {
		return
	}

	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]
	resetDone := false

	switch tab {
	case tabGeneral:
		switch local {
		case 0:
			m.editorInput.SetValue("code")
			resetDone = true
		case 1:
			m.externalEditorInput.SetValue("")
			resetDone = true
		case 2:
			m.contextInput.SetValue("personal")
			m.contextCursor = settingsOptionCursor(m.contexts, "personal", -1)
			resetDone = true
		case 3:
			m.undoDepthInput.SetValue("50")
			resetDone = true
		case 4:
			m.defaultViewCursor = 0
			resetDone = true
		}
	case tabReview:
		switch local {
		case 0:
			m.dueSoonInput.SetValue("7")
			resetDone = true
		case 1:
			m.futureInput.SetValue("30")
			resetDone = true
		case 2:
			m.oldDaysInput.SetValue("14")
			resetDone = true
		case 3:
			m.completedHoursInput.SetValue("24")
			resetDone = true
		case 4:
			m.staleTaskDaysInput.SetValue("0")
			resetDone = true
		}
	case tabBehavior:
		switch local {
		case 0:
			m.recentActivityInput.SetValue("20")
			resetDone = true
		case 1:
			m.recentActivityShowAll = false
			resetDone = true
		case 2:
			m.linkedTaskJump = false
			resetDone = true
		case 3:
			m.maxPinnedInput.SetValue("5")
			resetDone = true
		case 4:
			m.maxRecentTaskVisitsInput.SetValue("15")
			resetDone = true
		case 5:
			m.popupTimeoutInput.SetValue("0")
			resetDone = true
		case 6:
			m.sectionLimitActiveInput.SetValue("5")
			resetDone = true
		case 7:
			m.sectionLimitNextInput.SetValue("5")
			resetDone = true
		case 8:
			m.sectionLimitInboxInput.SetValue("5")
			resetDone = true
		case 9:
			m.sectionLimitStandbyInput.SetValue("5")
			resetDone = true
		case 10:
			m.sectionLimitWaitingInput.SetValue("3")
			resetDone = true
		case 11:
			m.sectionLimitSomedayInput.SetValue("10")
			resetDone = true
		case 12:
			m.sectionLimitReviewInput.SetValue("5")
			resetDone = true
		case 13:
			m.startViewCursor = 0
			resetDone = true
		case 14:
			m.taskFormCollapsed = false
			resetDone = true
		case 15:
			m.focusBoardModeCursor = 0
			resetDone = true
		case 16:
			m.dashboardLayoutCursor = 0
			resetDone = true
		case 17:
			m.dashboardHorizSplitInput.SetValue(strconv.Itoa(config.DefaultDashboardHorizontalSplitPct))
			resetDone = true
		case 18:
			m.dashboardVertSplitInput.SetValue(strconv.Itoa(config.DefaultDashboardVerticalSplitPct))
			resetDone = true
		}
	case tabAttachments:
		switch local {
		case 0:
			m.attachmentEditorInput.SetValue("")
			resetDone = true
		case 1:
			m.codeEditorInput.SetValue("code")
			resetDone = true
		}
	case tabAppearance:
		switch local {
		case 0:
			m.zebraStriping = false
			resetDone = true
		case 1:
			m.weekEndDayCursor = 2
			resetDone = true
		case 2:
			m.timeFormatCursor = 1
			resetDone = true
		case 3:
			m.autoTheme = false
			resetDone = true
		case 4:
			m.useEmoji = false
			resetDone = true
		}
	}

	if !resetDone {
		return
	}
	label := meta.Default
	if label == "" {
		label = "(empty)"
	}
	m.searchStatusMsg = "Reset to default: " + label
}

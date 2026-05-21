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
		switch local {
		case 2:
			if len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == -1 {
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}
		case 4:
			if m.defaultViewCursor > 0 {
				m.defaultViewCursor--
			}
		}
	case tabPalette:
		if local == 0 {
			m.togglePaletteTheme()
		}
	case tabBehavior:
		switch {
		case local == 13 && m.startViewCursor > 0:
			m.startViewCursor--
		case local == 15 && m.focusBoardModeCursor > 0:
			m.focusBoardModeCursor--
		case local == 16 && m.dashboardLayoutCursor > 0:
			m.dashboardLayoutCursor--
		}
	case tabProjectTree:
		if local == 1 && m.treePriorityMode > 0 {
			m.treePriorityMode--
		}
	case tabOutliner:
		if local == 10 && m.outlinerPriorityMode > 0 {
			m.outlinerPriorityMode--
		}
	case tabKanban:
		switch local {
		case 0:
			if m.kanbanDefaultModeCursor > 0 {
				m.kanbanDefaultModeCursor--
			}
		case 4:
			if m.kanbanDetailHeightCursor > 0 {
				m.kanbanDetailHeightCursor--
			}
		}
	case tabNotifications:
		if local == 3 && m.alertWeeklyReviewDay > 0 {
			m.alertWeeklyReviewDay--
		}
	case tabAppearance:
		switch local {
		case 1:
			if m.weekEndDayCursor > 0 {
				m.weekEndDayCursor--
			}
		case 2:
			if m.timeFormatCursor > 0 {
				m.timeFormatCursor--
			}
		}
	}
}

func (m *SettingsModel) handleRightKey() {
	tab := tabForField(m.focusIndex)
	local := m.focusIndex - tabFieldStart[tab]

	switch tab {
	case tabGeneral:
		switch local {
		case 2:
			if len(m.contexts) > 0 && m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
				m.contextCursor++
				m.contextInput.SetValue(m.contexts[m.contextCursor])
			}
		case 4:
			if m.defaultViewCursor < len(defaultProjectViewOptions)-1 {
				m.defaultViewCursor++
			}
		}
	case tabPalette:
		if local == 0 {
			m.togglePaletteTheme()
		}
	case tabBehavior:
		switch {
		case local == 13 && m.startViewCursor < len(startViewOptions)-1:
			m.startViewCursor++
		case local == 15 && m.focusBoardModeCursor < len(focusBoardModeOptions)-1:
			m.focusBoardModeCursor++
		case local == 16 && m.dashboardLayoutCursor < len(dashboardLayoutOptions)-1:
			m.dashboardLayoutCursor++
		}
	case tabProjectTree:
		if local == 1 && m.treePriorityMode < 2 {
			m.treePriorityMode++
		}
	case tabOutliner:
		if local == 10 && m.outlinerPriorityMode < 2 {
			m.outlinerPriorityMode++
		}
	case tabKanban:
		switch local {
		case 0:
			if m.kanbanDefaultModeCursor < len(kanbanModeOptions)-1 {
				m.kanbanDefaultModeCursor++
			}
		case 4:
			if m.kanbanDetailHeightCursor < len(kanbanDetailHeightOptions)-1 {
				m.kanbanDetailHeightCursor++
			}
		}
	case tabNotifications:
		if local == 3 && m.alertWeeklyReviewDay < 6 {
			m.alertWeeklyReviewDay++
		}
	case tabAppearance:
		switch local {
		case 1:
			if m.weekEndDayCursor < len(weekEndDayOptions)-1 {
				m.weekEndDayCursor++
			}
		case 2:
			if m.timeFormatCursor < len(timeFormatOptions)-1 {
				m.timeFormatCursor++
			}
		}
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
			m.notesEditorInput.SetValue("")
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

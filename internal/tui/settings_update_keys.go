package tui

import (
	"fmt"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) handleKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// When search overlay is active, consume all keys there.
	if m.searchActive {
		return m.handleSearchKey(msg)
	}
	// When a preset overlay is active on the palette tab, route there first.
	if m.tabs.Active == tabPalette && m.themePresetMode != presetModeNone {
		return m.handlePresetKey(msg)
	}
	if handled := m.handleStageConfirmKey(msg.String()); handled {
		return true, nil
	}
	if handled := m.handleStageListKey(msg.String()); handled {
		return true, nil
	}
	return m.handleGlobalKey(msg.String())
}

func (m *SettingsModel) handleStageConfirmKey(key string) bool {
	if m.tabs.Active != tabStages || !m.stageConfirmDelete.IsActive() {
		return false
	}

	confirmed, handled := m.stageConfirmDelete.HandleKey(key)
	if !handled {
		return false
	}
	if confirmed && len(m.stages) > 0 {
		m.stages = append(m.stages[:m.stageCursor], m.stages[m.stageCursor+1:]...)
		if m.stageCursor >= len(m.stages) && m.stageCursor > 0 {
			m.stageCursor--
		}
	}
	return true
}

func (m *SettingsModel) handleStageListKey(key string) bool {
	if m.tabs.Active != tabStages || m.stageEditMode {
		return false
	}

	switch key {
	case "j", "down":
		if len(m.stages) > 0 && m.stageCursor < len(m.stages)-1 {
			m.stageCursor++
		}
		return true
	case "k", "up":
		if len(m.stages) > 0 && m.stageCursor > 0 {
			m.stageCursor--
		}
		return true
	case "a":
		m.stageEditMode = true
		m.stageAddMode = true
		m.stageInput.SetValue("")
		m.stageInput.Focus()
		return true
	case "d", "delete", "backspace":
		if len(m.stages) > 0 {
			m.stageConfirmDelete.Show("delete", fmt.Sprintf("Delete stage '%s'?", m.stages[m.stageCursor]))
		}
		return true
	case "J":
		if len(m.stages) > 1 && m.stageCursor < len(m.stages)-1 {
			m.stages[m.stageCursor], m.stages[m.stageCursor+1] = m.stages[m.stageCursor+1], m.stages[m.stageCursor]
			m.stageCursor++
		}
		return true
	case "K":
		if len(m.stages) > 1 && m.stageCursor > 0 {
			m.stages[m.stageCursor], m.stages[m.stageCursor-1] = m.stages[m.stageCursor-1], m.stages[m.stageCursor]
			m.stageCursor--
		}
		return true
	case "e":
		if len(m.stages) > 0 {
			m.stageEditMode = true
			m.stageAddMode = false
			m.stageInput.SetValue(m.stages[m.stageCursor])
			m.stageInput.Focus()
		}
		return true
	default:
		return false
	}
}

func (m *SettingsModel) handleGlobalKey(key string) (bool, tea.Cmd) { //nolint:gocyclo
	switch key {
	case "esc":
		if m.stageEditMode {
			m.stageEditMode = false
			m.stageAddMode = false
			m.stageInput.Blur()
			return true, nil
		}
		if m.subTabRowFocused {
			m.subTabRowFocused = false
			return true, nil
		}
		return true, func() tea.Msg {
			return BackMsg{}
		}

	case "shift+down":
		if visualSubTabNamesForVTab(m.visualTab) != nil {
			m.subTabRowFocused = true
		}
		return true, nil
	case "shift+up":
		if visualSubTabNamesForVTab(m.visualTab) != nil {
			m.subTabRowFocused = false
		}
		return true, nil

	case "tab", "shift+tab":
		if m.stageEditMode {
			return false, nil
		}
		m.blurCurrentField()
		if key == "tab" {
			m.focusIndex = (m.focusIndex + 1) % settingsFieldCount
		} else {
			m.focusIndex = (m.focusIndex + settingsFieldCount - 1) % settingsFieldCount
		}
		m.tabs.Active = tabForField(m.focusIndex)
		m.subTab = subTabForField(m.focusIndex)
		m.syncVisualFromOld()
		m.focusCurrentField()
		return true, nil

	case "shift+left", "shift+right":
		m.blurCurrentField()
		delta := 1
		if key == "shift+left" {
			delta = -1
		}
		if m.subTabRowFocused && visualSubTabNamesForVTab(m.visualTab) != nil {
			// Cycle within the current visual tab's sub-tabs
			refs := visualTabSubTabs[m.visualTab]
			CycleTab(&m.visualSubTab, len(refs), delta)
			m.tabs.Active = refs[m.visualSubTab].oldTab
			m.subTab = refs[m.visualSubTab].oldSubTab
			m.focusIndex = subTabFieldStart(m.tabs.Active, m.subTab)
			m.scrollOffset = 0
			m.focusCurrentField()
			return true, nil
		}
		// Cycle visual tabs
		CycleTab(&m.visualTab, len(settingsVisualTabNames), delta)
		m.visualSubTab = 0
		m.setFromVisual()
		m.subTabRowFocused = false
		m.scrollOffset = 0
		m.focusCurrentField()
		return true, nil

	case "space":
		m.handleSpaceKey()
		return true, nil

	case "enter":
		if m.tabs.Active == tabStages && m.stageEditMode {
			return true, m.commitStageEditCmd()
		}
		if err := m.validateAll(); err != nil {
			m.validationErr = err.Error()
			return true, nil
		}
		return true, m.saveSettings()

	case "ctrl+s":
		// On the palette tab, ctrl+s opens the save preset overlay.
		if m.tabs.Active == tabPalette {
			m.openPresetSave()
			return true, nil
		}
		if m.stageEditMode {
			if cmd := m.commitStageEditCmd(); cmd != nil {
				return true, cmd
			}
		}
		if err := m.validateAll(); err != nil {
			m.validationErr = err.Error()
			return true, nil
		}
		return true, m.saveSettings()

	case "ctrl+o":
		if m.tabs.Active == tabPalette {
			m.openPresetLoad()
		}
		return true, nil

	case "ctrl+r":
		if m.tabs.Active == tabPalette {
			m.themePresetMode = presetModeResetConfirm
			m.themePresetErr = ""
		}
		return true, nil

	case "/":
		si := textinput.New()
		si.Placeholder = "search settings..."
		si.CharLimit = 100
		si.SetWidth(SearchInputWidth(m.width))
		applyThemedTextInput(&si)
		si.Focus()
		m.searchInput = si
		m.searchActive = true
		m.searchResults = searchSettingsFields("")
		m.searchCursor = 0
		m.searchStatusMsg = ""
		return true, textinput.Blink

	case "ctrl+d":
		m.handleResetToDefault()
		return true, nil

	case "?":
		return true, helpCmd(ViewSettings, "Settings", m.GetHelpItems())

	case "left", "h":
		m.handleLeftKey()
		return false, nil

	case "right", "l":
		m.handleRightKey()
		return false, nil

	default:
		return false, nil
	}
}

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
		case 1: // show completed
			m.recentActivityShowAll = !m.recentActivityShowAll
		case 2: // after creation
			m.linkedTaskJump = !m.linkedTaskJump
		case 13: // start view
			m.startViewCursor = (m.startViewCursor + 1) % len(startViewOptions)
		case 14: // task form collapsed
			m.taskFormCollapsed = !m.taskFormCollapsed
		case 15: // focus board default mode
			m.focusBoardModeCursor = (m.focusBoardModeCursor + 1) % len(focusBoardModeOptions)
		}
	case tabProjectTree:
		if local == 1 { // priority column
			m.treePriorityMode = (m.treePriorityMode + 1) % 3
		} else {
			m.treeColumnDefaults[local] = !m.treeColumnDefaults[local]
		}
	case tabOutliner:
		switch {
		case local == 0:
			m.crossOutlinerSidebar = !m.crossOutlinerSidebar
		case local == 1:
			m.crossOutlinerCycleOnly = !m.crossOutlinerCycleOnly
		case local >= 2 && local <= 8:
			m.outlinerCollapsed[local-2] = !m.outlinerCollapsed[local-2]
		case local == 10: // priority column in column visibility
			m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
		case local >= 9 && local <= 21:
			m.outlinerColumnDefaults[local-9] = !m.outlinerColumnDefaults[local-9]
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
		case 4: // overdue
			m.alertOverdueEnabled = !m.alertOverdueEnabled
		case 5: // due soon
			m.alertDueSoonEnabled = !m.alertDueSoonEnabled
		case 7: // stale
			m.alertStaleEnabled = !m.alertStaleEnabled
		case 9: // inbox
			m.alertInboxEnabled = !m.alertInboxEnabled
		case 11: // blocked
			m.alertBlockedEnabled = !m.alertBlockedEnabled
		case 13: // project health
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
		case 2: // context
			if len(m.contexts) > 0 {
				if m.contextCursor > 0 {
					m.contextCursor--
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				} else if m.contextCursor == -1 {
					m.contextCursor = len(m.contexts) - 1
					m.contextInput.SetValue(m.contexts[m.contextCursor])
				}
			}
		case 4: // default view
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
		case 2: // context
			if len(m.contexts) > 0 && m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
				m.contextCursor++
				m.contextInput.SetValue(m.contexts[m.contextCursor])
			}
		case 4: // default view
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

func (m *SettingsModel) commitStageEditCmd() tea.Cmd {
	if err := m.commitStageEdit(); err != nil {
		m.stageValidationErr = err.Error()
		return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return stageValidationClearMsg{}
		})
	}
	return nil
}

// handleSearchKey consumes all keys when the settings search overlay is active.
func (m *SettingsModel) handleSearchKey(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.searchActive = false
		m.searchInput.Blur()
		return true, nil
	case "enter":
		if len(m.searchResults) > 0 && m.searchCursor < len(m.searchResults) {
			result := m.searchResults[m.searchCursor]
			m.blurCurrentField()
			m.focusIndex = result.FocusIndex
			m.tabs.Active = result.Tab
			m.subTab = result.SubTab
			m.syncVisualFromOld()
			m.focusCurrentField()
			m.scrollOffset = 0
		}
		m.searchActive = false
		m.searchInput.Blur()
		return true, nil
	case "j", "down", "ctrl+j":
		if m.searchCursor < len(m.searchResults)-1 {
			m.searchCursor++
		}
		return true, nil
	case "k", "up", "ctrl+k":
		if m.searchCursor > 0 {
			m.searchCursor--
		}
		return true, nil
	default:
		old := m.searchInput.Value()
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		if m.searchInput.Value() != old {
			m.searchResults = searchSettingsFields(m.searchInput.Value())
			m.searchCursor = 0
		}
		return true, cmd
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
			m.weekEndDayCursor = 2 // Sunday
			resetDone = true
		case 2:
			m.timeFormatCursor = 1 // 24-hour
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

package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) handleKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
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
		if subTabNamesForTab(m.tabs.Active) != nil {
			m.subTabRowFocused = true
		}
		return true, nil
	case "shift+up":
		if subTabNamesForTab(m.tabs.Active) != nil {
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
		m.focusCurrentField()
		return true, nil

	case "shift+left", "shift+right":
		m.blurCurrentField()
		if m.subTabRowFocused && subTabNamesForTab(m.tabs.Active) != nil {
			sections := settingsTabSections[m.tabs.Active]
			delta := 1
			if key == "shift+left" {
				delta = -1
			}
			CycleTab(&m.subTab, len(sections), delta)
			m.focusIndex = subTabFieldStart(m.tabs.Active, m.subTab)
			m.scrollOffset = 0
			m.focusCurrentField()
			return true, nil
		}
		if handleTabCycleKey(key, &m.tabs, func() {
			m.focusIndex = tabFieldStart[m.tabs.Active]
			m.subTab = 0
			m.subTabRowFocused = false
			m.scrollOffset = 0
			m.focusCurrentField()
		}) {
			return true, nil
		}
		return false, nil

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
		if local == 0 {
			m.zebraStriping = !m.zebraStriping
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

package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) handleKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	// When a preset overlay is active on the palette tab, route there first.
	if m.tabs.Active == paletteTabIndex && m.themePresetMode != presetModeNone {
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
	if m.tabs.Active != 8 || !m.stageConfirmDelete.IsActive() {
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
	if m.tabs.Active != 8 || m.stageEditMode {
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

	case "(", ")":
		if subTabNamesForTab(m.tabs.Active) != nil {
			m.subTabRowFocused = !m.subTabRowFocused
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

	case "[", "]":
		m.blurCurrentField()
		if m.subTabRowFocused && subTabNamesForTab(m.tabs.Active) != nil {
			sections := settingsTabSections[m.tabs.Active]
			delta := 1
			if key == "[" {
				delta = -1
			}
			CycleTab(&m.subTab, len(sections), delta)
			m.focusIndex = subTabFieldStart(m.tabs.Active, m.subTab)
			m.scrollOffset = 0
			m.focusCurrentField()
			return true, nil
		}
		if handleTabCycleKey(key, &m.tabs, false, func() {
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
		if m.tabs.Active == 8 && m.stageEditMode {
			return true, m.commitStageEditCmd()
		}
		if err := m.validateAll(); err != nil {
			m.validationErr = err.Error()
			return true, nil
		}
		return true, m.saveSettings()

	case "ctrl+s":
		// On the palette tab, ctrl+s opens the save preset overlay.
		if m.tabs.Active == paletteTabIndex {
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
		if m.tabs.Active == paletteTabIndex {
			m.openPresetLoad()
		}
		return true, nil

	case "ctrl+r":
		if m.tabs.Active == paletteTabIndex {
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
	snapshot := [17]string{
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
	m.inactiveThemeDraft = snapshot
	m.activePresetName, m.inactivePresetName = m.inactivePresetName, m.activePresetName
	if m.activeTheme == "light" {
		m.activeTheme = "dark"
	} else {
		m.activeTheme = "light"
	}
}

func (m *SettingsModel) handleSpaceKey() {
	if m.focusIndex == 4 {
		m.togglePaletteTheme()
		return
	}
	if m.focusIndex == 27 {
		m.recentActivityShowAll = !m.recentActivityShowAll
	}
	if m.focusIndex == 28 {
		m.linkedTaskJump = !m.linkedTaskJump
	}
	if m.focusIndex >= 41 && m.focusIndex <= 53 {
		idx := m.focusIndex - 41
		if m.focusIndex == 42 {
			m.treePriorityMode = (m.treePriorityMode + 1) % 3
		} else {
			m.treeColumnDefaults[idx] = !m.treeColumnDefaults[idx]
		}
	}
	if m.focusIndex == 54 {
		m.crossOutlinerSidebar = !m.crossOutlinerSidebar
	}
	if m.focusIndex == 91 {
		m.zebraStriping = !m.zebraStriping
	}
	if m.focusIndex >= 55 && m.focusIndex <= 61 {
		idx := m.focusIndex - 55
		m.outlinerCollapsed[idx] = !m.outlinerCollapsed[idx]
	}
	if m.focusIndex >= 62 && m.focusIndex <= 74 {
		if m.focusIndex == 63 {
			m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
		} else {
			idx := m.focusIndex - 62
			m.outlinerColumnDefaults[idx] = !m.outlinerColumnDefaults[idx]
		}
	}
	switch m.focusIndex {
	case 75:
		m.alertDailyReviewEnabled = !m.alertDailyReviewEnabled
	case 77:
		m.alertWeeklyReviewEnabled = !m.alertWeeklyReviewEnabled
	case 79:
		m.alertOverdueEnabled = !m.alertOverdueEnabled
	case 80:
		m.alertDueSoonEnabled = !m.alertDueSoonEnabled
	case 82:
		m.alertStaleEnabled = !m.alertStaleEnabled
	case 84:
		m.alertInboxEnabled = !m.alertInboxEnabled
	case 86:
		m.alertBlockedEnabled = !m.alertBlockedEnabled
	case 88:
		m.alertProjectHealthEnabled = !m.alertProjectHealthEnabled
	}
}

func (m *SettingsModel) handleLeftKey() {
	if m.focusIndex == 4 {
		m.togglePaletteTheme()
		return
	}
	if m.focusIndex == 1 && len(m.contexts) > 0 {
		if m.contextCursor > 0 {
			m.contextCursor--
			m.contextInput.SetValue(m.contexts[m.contextCursor])
		} else if m.contextCursor == -1 {
			m.contextCursor = len(m.contexts) - 1
			m.contextInput.SetValue(m.contexts[m.contextCursor])
		}
	}
	if m.focusIndex == 3 && m.defaultViewCursor > 0 {
		m.defaultViewCursor--
	}
	switch m.focusIndex {
	case 42:
		if m.treePriorityMode > 0 {
			m.treePriorityMode--
		}
	case 63:
		if m.outlinerPriorityMode > 0 {
			m.outlinerPriorityMode--
		}
	}
	if m.focusIndex == 78 && m.alertWeeklyReviewDay > 0 {
		m.alertWeeklyReviewDay--
	}
}

func (m *SettingsModel) handleRightKey() {
	if m.focusIndex == 4 {
		m.togglePaletteTheme()
		return
	}
	if m.focusIndex == 1 && len(m.contexts) > 0 {
		if m.contextCursor >= 0 && m.contextCursor < len(m.contexts)-1 {
			m.contextCursor++
			m.contextInput.SetValue(m.contexts[m.contextCursor])
		}
	}
	if m.focusIndex == 3 && m.defaultViewCursor < len(defaultProjectViewOptions)-1 {
		m.defaultViewCursor++
	}
	switch m.focusIndex {
	case 42:
		if m.treePriorityMode < 2 {
			m.treePriorityMode++
		}
	case 63:
		if m.outlinerPriorityMode < 2 {
			m.outlinerPriorityMode++
		}
	}
	if m.focusIndex == 78 && m.alertWeeklyReviewDay < 6 {
		m.alertWeeklyReviewDay++
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

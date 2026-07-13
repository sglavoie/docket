package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) handleKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
	if m.searchActive {
		return m.handleSearchKey(msg)
	}
	if m.tabs.Active == tabPalette && m.themePresetMode != presetModeNone {
		return m.handlePresetKey(msg)
	}
	return m.handleGlobalKey(msg.String())
}

func (m *SettingsModel) handleGlobalKey(key string) (bool, tea.Cmd) { //nolint:gocyclo
	switch key {
	case "esc":
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
			refs := visualTabSubTabs[m.visualTab]
			CycleTab(&m.visualSubTab, len(refs), delta)
			m.tabs.Active = refs[m.visualSubTab].oldTab
			m.subTab = refs[m.visualSubTab].oldSubTab
			m.focusIndex = subTabFieldStart(m.tabs.Active, m.subTab)
			m.scrollOffset = 0
			m.focusCurrentField()
			return true, nil
		}
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
		if err := m.validateAll(); err != nil {
			m.validationErr = err.Error()
			return true, nil
		}
		return true, m.saveSettings()

	case "ctrl+s":
		if m.tabs.Active == tabPalette {
			m.openPresetSave()
			return true, nil
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

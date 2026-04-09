package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

func (m *SettingsModel) handleKeyPress(msg tea.KeyPressMsg) (bool, tea.Cmd) {
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

func (m *SettingsModel) handleSpaceKey() {
	if m.focusIndex == 21 {
		m.recentActivityShowAll = !m.recentActivityShowAll
	}
	if m.focusIndex == 22 {
		m.linkedTaskJump = !m.linkedTaskJump
	}
	if m.focusIndex >= 35 && m.focusIndex <= 47 {
		idx := m.focusIndex - 35
		if m.focusIndex == 36 {
			m.treePriorityMode = (m.treePriorityMode + 1) % 3
		} else {
			m.treeColumnDefaults[idx] = !m.treeColumnDefaults[idx]
		}
	}
	if m.focusIndex == 48 {
		m.crossOutlinerSidebar = !m.crossOutlinerSidebar
	}
	if m.focusIndex == 85 {
		m.zebraStriping = !m.zebraStriping
	}
	if m.focusIndex >= 49 && m.focusIndex <= 55 {
		idx := m.focusIndex - 49
		m.outlinerCollapsed[idx] = !m.outlinerCollapsed[idx]
	}
	if m.focusIndex >= 56 && m.focusIndex <= 68 {
		if m.focusIndex == 57 {
			m.outlinerPriorityMode = (m.outlinerPriorityMode + 1) % 3
		} else {
			idx := m.focusIndex - 56
			m.outlinerColumnDefaults[idx] = !m.outlinerColumnDefaults[idx]
		}
	}
	switch m.focusIndex {
	case 69:
		m.alertDailyReviewEnabled = !m.alertDailyReviewEnabled
	case 71:
		m.alertWeeklyReviewEnabled = !m.alertWeeklyReviewEnabled
	case 73:
		m.alertOverdueEnabled = !m.alertOverdueEnabled
	case 74:
		m.alertDueSoonEnabled = !m.alertDueSoonEnabled
	case 76:
		m.alertStaleEnabled = !m.alertStaleEnabled
	case 78:
		m.alertInboxEnabled = !m.alertInboxEnabled
	case 80:
		m.alertBlockedEnabled = !m.alertBlockedEnabled
	case 82:
		m.alertProjectHealthEnabled = !m.alertProjectHealthEnabled
	}
}

func (m *SettingsModel) handleLeftKey() {
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
	case 36:
		if m.treePriorityMode > 0 {
			m.treePriorityMode--
		}
	case 57:
		if m.outlinerPriorityMode > 0 {
			m.outlinerPriorityMode--
		}
	}
	if m.focusIndex == 72 && m.alertWeeklyReviewDay > 0 {
		m.alertWeeklyReviewDay--
	}
}

func (m *SettingsModel) handleRightKey() {
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
	case 36:
		if m.treePriorityMode < 2 {
			m.treePriorityMode++
		}
	case 57:
		if m.outlinerPriorityMode < 2 {
			m.outlinerPriorityMode++
		}
	}
	if m.focusIndex == 72 && m.alertWeeklyReviewDay < 6 {
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

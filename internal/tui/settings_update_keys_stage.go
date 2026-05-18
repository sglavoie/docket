package tui

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
)

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

func (m *SettingsModel) commitStageEditCmd() tea.Cmd {
	if err := m.commitStageEdit(); err != nil {
		m.stageValidationErr = err.Error()
		return tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return stageValidationClearMsg{}
		})
	}
	return nil
}

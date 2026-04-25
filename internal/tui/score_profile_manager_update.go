package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

func (m *ScoreProfileManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.confirm.IsActive() {
			confirmed, handled := m.confirm.HandleKey(msg.String())
			if handled {
				if confirmed {
					return m, m.executeDelete()
				}
				return m, nil
			}
		}

		if m.pickingPreset {
			return m.handlePresetPicker(msg)
		}

		return m.handleKeypress(msg)
	}

	return m, nil
}

func (m *ScoreProfileManagerModel) HandledKeys() []string {
	return []string{
		"j", "down", "k", "up", "g", "G",
		"a", "d", "enter", "space", "u",
	}
}

func (m *ScoreProfileManagerModel) handleKeypress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.pendingPrefix != "" && isModifierOnly(msg) {
		return m, nil
	}
	if m.pendingPrefix != "" {
		prefix := m.pendingPrefix
		m.pendingPrefix = ""
		if key != "esc" && prefix == "g" && key == "g" {
			m.cursor = 0
		}
		return m, nil
	}

	switch key {
	case "esc":
		return m, func() tea.Msg {
			return CloseScoreProfileManagerMsg{}
		}

	case "j", "down":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
		return m, nil

	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
		return m, nil

	case "g":
		m.pendingPrefix = "g"
		return m, nil

	case "G":
		if len(m.profiles) > 0 {
			m.cursor = len(m.profiles) - 1
		}
		return m, nil

	case "a":
		return m, m.startAddPreset()

	case "d":
		if len(m.profiles) > 0 && m.cursor < len(m.profiles) {
			profile := m.profiles[m.cursor]
			m.confirm.Show("delete", fmt.Sprintf("Delete profile %q?", profile.Name))
		}
		return m, nil

	case "enter", " ":
		return m, m.assignProfile()

	case "u":
		return m, m.unassignProfile()
	}

	return m, nil
}

func (m *ScoreProfileManagerModel) handlePresetPicker(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		m.pickingPreset = false
		return m, nil

	case "j", "down":
		if m.presetCursor < len(m.presetOptions)-1 {
			m.presetCursor++
		}
		return m, nil

	case "k", "up":
		if m.presetCursor > 0 {
			m.presetCursor--
		}
		return m, nil

	case "enter":
		return m, m.addPreset(m.presetOptions[m.presetCursor])
	}

	return m, nil
}

package tui

import (
	tea "charm.land/bubbletea/v2"
)

// Init initializes the model.
func (m *SectionManagerModel) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (m *SectionManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case SectionColorChangedMsg:
		m.buildSectionList()
		m.colorPickMode = false
		return m, nil

	case ErrorMsg:
		m.errorMsg = msg.Err.Error()
		m.colorPickMode = false
		return m, nil

	case tea.KeyPressMsg:
		m.errorMsg = ""

		if m.colorPickMode {
			return m.handleColorPickMode(msg)
		}

		return m.handleDefaultMode(msg)
	}

	return m, nil
}

func closeSectionManagerCmd() tea.Cmd {
	return func() tea.Msg {
		return CloseSectionManagerMsg{}
	}
}

func (m *SectionManagerModel) handleDefaultMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+s":
		return m, closeSectionManagerCmd()

	case "j", "down":
		if len(m.sections) > 0 {
			m.cursor++
			if m.cursor >= len(m.sections) {
				m.cursor = 0
			}
		}
		return m, nil

	case "k", "up":
		if len(m.sections) > 0 {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.sections) - 1
			}
		}
		return m, nil

	case "enter":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			return m, m.filterBySection(m.sections[m.cursor].Name)
		}
		return m, nil

	case "c":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			section := m.sections[m.cursor]
			m.colorPickMode = true
			currentColor := m.getProject().GetSectionColor(section.Name)
			m.colorCursor = GetSectionColorIndex(currentColor)
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

// handleColorPickMode handles keyboard input while in color pick mode.
func (m *SectionManagerModel) handleColorPickMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	paletteLen := len(SectionColorPalette)

	switch msg.String() {
	case "esc":
		m.colorPickMode = false
		return m, nil

	case "enter":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			sectionName := m.sections[m.cursor].Name
			colorName := GetSectionColorByIndex(m.colorCursor)
			m.colorPickMode = false
			return m, m.setSectionColor(sectionName, colorName)
		}
		m.colorPickMode = false
		return m, nil

	case "c":
		m.colorCursor = (m.colorCursor + 1) % paletteLen
		return m, nil

	case "h", "left":
		m.colorCursor = (m.colorCursor - 1 + paletteLen) % paletteLen
		return m, nil

	case "l", "right":
		m.colorCursor = (m.colorCursor + 1) % paletteLen
		return m, nil

	case "j", "down":
		m.colorCursor = (m.colorCursor + 5) % paletteLen
		return m, nil

	case "k", "up":
		m.colorCursor = (m.colorCursor - 5 + paletteLen) % paletteLen
		return m, nil

	case "x":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			sectionName := m.sections[m.cursor].Name
			m.colorPickMode = false
			return m, m.setSectionColor(sectionName, "gray")
		}
		return m, nil
	}

	return m, nil
}

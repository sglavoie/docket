package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/models"
)

// Init initializes the model.
func (m *SectionManagerModel) Init() tea.Cmd {
	if m.addMode {
		return textinput.Blink
	}
	return nil
}

// Update handles messages.
func (m *SectionManagerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case SectionRenamedMsg, SectionAddedMsg, SectionRemovedMsg:
		return m, closeSectionManagerCmd()

	case SectionColorChangedMsg:
		m.buildSectionList()
		m.colorPickMode = false
		return m, nil

	case ErrorMsg:
		m.errorMsg = msg.Err.Error()
		m.renameMode = false
		m.addMode = false
		m.colorPickMode = false
		m.renameInput.SetValue("")
		m.addInput.SetValue("")
		return m, nil

	case tea.KeyPressMsg:
		if m.confirmDiscard {
			return m, m.handleConfirmDiscard(msg)
		}

		m.errorMsg = ""

		if m.colorPickMode {
			return m.handleColorPickMode(msg)
		}
		if m.renameMode {
			return m.handleRenameMode(msg)
		}
		if m.addMode {
			return m.handleAddMode(msg)
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

func (m *SectionManagerModel) handleConfirmDiscard(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y":
		m.confirmDiscard = false
		switch m.confirmAction {
		case "discard-rename":
			m.renameMode = false
			m.renameInput.SetValue("")
		case "discard-add":
			m.addMode = false
			m.addInput.SetValue("")
		}
		m.confirmAction = ""
		return nil
	case "n", "N", "esc":
		m.confirmDiscard = false
		m.confirmAction = ""
		return nil
	default:
		return nil
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

	case "g":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			return m, func() tea.Msg {
				return GoToSectionMsg{SectionName: m.sections[m.cursor].Name}
			}
		}
		return m, nil

	case "a":
		m.addMode = true
		m.addInput.SetValue("")
		m.addInput.Focus()
		return m, textinput.Blink

	case "r":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			section := m.sections[m.cursor]
			if models.IsGTDSection(section.Name) {
				m.errorMsg = "cannot rename GTD sections"
				return m, nil
			}
			m.renameMode = true
			m.renameInput.SetValue(section.Name)
			m.renameInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case "d":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			section := m.sections[m.cursor]
			if models.IsGTDSection(section.Name) {
				m.errorMsg = "cannot remove GTD sections"
				return m, nil
			}
			return m, m.removeSection(section.Name)
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

// handleRenameMode handles keyboard input while in rename mode.
func (m *SectionManagerModel) handleRenameMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.cursor < len(m.sections) && strings.TrimSpace(m.renameInput.Value()) != m.sections[m.cursor].Name {
			m.confirmDiscard = true
			m.confirmAction = "discard-rename"
			return m, nil
		}
		m.renameMode = false
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		if len(m.sections) > 0 && m.cursor < len(m.sections) {
			oldName := m.sections[m.cursor].Name
			newName := strings.TrimSpace(m.renameInput.Value())
			if newName != "" && newName != oldName {
				return m, m.renameSection(oldName, newName)
			}
		}
		m.renameMode = false
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// handleAddMode handles keyboard input while in add mode.
func (m *SectionManagerModel) handleAddMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.addInput.Value()) != "" {
			m.confirmDiscard = true
			m.confirmAction = "discard-add"
			return m, nil
		}
		m.addMode = false
		m.addInput.SetValue("")
		return m, nil

	case "enter":
		name := strings.TrimSpace(m.addInput.Value())
		if name != "" {
			return m, m.addSection(name)
		}
		m.addMode = false
		m.addInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.addInput, cmd = m.addInput.Update(msg)
		return m, cmd
	}
}

// handleColorPickMode handles keyboard input while in color pick mode.
func (m *SectionManagerModel) handleColorPickMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	paletteLen := len(SectionColorPalette)

	switch msg.String() {
	case "esc", "q":
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

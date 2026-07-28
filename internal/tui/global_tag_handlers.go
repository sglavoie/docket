package tui

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// handleNormalMode handles keyboard input in normal mode.
func (m *GlobalTagManagerModel) handleNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "ctrl+t":
		return m, func() tea.Msg {
			return CloseGlobalTagManagerMsg{}
		}

	case "j", "down":
		if len(m.tags) > 0 {
			m.cursor++
			if m.cursor >= len(m.tags) {
				m.cursor = 0
			}
		}
		return m, nil

	case "k", "up":
		if len(m.tags) > 0 {
			m.cursor--
			if m.cursor < 0 {
				m.cursor = len(m.tags) - 1
			}
		}
		return m, nil

	case "enter":
		if len(m.tags) > 0 && m.cursor < len(m.tags) {
			tag := m.tags[m.cursor]
			if len(tag.Projects) == 1 {
				// Single project, navigate directly
				return m, func() tea.Msg {
					return GlobalTagNavigateMsg{
						Project: tag.Projects[0].Project,
						Tag:     tag.Name,
					}
				}
			}
			// Multiple projects, show picker
			m.showProjectPicker = true
			m.projectCursor = 0
			m.selectedTag = tag.Name
		}
		return m, nil

	case "e", "r":
		if len(m.tags) > 0 && m.cursor < len(m.tags) {
			m.renameMode = true
			m.renameTarget = m.tags[m.cursor].Name
			m.renameInput.SetValue(m.renameTarget)
			m.renameInput.Focus()
			return m, textinput.Blink
		}
		return m, nil

	case "space", "p":
		if len(m.tags) > 0 && m.cursor < len(m.tags) {
			return m, m.togglePinnedTag(m.tags[m.cursor].Name)
		}
		return m, nil
	}

	return m, nil
}

func (m *GlobalTagManagerModel) togglePinnedTag(tagName string) tea.Cmd {
	return toggleGlobalPinnedTagCmd(m.runtime, tagName)
}

// handleRenameMode handles keyboard input while in rename mode.
func (m *GlobalTagManagerModel) handleRenameMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if strings.TrimSpace(m.renameInput.Value()) != m.renameTarget {
			m.confirmDiscard = true
			return m, nil
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	case "enter":
		newName := strings.TrimSpace(m.renameInput.Value())
		if newName != "" && newName != m.renameTarget {
			return m, m.renameGlobalTag(m.renameTarget, newName)
		}
		m.renameMode = false
		m.renameTarget = ""
		m.renameInput.SetValue("")
		return m, nil

	default:
		var cmd tea.Cmd
		m.renameInput, cmd = m.renameInput.Update(msg)
		return m, cmd
	}
}

// handleProjectPickerMode handles keyboard input in project picker mode.
func (m *GlobalTagManagerModel) handleProjectPickerMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Find the current tag's projects
	var projects []TagProjectUsage
	for _, tag := range m.tags {
		if tag.Name == m.selectedTag {
			projects = tag.Projects
			break
		}
	}

	switch msg.String() {
	case "esc":
		m.showProjectPicker = false
		m.projectCursor = 0
		m.selectedTag = ""
		return m, nil

	case "j", "down":
		if len(projects) > 0 {
			m.projectCursor++
			if m.projectCursor >= len(projects) {
				m.projectCursor = 0
			}
		}
		return m, nil

	case "k", "up":
		if len(projects) > 0 {
			m.projectCursor--
			if m.projectCursor < 0 {
				m.projectCursor = len(projects) - 1
			}
		}
		return m, nil

	case "enter":
		if len(projects) > 0 && m.projectCursor < len(projects) {
			project := projects[m.projectCursor].Project
			tag := m.selectedTag
			return m, func() tea.Msg {
				return GlobalTagNavigateMsg{
					Project: project,
					Tag:     tag,
				}
			}
		}
		return m, nil
	}

	return m, nil
}

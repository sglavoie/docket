package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// HasTextInput returns true when search is active.
func (m *TutorModel) HasTextInput() bool {
	return m.searchMode
}

// IsFullHeight returns true because the tutor view renders its own footer.
func (m *TutorModel) IsFullHeight() bool {
	return true
}

// ShouldCaptureEsc returns true when the view needs to handle esc internally.
func (m *TutorModel) ShouldCaptureEsc() bool {
	return m.searchMode || m.searchQuery != ""
}

func (m *TutorModel) Init() tea.Cmd {
	return nil
}

func (m *TutorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		if m.searchMode {
			switch msg.String() {
			case "esc":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			case "enter":
				m.searchMode = false
				m.searchInput.Blur()
				return m, nil
			default:
				var cmd tea.Cmd
				m.searchInput, cmd = m.searchInput.Update(msg)
				m.searchQuery = m.searchInput.Value()
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
				return m, cmd
			}
		}

		switch msg.String() {
		case "esc":
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.searchInput.SetValue("")
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
				return m, nil
			}
			return m, func() tea.Msg { return BackMsg{} }
		case "/":
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink
		case "tab":
			m.activePane = 1 - m.activePane
			return m, nil
		case "h":
			if m.activePane == 0 {
				CycleTab(&m.activeTab, len(m.categories)+1, -1)
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "l":
			if m.activePane == 0 {
				CycleTab(&m.activeTab, len(m.categories)+1, 1)
				m.filterTopics()
				m.selectedIndex = 0
				m.listOffset = 0
				m.updateViewport()
			}
			return m, nil
		case "shift+left":
			CycleTab(&m.activeTab, len(m.categories)+1, -1)
			m.filterTopics()
			m.selectedIndex = 0
			m.listOffset = 0
			m.updateViewport()
			return m, nil
		case "shift+right":
			CycleTab(&m.activeTab, len(m.categories)+1, 1)
			m.filterTopics()
			m.selectedIndex = 0
			m.listOffset = 0
			m.updateViewport()
			return m, nil
		case "up", "k":
			if m.activePane == 0 {
				if m.selectedIndex > 0 {
					m.selectedIndex--
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollUp(1)
			}
			return m, nil
		case "down", "j":
			if m.activePane == 0 {
				if m.selectedIndex < len(m.filteredTopics)-1 {
					m.selectedIndex++
					m.updateViewport()
				}
			} else {
				m.viewport.ScrollDown(1)
			}
			return m, nil
		case "g":
			if m.activePane == 1 {
				m.viewport.GotoTop()
			}
			return m, nil
		case "G":
			if m.activePane == 1 {
				m.viewport.GotoBottom()
			}
			return m, nil
		case "pgup", "ctrl+u":
			m.viewport.HalfPageUp()
			return m, nil
		case "pgdown", "ctrl+d":
			m.viewport.HalfPageDown()
			return m, nil
		case "enter":
			if m.activePane == 0 {
				m.activePane = 1
				m.viewport.GotoTop()
			}
			return m, nil
		}
	}

	if m.activePane == 1 {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *TutorModel) HandledKeys() []string {
	return []string{
		"/", "tab", "shift+left", "shift+right",
		"up", "k", "down", "j",
		"g", "G",
		"ctrl+u", "ctrl+d",
		"enter",
	}
}

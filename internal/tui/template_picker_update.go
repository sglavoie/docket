package tui

import tea "charm.land/bubbletea/v2"

func (m *TemplatePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case tea.KeyPressMsg:
		// Navigation keys that work regardless of search mode
		switch msg.String() {
		case "up", "ctrl+k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
				m.previewScroll = 0
			}
			return m, nil
		case "down", "ctrl+j", "ctrl+n":
			if m.cursor < len(m.filteredTemplates) {
				m.cursor++
				m.previewScroll = 0
			}
			return m, nil
		case "enter":
			return m, m.selectTemplate()
		case "esc":
			return m, m.skipTemplate()
		case "tab":
			// Toggle preview in narrow mode
			if m.Width()-4 < minWidthForSplitView {
				m.showingPreview = !m.showingPreview
				return m, nil
			}
		case "ctrl+d", "ctrl+f":
			m.previewScroll += 5
			return m, nil
		case "ctrl+u", "ctrl+b":
			m.previewScroll = max(0, m.previewScroll-5)
			return m, nil
		}

		// j/k only navigate when NOT in search mode (allow typing j/k in search)
		if !m.searchActive {
			if m.SkipModifierInPrefix(msg) {
				return m, nil
			}
			if m.pendingPrefix == "g" {
				m.pendingPrefix = ""
				_, cmd := handleGotoTopPrefix(msg.String(), &m.cursor, &m.cursorFlash, func() { m.previewScroll = 0 })
				return m, cmd
			}

			switch msg.String() {
			case "j":
				if m.cursor < len(m.filteredTemplates) {
					m.cursor++
					m.previewScroll = 0
				}
				return m, nil
			case "k":
				if m.cursor > 0 {
					m.cursor--
					m.previewScroll = 0
				}
				return m, nil
			case "g":
				m.pendingPrefix = "g"
				return m, nil
			case "G":
				m.cursor = len(m.filteredTemplates)
				m.previewScroll = 0
				return m, nil
			case "/":
				return m, m.ActivateSearch()
			}
		}

		// Let search input handle remaining keys
		if m.searchActive {
			_, cmd, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterTemplates()
				m.previewScroll = 0 // Reset scroll when filter changes
			}
			return m, cmd
		}
	}

	return m, nil
}

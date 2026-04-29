package tui

import tea "charm.land/bubbletea/v2"

// Update handles messages.
func (m *NotesSearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Reload notes if they were mutated while this view was inactive.
	m.RefreshIfStale()

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil
	case tea.KeyPressMsg:
		switch msg.String() {
		case "esc":
			return m, func() tea.Msg { return CloseNotesSearchMsg{} }
		case "enter":
			if len(m.filtered) == 0 || m.cursor >= len(m.filtered) {
				return m, nil
			}

			note := m.filtered[m.cursor].note
			return m, func() tea.Msg {
				return NotesSearchSelectMsg{
					Note:    note,
					Project: m.project,
				}
			}
		case "up", "ctrl+k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "ctrl+j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "ctrl+u":
			jumpCursor(&m.cursor, len(m.filtered), -10)
			return m, nil
		case "ctrl+d":
			jumpCursor(&m.cursor, len(m.filtered), 10)
			return m, nil
		default:
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.filterNotes()
			return m, cmd
		}
	}

	return m, nil
}

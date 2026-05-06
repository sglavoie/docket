package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *CustomContextEditorModel) hasChanges() bool {
	if strings.TrimSpace(m.nameInput.Value()) != m.originalName {
		return true
	}
	if len(m.selectedIDs) != len(m.initialSelectedIDs) {
		return true
	}
	for k := range m.selectedIDs {
		if !m.initialSelectedIDs[k] {
			return true
		}
	}
	return false
}

func (m *CustomContextEditorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	var cmd tea.Cmd

	if m.confirmDiscard {
		if msg, ok := msg.(tea.KeyPressMsg); ok {
			switch msg.String() {
			case "y", "Y":
				m.confirmDiscard = false
				return m, func() tea.Msg { return BackMsg{} }
			case "n", "N", "esc":
				m.confirmDiscard = false
				return m, nil
			}
			return m, nil
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyPressMsg:
		// Handle search mode input first
		if m.IsSearchMode() {
			_, cmdHandleSearch, valueChanged := m.HandleSearchInput(msg)
			if valueChanged {
				m.filterProjects()
			}
			return m, cmdHandleSearch
		}

		switch msg.String() {
		case "esc":
			if m.projectListMode {
				// Exit project list mode, go back to name input
				m.projectListMode = false
				m.nameInput.Focus()
				return m, nil
			}
			if m.hasChanges() {
				m.confirmDiscard = true
				return m, nil
			}
			return m, func() tea.Msg {
				return BackMsg{}
			}

		case "tab":
			// Toggle between name input and project list
			if m.projectListMode {
				m.projectListMode = false
				m.nameInput.Focus()
			} else {
				m.projectListMode = true
				m.nameInput.Blur()
			}
			return m, nil

		case "/":
			if m.projectListMode {
				return m, m.ActivateSearch()
			}

		case "enter":
			if m.projectListMode {
				// Toggle selection of current project
				if m.cursor < len(m.filteredProjects) {
					id := m.filteredProjects[m.cursor].ID
					m.selectedIDs[id] = !m.selectedIDs[id]
					if !m.selectedIDs[id] {
						delete(m.selectedIDs, id)
					}
				}
				return m, nil
			}
			// From name input, move to project list
			m.projectListMode = true
			m.nameInput.Blur()
			return m, nil

		case "space":
			// Space toggles selection in project list mode
			if m.projectListMode && m.cursor < len(m.filteredProjects) {
				id := m.filteredProjects[m.cursor].ID
				m.selectedIDs[id] = !m.selectedIDs[id]
				if !m.selectedIDs[id] {
					delete(m.selectedIDs, id)
				}
				return m, nil
			}

		case "up", "k":
			if m.projectListMode {
				if m.cursor > 0 {
					m.cursor--
					// Adjust scroll offset
					if m.cursor < m.scrollOffset {
						m.scrollOffset = m.cursor
					}
				}
				return m, nil
			}

		case "down", "j":
			if m.projectListMode {
				if m.cursor < len(m.filteredProjects)-1 {
					m.cursor++
					// Adjust scroll offset
					maxVisible := m.maxVisibleProjects()
					if m.cursor >= m.scrollOffset+maxVisible {
						m.scrollOffset = m.cursor - maxVisible + 1
					}
				}
				return m, nil
			}

		case "ctrl+s":
			return m, m.save()

		case "a":
			// Select all visible (filtered) projects
			if m.projectListMode {
				for _, p := range m.filteredProjects {
					m.selectedIDs[p.ID] = true
				}
				return m, nil
			}

		case "A":
			// Deselect all visible (filtered) projects
			if m.projectListMode {
				for _, p := range m.filteredProjects {
					delete(m.selectedIDs, p.ID)
				}
				return m, nil
			}
		}
	}

	// Update name input if not in project list mode
	if !m.projectListMode {
		m.nameInput, cmd = m.nameInput.Update(msg)
	}

	return m, cmd
}

func (m *CustomContextEditorModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

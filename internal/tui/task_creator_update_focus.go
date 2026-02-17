package tui

import tea "github.com/charmbracelet/bubbletea"

func (m *TaskCreatorModel) blurCurrentField() {
	section, idx := m.getFocusSection()
	switch section {
	case "title":
		m.titleInput.Blur()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Blur()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Blur()
		}
	}
}

func (m *TaskCreatorModel) focusCurrentField() tea.Cmd {
	section, idx := m.getFocusSection()
	switch section {
	case "title":
		m.titleInput.Focus()
	case "checklist":
		if idx >= 0 && idx < len(m.checklist) {
			m.checklist[idx].Focus()
		}
	case "subtask":
		if idx >= 0 && idx < len(m.subtasks) {
			m.subtasks[idx].titleInput.Focus()
		}
	}
	return nil
}

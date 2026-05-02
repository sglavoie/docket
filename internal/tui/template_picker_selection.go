package tui

import tea "charm.land/bubbletea/v2"

func (m *TemplatePickerModel) selectTemplate() tea.Cmd {
	return func() tea.Msg {
		// cursor == len(filteredTemplates) means "Skip" is selected
		if m.cursor >= len(m.filteredTemplates) {
			return TemplatePickerDoneMsg{Canceled: false, Template: nil}
		}
		t := m.filteredTemplates[m.cursor]
		return TemplatePickerDoneMsg{Canceled: false, Template: &t}
	}
}

func (m *TemplatePickerModel) skipTemplate() tea.Cmd {
	return func() tea.Msg {
		return TemplatePickerDoneMsg{Canceled: true, Template: nil}
	}
}

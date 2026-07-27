package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
)

func (m *TaskTemplatesModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) { //nolint:gocyclo
	switch msg := msg.(type) {
	case CursorFlashClearMsg:
		m.cursorFlash = false
		return m, nil
	case TemplateDeletedMsg:
		if msg.Error != nil {
			return m, func() tea.Msg { return ErrorMsg{Err: msg.Error} }
		}

		defaultTemplates, customTemplates := m.runtime.Automation().GetTaskTemplatesSorted()
		m.allTemplates = append(defaultTemplates, customTemplates...)
		m.defaultCount = len(defaultTemplates)
		m.filterTemplates()
		m.cursor = ClampCursor(m.cursor, len(m.filteredTemplates))
		return m, func() tea.Msg { return TaskTemplatesUpdatedMsg{} }

	case tea.KeyPressMsg:
		if m.expandedView && !m.isWideScreen() {
			return m, m.handleExpandedViewKey(msg.String())
		}

		if confirmed, handled := m.HandleConfirmKey(msg.String()); handled {
			if confirmed {
				cmd := m.deleteCurrentTemplate()
				m.expandedView = false
				return m, cmd
			}
			return m, nil
		}

		if model, cmd, handled := m.HandleSearchMode(msg, m, m.filterTemplates); handled {
			return model, cmd
		}

		if m.isWideScreen() {
			switch msg.String() {
			case "ctrl+d":
				m.previewScroll += 5
				return m, nil
			case "ctrl+u":
				m.previewScroll = max(0, m.previewScroll-5)
				return m, nil
			}
		}

		return m.handleDefaultKey(msg.String())
	}

	return m, nil
}

func (m *TaskTemplatesModel) handleExpandedViewKey(key string) tea.Cmd {
	switch key {
	case "esc", "enter":
		m.expandedView = false
		m.previewScroll = 0
		return nil
	case "space":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			return m.toggleCurrentTemplate()
		}
		return nil
	case "e":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			m.expandedView = false
			m.previewScroll = 0
			t := m.filteredTemplates[m.cursor]
			return func() tea.Msg { return ShowTaskTemplateEditorMsg{Template: t} }
		}
		return nil
	case "d", "D":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'?", m.filteredTemplates[m.cursor].Name))
		}
		return nil
	case "ctrl+d":
		m.previewScroll += 5
		return nil
	case "ctrl+u":
		m.previewScroll = max(0, m.previewScroll-5)
		return nil
	case "j", "down":
		m.previewScroll++
		return nil
	case "k", "up":
		m.previewScroll = max(0, m.previewScroll-1)
		return nil
	default:
		return nil
	}
}

func (m *TaskTemplatesModel) handleDefaultKey(key string) (tea.Model, tea.Cmd) {
	if handled, model, cmd := m.handleNavigationKey(key); handled {
		return model, cmd
	}
	if handled, model, cmd := m.handleTemplateActionKey(key); handled {
		return model, cmd
	}
	return m, nil
}

func (m *TaskTemplatesModel) handleNavigationKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		if m.searchQuery != "" {
			m.ClearSearch()
			m.filterTemplates()
			return true, m, nil
		}
		return true, m, func() tea.Msg { return BackMsg{} }
	case "/":
		return true, m, m.ActivateSearch()
	case "j", "down":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates)-1 {
			m.cursor++
			m.previewScroll = 0
		}
		return true, m, nil
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
			m.previewScroll = 0
		}
		return true, m, nil
	case "home":
		m.cursor = 0
		m.previewScroll = 0
		return true, m, nil
	case "G":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			m.cursor = len(m.filteredTemplates) - 1
			m.previewScroll = 0
		}
		return true, m, nil
	default:
		return false, m, nil
	}
}

func (m *TaskTemplatesModel) handleTemplateActionKey(key string) (bool, tea.Model, tea.Cmd) {
	switch key {
	case "space":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			return true, m, m.toggleCurrentTemplate()
		}
		return true, m, nil
	case "enter":
		if !m.isWideScreen() && len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			m.expandedView = true
		}
		return true, m, nil
	case "d", "D":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'?", m.filteredTemplates[m.cursor].Name))
		}
		return true, m, nil
	case "n":
		return true, m, func() tea.Msg { return ShowTaskTemplateCreatorMsg{} }
	case "e":
		if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
			t := m.filteredTemplates[m.cursor]
			return true, m, func() tea.Msg { return ShowTaskTemplateEditorMsg{Template: t} }
		}
		return true, m, nil
	default:
		return false, m, nil
	}
}

func (m *TaskTemplatesModel) HandledKeys() []string {
	return m.buildKeyMap().HandledKeys()
}

// filterTemplates filters allTemplates based on the current search query.
func (m *TaskTemplatesModel) filterTemplates() {
	query := strings.ToLower(m.searchQuery)
	if query == "" {
		m.filteredTemplates = m.allTemplates
		m.cursor = ClampCursor(m.cursor, len(m.filteredTemplates))
		return
	}

	m.filteredTemplates = nil
	for _, t := range m.allTemplates {
		nameLower := strings.ToLower(t.Name)
		summaryLower := strings.ToLower(t.Summary())
		categoryLower := strings.ToLower(t.Category)
		if strings.Contains(nameLower, query) || strings.Contains(summaryLower, query) || strings.Contains(categoryLower, query) {
			m.filteredTemplates = append(m.filteredTemplates, t)
		}
	}

	m.cursor = ClampCursor(m.cursor, len(m.filteredTemplates))
}

func (m *TaskTemplatesModel) toggleCurrentTemplate() tea.Cmd {
	if m.cursor >= len(m.filteredTemplates) {
		return nil
	}

	t := &m.filteredTemplates[m.cursor]
	automation := cloneAutomationConfig(m.runtime.Automation())
	enabled, found := automation.ToggleTaskTemplate(t.ID)
	if !found {
		return func() tea.Msg { return ErrorMsg{Err: fmt.Errorf("template not found")} }
	}

	t.Enabled = enabled
	for i := range m.allTemplates {
		if m.allTemplates[i].ID == t.ID {
			m.allTemplates[i].Enabled = enabled
			break
		}
	}

	return m.coordinator.SaveThen(automation, func() tea.Msg { return TaskTemplatesUpdatedMsg{} })
}

func (m *TaskTemplatesModel) deleteCurrentTemplate() tea.Cmd {
	if m.cursor >= len(m.filteredTemplates) {
		return nil
	}

	templateID := m.filteredTemplates[m.cursor].ID
	automation := cloneAutomationConfig(m.runtime.Automation())
	if !automation.DeleteTaskTemplate(templateID) {
		return func() tea.Msg {
			return TemplateDeletedMsg{DeletedID: templateID, Error: fmt.Errorf("template not found")}
		}
	}

	return m.coordinator.SaveThen(automation, func() tea.Msg {
		return TemplateDeletedMsg{DeletedID: templateID}
	})
}

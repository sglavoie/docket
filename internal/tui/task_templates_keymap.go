package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for TaskTemplatesModel.
func (m *TaskTemplatesModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	// Navigation
	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates)-1 {
				m.cursor++
				m.previewScroll = 0
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates)-1 {
				m.cursor++
				m.previewScroll = 0
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.previewScroll = 0
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.previewScroll = 0
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 {
				m.cursor = len(m.filteredTemplates) - 1
				m.previewScroll = 0
			}
			return m, nil
		}),
		keymap.Bind("/", "Search templates", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
		keymap.Bind("ctrl+d", "Scroll preview down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.previewScroll += 5
			return m, nil
		}),
		keymap.Bind("ctrl+u", "Scroll preview up", "Navigation", func() (tea.Model, tea.Cmd) {
			m.previewScroll = max(0, m.previewScroll-5)
			return m, nil
		}),
	)

	entries = append(entries, keymap.Bind("home", "Jump to first", "Navigation", func() (tea.Model, tea.Cmd) {
		m.cursor = 0
		m.previewScroll = 0
		return m, nil
	}))

	// Actions
	entries = append(entries,
		keymap.Bind("space", "Toggle enabled/disabled", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				return m, m.toggleCurrentTemplate()
			}
			return m, nil
		}),
		keymap.Bind("enter", "Preview (narrow screen)", "Actions", func() (tea.Model, tea.Cmd) {
			if !m.isWideScreen() && len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				m.expandedView = true
			}
			return m, nil
		}),
		keymap.Bind("d", "Delete template", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'?", m.filteredTemplates[m.cursor].Name))
			}
			return m, nil
		}),
		keymap.Bind("D", "Delete template", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				m.confirm.Show("delete", fmt.Sprintf("Delete template '%s'?", m.filteredTemplates[m.cursor].Name))
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("n", "New template", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return ShowTaskTemplateCreatorMsg{} }
		}),
		keymap.Bind("e", "Edit template", "Actions", func() (tea.Model, tea.Cmd) {
			if len(m.filteredTemplates) > 0 && m.cursor < len(m.filteredTemplates) {
				t := m.filteredTemplates[m.cursor]
				return m, func() tea.Msg { return ShowTaskTemplateEditorMsg{Template: t} }
			}
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Task Templates",
		Entries:  entries,
	}
}

package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for WorkflowDryRunModel.
func (m *WorkflowDryRunModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("j", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move cursor down", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filtered) > 0 && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move cursor up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) {
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, nil
		}),
		keymap.Bind("/", "Search tasks", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
	)

	entries = append(entries, keymap.Bind("home", "Jump to first result", "Navigation", func() (tea.Model, tea.Cmd) {
		m.cursor = 0
		return m, nil
	}))

	return &keymap.KeyMap{
		ViewName: "Workflow Dry Run",
		Entries:  entries,
	}
}

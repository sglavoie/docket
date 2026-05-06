package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for PipelineHubModel.
func (m *PipelineHubModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("h", "Focus previous column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.activeCol = (m.activeCol - 1 + len(m.columns)) % len(m.columns)
			m.clampRow()
			return m, nil
		}),
		keymap.Bind("l", "Focus next column", "Navigation", func() (tea.Model, tea.Cmd) {
			m.activeCol = (m.activeCol + 1) % len(m.columns)
			m.clampRow()
			return m, nil
		}),
		keymap.Bind("j", "Move cursor down within column", "Navigation", func() (tea.Model, tea.Cmd) {
			col := m.columns[m.activeCol]
			if m.activeRow < len(col.entries)-1 {
				m.activeRow++
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move cursor up within column", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.activeRow > 0 {
				m.activeRow--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("enter", "Open selected view", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.openSelectedEntry()
		}),
	)

	return &keymap.KeyMap{
		ViewName: "PipelineHub",
		Entries:  entries,
	}
}

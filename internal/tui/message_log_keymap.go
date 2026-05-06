package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap constructs the declarative KeyMap for MessageLogViewModel.
func (m *MessageLogViewModel) buildKeyMap() *keymap.KeyMap {
	var entries []keymap.Entry

	entries = append(entries,
		keymap.Bind("j", "Scroll down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.scroll = min(m.scroll+1, max(0, len(m.entries)-1))
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("down", "Scroll down", "Navigation", func() (tea.Model, tea.Cmd) {
			m.scroll = min(m.scroll+1, max(0, len(m.entries)-1))
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("k", "Scroll up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.scroll > 0 {
				m.scroll--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("up", "Scroll up", "Navigation", func() (tea.Model, tea.Cmd) {
			if m.scroll > 0 {
				m.scroll--
			}
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("g", "Jump to top", "Navigation", func() (tea.Model, tea.Cmd) {
			m.scroll = 0
			return m, nil
		}),
		keymap.Bind("G", "Jump to bottom", "Navigation", func() (tea.Model, tea.Cmd) {
			m.scroll = max(0, len(m.entries)-1)
			return m, nil
		}),
		keymap.Bind("ctrl+d", "Page down", "Navigation", func() (tea.Model, tea.Cmd) {
			half := max(m.visibleLines()/2, 1)
			m.scroll = min(m.scroll+half, max(0, len(m.entries)-1))
			return m, nil
		}),
		keymap.Bind("ctrl+u", "Page up", "Navigation", func() (tea.Model, tea.Cmd) {
			half := max(m.visibleLines()/2, 1)
			m.scroll = max(0, m.scroll-half)
			return m, nil
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Message Log",
		Entries:  entries,
	}
}

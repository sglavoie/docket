package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildFocusBoardKeyMap constructs the declarative KeyMap for FocusBoardModel.
func buildFocusBoardKeyMap(m *FocusBoardModel) *keymap.KeyMap {
	var entries []keymap.Entry

	// --- List navigation: j, k, G, ctrl+u, ctrl+d ---
	entries = append(entries, keymap.ListNavigationGroup(keymap.ListNavigationDeps{
		CursorDown: func() (tea.Model, tea.Cmd) {
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				m.skipGroupHeaderForward()
			}
			return m, nil
		},
		CursorUp: func() (tea.Model, tea.Cmd) {
			if m.cursor > 0 {
				m.cursor--
				m.skipGroupHeaderBackward()
			}
			return m, nil
		},
		JumpToBottom: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			if len(m.filtered) > 0 {
				m.cursor = len(m.filtered) - 1
			}
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageUp: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), -HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
		PageDown: func() (tea.Model, tea.Cmd) {
			old := m.cursor
			jumpCursor(&m.cursor, len(m.filtered), HalfPage(m.Height()))
			return m, flashIfJumped(old, m.cursor, &m.cursorFlash)
		},
	})...)

	// --- View-specific direct bindings ---

	// Tab navigation
	entries = append(entries,
		keymap.Bind("tab", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}),
		keymap.Bind("shift+left", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+right", "Next tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
		keymap.Bind("shift+tab", "Previous tab", "Navigation", func() (tea.Model, tea.Cmd) {
			m.tabs.Cycle(-1)
			m.applyFilter()
			m.clampCursor()
			return m, nil
		}, keymap.AsHidden()),
	)

	// Search
	entries = append(entries,
		keymap.Bind("/", "Search", "Filtering", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("/"))
		}),
	)

	// Quick-add
	entries = append(entries,
		keymap.Bind("a", "Quick-add board item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("a"))
		}),
	)

	// Actions
	entries = append(entries,
		keymap.Bind("enter", "Navigate to task / open URL", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("enter"))
		}),
		keymap.Bind("x", "Complete or dismiss item", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("x"))
		}),
		keymap.Bind("p", "Plan or set priority", "Actions", func() (tea.Model, tea.Cmd) {
			return m.executeCockpitAction(m.resolveCockpitActionForKey("p"))
		}),
		keymap.Bind("?", "Toggle help", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.toggleHelpAction()
		}),
	)

	return &keymap.KeyMap{
		ViewName: "Today",
		Entries:  entries,
	}
}

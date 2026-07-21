package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/sglavoie/tt/internal/tui/keymap"
)

// buildKeyMap defines Home's complete local grammar. Dashboard layout controls,
// project-cycle shortcuts, hidden numeric aliases, and prefix grammars are not
// registered here, so Help and dispatch stay in lockstep.
func (m *DashboardModel) buildKeyMap() *keymap.KeyMap {
	entries := []keymap.Entry{
		keymap.Bind("j", "Move down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.moveHomeCursor("j")
		}, keymap.AsHidden()),
		keymap.Bind("k", "Move up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.moveHomeCursor("k")
		}, keymap.AsHidden()),
		keymap.Bind("down", "Move down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.moveHomeCursor("down")
		}, keymap.AsHidden()),
		keymap.Bind("up", "Move up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.moveHomeCursor("up")
		}, keymap.AsHidden()),
		keymap.Bind("g", "Go to first", "Navigation", func() (tea.Model, tea.Cmd) {
			cursor := m.activeHomeCursor()
			old := *cursor
			*cursor = 0
			return m, flashIfJumped(old, *cursor, &m.cursorFlash)
		}),
		keymap.Bind("G", "Go to last", "Navigation", func() (tea.Model, tea.Cmd) {
			cursor := m.activeHomeCursor()
			old := *cursor
			*cursor = max(m.activeHomeItemCount()-1, 0)
			return m, flashIfJumped(old, *cursor, &m.cursorFlash)
		}),
		keymap.Bind("ctrl+u", "Page up", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.pageHomeCursor(-HalfPage(m.homePageSize()))
		}),
		keymap.Bind("ctrl+d", "Page down", "Navigation", func() (tea.Model, tea.Cmd) {
			return m.pageHomeCursor(HalfPage(m.homePageSize()))
		}),
		keymap.Bind("h", "Focus Projects", "Navigation", func() (tea.Model, tea.Cmd) {
			m.homePane = homePaneProjects
			return m, nil
		}),
		keymap.Bind("l", "Focus Attention", "Navigation", func() (tea.Model, tea.Cmd) {
			m.homePane = homePaneAttention
			return m, nil
		}),
		keymap.Bind("/", "Search Home", "Navigation", func() (tea.Model, tea.Cmd) {
			return m, m.ActivateSearch()
		}),
		keymap.Bind("enter", "Open", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.activateHomeSelection()
		}),
		keymap.Bind("shift+enter", "Peek", "Actions", func() (tea.Model, tea.Cmd) {
			return m, m.peekHomeSelection()
		}),
		keymap.Bind("a", "Create project", "Actions", func() (tea.Model, tea.Cmd) {
			return m, func() tea.Msg { return CreateProjectMsg{} }
		}),
		keymap.Bind("e", "Edit project", "Actions", func() (tea.Model, tea.Cmd) {
			if m.homePane != homePaneProjects {
				return m, nil
			}
			project := m.SelectedProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg { return EditProjectMsg{Project: project} }
		}),
		keymap.Bind("v", "Show archived projects", "View", func() (tea.Model, tea.Cmd) {
			m.showArchived = !m.showArchived
			m.cursor = 0
			m.loadProjects()
			return m, nil
		}),
		keymap.Bind("u", "Unarchive project", "Actions", func() (tea.Model, tea.Cmd) {
			if m.homePane != homePaneProjects {
				return m, nil
			}
			project := m.SelectedProject()
			if project == nil {
				return m, nil
			}
			return m, func() tea.Msg {
				return ProjectArchiveRequestMsg{Project: project, Archive: false}
			}
		}, keymap.WithGate(func() bool { return m.showArchived })),
	}

	return &keymap.KeyMap{ViewName: "Home", Entries: entries}
}

func (m *DashboardModel) activeHomeCursor() *int {
	if m.homePane == homePaneAttention {
		return &m.attentionCursor
	}
	return &m.cursor
}

func (m *DashboardModel) activeHomeItemCount() int {
	if m.homePane == homePaneAttention {
		return len(m.visibleAttentionRows())
	}
	return len(m.visibleProjects())
}

func (m *DashboardModel) moveHomeCursor(key string) (tea.Model, tea.Cmd) {
	cursor := m.activeHomeCursor()
	_, cmd := handleListNavigation(key, cursor, m.activeHomeItemCount(), m.height, &m.cursorFlash)
	return m, cmd
}

func (m *DashboardModel) pageHomeCursor(delta int) (tea.Model, tea.Cmd) {
	cursor := m.activeHomeCursor()
	old := *cursor
	jumpCursor(cursor, m.activeHomeItemCount(), delta)
	return m, flashIfJumped(old, *cursor, &m.cursorFlash)
}

func (m *DashboardModel) activateHomeSelection() tea.Cmd {
	if m.homePane == homePaneAttention {
		return m.activateAttentionRow()
	}
	project := m.SelectedProject()
	if project == nil {
		return nil
	}
	return func() tea.Msg { return SelectProjectMsg{Project: project} }
}

func (m *DashboardModel) peekHomeSelection() tea.Cmd {
	if m.homePane == homePaneAttention {
		// Task-backed Attention rows are intercepted by the app-level Peek
		// grammar. Action-only rows use Shift+Enter as an alternate activation.
		return m.activateAttentionRow()
	}
	project := m.SelectedProject()
	if project == nil {
		return nil
	}
	return func() tea.Msg { return ShowProjectPreviewMsg{Project: project} }
}

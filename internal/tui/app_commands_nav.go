package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) navCommands() []Command {
	return []Command{
		{
			Name:        "dashboard",
			Description: "Go to Home",
			Shortcut:    "ctrl+esc",
			Category:    CategoryNavigation,
			Aliases:     []string{"home", "projects", "attention"},
			Handler: func() tea.Msg {
				return JumpDashboardMsg{}
			},
		},
		{
			Name:        "recent tasks",
			Description: "Quick-switch between recently visited tasks",
			Shortcut:    "|",
			Category:    CategoryNavigation,
			Aliases:     []string{"quick switch", "task switcher", "recent visits"},
			Handler: func() tea.Msg {
				if cmd := m.toggleQuickSwitch(); cmd != nil {
					return PaletteCommandResultMsg{Cmd: cmd}
				}
				return nil
			},
		},
		{
			Name:        "peek task detail",
			Description: "Peek at the selected task's details",
			Shortcut:    "shift+enter",
			Category:    CategoryNavigation,
			Views:       taskPeekViews(),
			Aliases:     []string{"peek task", "task peek", "preview task"},
			Handler: func() tea.Msg {
				task, project := m.getCurrentCursorTaskAndProject()
				if task == nil {
					return nil
				}
				m.activatePeek(task, project)
				return nil
			},
		},
	}
}

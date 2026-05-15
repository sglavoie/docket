package tui

import tea "charm.land/bubbletea/v2"

func (m *AppModel) navCommands() []Command {
	return []Command{
		{
			Name:        "dashboard",
			Description: "Go to dashboard",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewProjectTree, ViewProjectList, ViewOutliner},
			Handler: func() tea.Msg {
				return BackMsg{}
			},
		},
		{
			Name:        "project cycle",
			Description: "Configure project cycle, jump to project",
			Shortcut:    "@",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Handler: func() tea.Msg {
				return ShowProjectCycleManagerMsg{}
			},
		},
		{
			Name:        "project picker",
			Description: "Open project cycle picker",
			Shortcut:    "`",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Handler: func() tea.Msg {
				return ShowProjectCyclePickerMsg{}
			},
		},
		{
			Name:        "previous project",
			Description: "Switch to previous project in cycle",
			Shortcut:    "~",
			Category:    CategoryNavigation,
			Views:       []ViewType{ViewDashboard, ViewProjectTree, ViewOutliner},
			Handler: func() tea.Msg {
				return CycleProjectMsg{Forward: false}
			},
		},
		{
			Name:        "history back",
			Description: "Navigate back through view history",
			Shortcut:    "alt+O",
			Category:    CategoryNavigation,
			Aliases:     []string{"go back history", "previous view", "back history"},
			Handler: func() tea.Msg {
				return NavigateHistoryBackMsg{}
			},
		},
		{
			Name:        "history forward",
			Description: "Navigate forward through view history",
			Shortcut:    "alt+I",
			Category:    CategoryNavigation,
			Aliases:     []string{"go forward history", "next view", "forward history"},
			Handler: func() tea.Msg {
				return NavigateHistoryForwardMsg{}
			},
		},
		{
			Name:        "recent tasks",
			Description: "Quick-switch between recently visited tasks",
			Shortcut:    "|",
			Category:    CategoryNavigation,
			Aliases:     []string{"quick switch", "task switcher", "recent visits"},
			Handler: func() tea.Msg {
				m.openQuickSwitch()
				return nil
			},
		},
		{
			Name:        "peek task detail",
			Description: "Hold space to peek at the selected task's details",
			Shortcut:    "space",
			Category:    CategoryNavigation,
			Aliases:     []string{"peek task", "task peek", "preview task"},
			Handler: func() tea.Msg {
				task, project := m.getCurrentCursorTaskAndProject()
				if task == nil {
					return nil
				}
				m.peekTask = task
				m.peekProject = project
				m.peekActive = true
				return nil
			},
		},
	}
}
